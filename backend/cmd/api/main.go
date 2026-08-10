/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Main entry point for the HTTP API server process.
 */

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/async"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/cache"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/dbguard"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/eid"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/observability"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Response deadline. /auth/eid/poll blocks for eid.PollWindow while the relying
// party waits for the citizen to approve, so this has to outlast it with room
// for the round trip. At 15s it did not: Go passed the write deadline while the
// handler was still waiting, closed the connection without a response, and
// nginx turned that into a 502 for every check the citizen did not answer
// within 15 seconds — which read as a slow, flaky sign-in.
//
// It is not only sign-in any more: /documents/{id}/sign/eid/poll waits on the same
// relying-party call for the same window, so a signature ceremony would have been
// cut off in the same way and lost the approval a citizen had just given.
var writeTimeout = eid.PollWindow + 15*time.Second

// Slowloris is held off by the header deadline rather than by writeTimeout, so
// a long poll does not have to buy a client the right to dribble out a request.
const (
	readTimeout       = 15 * time.Second
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	if err := config.ValidateProduction(); err != nil {
		slog.Error("invalid production configuration", "error", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable"
	}

	catalogPath := resolveCatalogPath(os.Getenv("APP_CATALOG_PATH"))

	ctx := context.Background()

	// Initialize OpenTelemetry distributed tracing
	shutdownTracing, err := observability.SetupTracing(ctx, "gerege-sso", os.Getenv("ENVIRONMENT"))
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
	} else {
		defer func() {
			_ = shutdownTracing(ctx)
		}()
	}

	// Connect to shared-schema PostgreSQL database pool
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		slog.Error("failed to parse db url", "error", err)
		os.Exit(1)
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnIdleTime = 15 * time.Minute

	// Bind every pooled connection to the tenant its caller is acting for, so
	// the row-level policies from migration 00029 apply. Installed before the
	// pool exists because it is a property of how connections are handed out;
	// it stays dormant until Probe confirms the database can enforce it.
	guard := &dbguard.Guard{}
	guard.Install(poolConfig)

	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		slog.Error("failed to connect to postgres pool", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	databaseReachable := db.Ping(ctx) == nil
	if !databaseReachable {
		slog.Warn("database unreachable on startup; continuing, /ready reports it")
	} else {
		// Before anything else touches the pool: a probe that ran alongside
		// live queries could leave a connection carrying a role nothing had
		// asked for.
		probeCtx, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
		err := guard.Probe(probeCtx, db)
		cancelProbe()
		if err != nil {
			// The guard could not be switched on safely. That is a deployment
			// fault — the migrations are present but the login role cannot
			// reach the app role — and running on one layer while the schema
			// says there are two is the state worth refusing to start in.
			slog.Error("failed to enable row-level tenant isolation", "error", err)
			os.Exit(1)
		}
	}

	// Redis, when configured, is what makes a permission revoked on one replica
	// stop being honoured by the others, and what makes a rate limit a budget
	// for the deployment rather than one per process. Absent, both fall back to
	// what this platform has always done.
	redisClient := cache.Dial(os.Getenv("REDIS_URL"))
	if redisClient != nil {
		defer func() { _ = redisClient.Close() }()
	}
	busCtx, stopBus := context.WithCancel(context.Background())
	defer stopBus()
	bus := cache.NewBus(busCtx, redisClient)
	slog.Info("cache invalidation configured", "shared", bus.Redis())

	// Initialize Platform Server
	srv, err := platform.NewServer(db, catalogPath, bus)
	if err != nil {
		slog.Error("failed to initialize platform server", "error", err)
		os.Exit(1)
	}

	// Restores the documented demo login (admin@example.com) and the two
	// organisations it belongs to. Skipped in production unless SEED_DEMO_DATA
	// is set explicitly.
	//
	// After the server rather than before it: seeding now installs apps for
	// those tenants, and an installation row references the apps table, which
	// NewServer is what fills from the catalogue file.
	if databaseReachable {
		seedInitialData(ctx, db, srv)
	}

	// Background jobs run until this context is cancelled during shutdown, so
	// a sweep in flight is not left holding a database connection.
	jobsCtx, stopJobs := context.WithCancel(context.Background())
	defer stopJobs()
	if !databaseReachable {
		// The probe above had nothing to talk to. Without this the process
		// would serve every request for the rest of its life with tenant
		// isolation switched off, because the one thing that switches it on
		// already ran and failed.
		async.Go("dbguard-probe-retry", func() {
			guard.ProbeUntilEnabled(jobsCtx, db, 15*time.Second)
		})
	}
	srv.StartBackgroundJobs(jobsCtx)

	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Router(),
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	async.Go("http-server", func() {
		slog.Info("starting Gerege SSO API server", "port", port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server listener error", "error", err)
			os.Exit(1)
		}
	})

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down HTTP API server gracefully...")
	stopJobs()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced server shutdown", "error", err)
	} else {
		slog.Info("server shutdown complete cleanly")
	}
	fmt.Println("Goodbye!")
}
