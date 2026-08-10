// Package dbguard binds a pooled database connection to the tenant the caller
// is acting for, so PostgreSQL row-level security can enforce the isolation the
// application layer already intends.
//
// Every query in this codebase carries `WHERE tenant_id = $1`, and that stays
// the primary control. This is the layer underneath it: a handler that forgets
// the clause — in a module written next year, by somebody who has not read
// every other module — returns nothing rather than another tenant's rows.
//
// How it works: pgxpool hands out a connection per query, and BeforeAcquire
// runs with the acquiring request's context. That is the one place that knows
// both which physical connection is about to be used and whose request is about
// to use it, so it is where the binding is made.
//
//	tenant in context     → SET ROLE gerege_nexus_app, app.current_tenant = <id>
//	no tenant in context  → SET ROLE NONE (the login role, not subject to the policies)
//
// The second case is not a gap, it is the platform path, and it is the reason
// this design needs no change to a single existing query. Signing in has no
// tenant yet; resolving a session is what discovers the tenant; the OAuth
// callback and the e-mail verification landing are reached by people who have
// not signed in; the housekeeping sweeps deliberately cross every tenant. All
// of those run as the login role and see everything, exactly as they do today.
package dbguard

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AppRole is the unprivileged role the tenant-scoped policies are written for.
// It is created by migration 00029 and owns nothing.
const AppRole = "gerege_nexus_app"

// bindStatement sets both variables in one round trip.
//
// `role` is an ordinary GUC, so set_config assigns it exactly as SET ROLE would
// and the whole binding costs one message rather than two. Both values are
// parameters: the tenant id arrives from a session row, and building this
// string by concatenation would put a SET ROLE one quote away from user data.
const bindStatement = `SELECT set_config('role', $1, false), set_config('app.current_tenant', $2, false)`

// Guard installs the binding and reports whether it is live.
//
// It starts disabled. Enable is called after Probe has confirmed the database
// is actually carrying the policies, because a pool that tried to SET ROLE to a
// role that does not exist would fail every acquisition — an outage caused by
// the security control, on a deployment whose migrations had not caught up.
type Guard struct {
	enabled atomic.Bool
}

// Install attaches the binding to a pool configuration. It must be called
// before the pool is created, and takes effect only once Probe succeeds.
//
// PrepareConn rather than the older BeforeAcquire: the two cannot coexist —
// pgxpool ignores BeforeAcquire entirely when PrepareConn is set — so a library
// or a later change that reaches for the current hook would silently switch
// this one off, and the isolation would be gone with nothing to show for it.
func (g *Guard) Install(cfg *pgxpool.Config) {
	cfg.PrepareConn = func(ctx context.Context, conn *pgx.Conn) (bool, error) {
		if !g.enabled.Load() {
			return true, nil
		}
		role, tenantID := "none", ""
		if id, err := tenant.FromContext(ctx); err == nil && id != "" {
			role, tenantID = AppRole, id
		}
		if _, err := conn.Exec(ctx, bindStatement, role, tenantID); err != nil {
			// False destroys the connection rather than handing over one whose
			// tenant binding is whatever the previous request left behind — the
			// failure this package exists to prevent. The error travels with it
			// so the query fails saying why, instead of pgxpool working through
			// the pool and reporting that it ran out of attempts.
			slog.Error("dbguard: could not bind the connection to a tenant",
				"role", role, "error", err)
			return false, fmt.Errorf("dbguard: could not bind the connection to tenant %q: %w",
				tenantID, err)
		}
		return true, nil
	}
}

// Enabled reports whether the binding is being applied.
func (g *Guard) Enabled() bool { return g.enabled.Load() }

// Probe checks that the database can enforce what the guard assumes, and turns
// the guard on if it can.
//
// A deployment whose migrations have not reached 00029 is a normal state during
// a rollout, and it is left running with the application-level filtering it has
// always had rather than being taken down. It is reported at WARN because a
// deployment that stays in that state is running with one layer, not two.
func (g *Guard) Probe(ctx context.Context, pool *pgxpool.Pool) error {
	var roleExists, policiesExist bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1),
		       EXISTS (SELECT 1 FROM pg_policies
		                WHERE schemaname = 'public' AND policyname = 'tenant_isolation')`,
		AppRole).Scan(&roleExists, &policiesExist)
	if err != nil {
		return fmt.Errorf("dbguard: could not inspect the database: %w", err)
	}
	if !roleExists || !policiesExist {
		slog.Warn("dbguard: row-level tenant isolation is not installed; "+
			"the application filter is the only layer",
			"role_present", roleExists, "policies_present", policiesExist,
			"remedy", "run the database migrations up to 00029_tenant_rls")
		return nil
	}

	// The role has to be reachable from whoever we connect as. If it is not,
	// enabling the guard would break every tenant-scoped query, so this is
	// checked before anything is switched on rather than discovered by traffic.
	//
	// One connection is held for both halves of the check. Two Exec calls would
	// be free to land on two different pooled connections, and the first of them
	// would go back to the pool still wearing the role — which nothing would
	// correct until the guard is on and starts rebinding on every acquisition.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("dbguard: could not take a connection to verify the role: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, bindStatement, AppRole, ""); err != nil {
		return fmt.Errorf("dbguard: cannot assume %s (grant it to the login role in DATABASE_URL): %w",
			AppRole, err)
	}
	if _, err := conn.Exec(ctx, bindStatement, "none", ""); err != nil {
		return fmt.Errorf("dbguard: cannot return to the login role: %w", err)
	}

	g.enabled.Store(true)
	slog.Info("dbguard: row-level tenant isolation is active", "role", AppRole)
	return nil
}

// ProbeUntilEnabled keeps probing until the guard is on or ctx is cancelled.
//
// It exists for the startup this process explicitly tolerates: the database
// being unreachable when the API comes up. /ready reports that, the pool
// reconnects on its own, and traffic starts working the moment the database
// does — but the guard is switched on by a probe, and a probe that ran once
// against nothing would leave the process serving that traffic for the rest of
// its life with the isolation switched off and nothing saying so.
//
// Connections acquired between the database returning and the probe succeeding
// are not bound. That window is bounded by interval and only exists on a
// degraded start; every acquisition after it is bound.
func (g *Guard) ProbeUntilEnabled(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if g.Enabled() {
			return
		}
		probeCtx, cancel := context.WithTimeout(ctx, interval)
		err := g.Probe(probeCtx, pool)
		cancel()
		if err != nil {
			slog.Warn("dbguard: still cannot enable tenant isolation", "error", err)
			continue
		}
		if g.Enabled() {
			return
		}
	}
}
