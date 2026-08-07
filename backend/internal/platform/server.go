/*
 * Gerege Template Platform
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package platform provides the core HTTP Server orchestrator, routing table,
 * authentication middleware, and app installer wiring.
 */

package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/billing"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/contacts"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/developer_portal"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/documents"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/esign"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/gov_services"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/inventory"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/apps/products"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/ai"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/appinstaller"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/audit"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/config"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/dan"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/eid"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/gerege"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/integration"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/mailer"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/menu"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/observability"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/rbac"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/resilience"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/security"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/ssoprovider"
	"github.com/gerege-systems/open-gerege-mn-erp/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

// PlatformVersion is the semver the app-store manifests are validated against.
const PlatformVersion = "1.0.0"

type Server struct {
	db             *pgxpool.Pool
	installer      *appinstaller.AppInstaller
	router         *chi.Mux
	sessions       *auth.SessionStore
	loginLimiter   *security.IPRateLimiter
	pollLimiter    *security.IPRateLimiter
	aiLimiter      *security.IPRateLimiter
	asyncMailer    *mailer.AsyncOTPMailer
	copilotSvc     *ai.CopilotService
	forecaster     *ai.Forecaster
	eidSvc         *eid.EIDService
	danSvc         *dan.DANService
	ssoProvider    *ssoprovider.SSOProvider
	geregeSvc      *gerege.GeregeService
	integrationMgr *integration.Manager
	permissions    *rbac.SQLPermissionStore
	billingMod     *billing.BillingModule
	documentsMod   *documents.DocumentsModule
	govMod         *gov_services.Module
	devPortalMod   *developer_portal.DeveloperPortalModule
	contactsMod    *contacts.Module
	productsMod    *products.Module
	inventoryMod   *inventory.Module
	esignMod       *esign.Module
}

func NewServer(db *pgxpool.Pool, catalogPath string) (*Server, error) {
	catalogData, err := os.ReadFile(catalogPath)
	if err != nil {
		return nil, err
	}

	var rawCatalog []appcatalog.CatalogApp
	if err := json.Unmarshal(catalogData, &rawCatalog); err != nil {
		return nil, err
	}

	// Populate full manifests.
	//
	// A manifest that failed to load used to be replaced by a silent stub with
	// no dependencies, permissions or menus. Three shipped manifests were in
	// fact malformed (object instead of array for "dependencies", plain
	// strings instead of objects for "permissions") and nobody noticed: the
	// apps installed with an empty dependency graph and never contributed a
	// menu entry. Catalog integrity is now a startup error.
	catalogDir := filepath.Dir(catalogPath)
	catalog := make([]appcatalog.CatalogApp, 0, len(rawCatalog))
	for _, app := range rawCatalog {
		if !security.IsValidSlug(app.Slug) {
			return nil, fmt.Errorf("catalog app %q has an invalid slug %q", app.ID, app.Slug)
		}
		manifestPath := filepath.Join(catalogDir, "manifests", app.Slug+".json")
		manifest, err := appcatalog.LoadManifestFile(manifestPath, PlatformVersion)
		if err != nil {
			return nil, fmt.Errorf("load manifest for %s: %w", app.ID, err)
		}
		if manifest.ID != app.ID {
			return nil, fmt.Errorf("manifest %s declares id %q but the catalog entry is %q",
				manifestPath, manifest.ID, app.ID)
		}
		app.Manifest = manifest
		catalog = append(catalog, app)
	}

	installer := appinstaller.NewAppInstaller(db, catalog, PlatformVersion)

	// Keep the apps table in step with the catalog file. A missing row makes
	// installation fail on the app_installations foreign key, so this is a
	// startup concern, not a seeding concern. A cold database must not stop the
	// process from booting — /ready reports that separately.
	syncCtx, cancelSync := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSync()
	if err := installer.SyncCatalog(syncCtx); err != nil {
		slog.Error("failed to sync app catalog into database", "error", err)
	}

	// Instantiate compile-time Go modules once. Each constructor registers the
	// module in the global app registry; calling them twice (here and again in
	// registerAppModuleRoutes) built two instances per app.
	contactsMod := contacts.New(db)
	productsMod := products.New(db)
	inventoryMod := inventory.New(db, false) // false = prevent negative stock
	billingMod := billing.New(db)
	documentsMod := documents.New(db)
	govMod := gov_services.New(db)
	esignMod := esign.New(db, gerege.NewEsignService())

	// Instantiate Async Mailer Queue
	syncMailer := mailer.NewSyncOTPMailer(os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_FROM"), os.Getenv("SMTP_PASSWORD"))
	asyncMailer := mailer.NewAsyncOTPMailer(syncMailer, 2, 64, 3)

	ssoProvider := ssoprovider.NewSSOProvider(db)
	devPortalMod := developer_portal.NewDeveloperPortalModule(ssoProvider)

	s := &Server{
		db:             db,
		installer:      installer,
		router:         chi.NewRouter(),
		sessions:       auth.NewSessionStore(db, auth.DefaultSessionTTL),
		loginLimiter:   newLoginLimiter(),
		pollLimiter:    newPollLimiter(),
		aiLimiter:      security.NewIPRateLimiter(rate.Limit(20.0/60.0), 10),
		asyncMailer:    asyncMailer,
		copilotSvc:     ai.NewCopilotService(db),
		forecaster:     ai.NewForecaster(db),
		eidSvc:         eid.NewEIDService(),
		danSvc:         dan.NewDANService(),
		ssoProvider:    ssoProvider,
		geregeSvc:      gerege.NewGeregeService(),
		integrationMgr: integration.NewManager(),
		permissions:    rbac.NewSQLPermissionStore(db),
		billingMod:     billingMod,
		documentsMod:   documentsMod,
		govMod:         govMod,
		devPortalMod:   devPortalMod,
		contactsMod:    contactsMod,
		productsMod:    productsMod,
		inventoryMod:   inventoryMod,
		esignMod:       esignMod,
	}

	// The authorization endpoint has to know who is signing in, which is the
	// platform session rather than anything OAuth owns.
	ssoProvider.AttachSessions(s.sessions)

	// Clients live in Postgres now, so the built-in one is registered once
	// rather than rebuilt into a map on every boot. A cold database must not
	// stop the process from starting: /ready reports that separately.
	ssoCtx, cancelSSO := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSSO()
	ssoProvider.EnsureDefaultClient(ssoCtx)

	// Spent codes and dead tokens are reclaimed on a timer. The in-memory
	// token map this replaced had no eviction at all — it only ever grew.
	ssoProvider.StartJanitor(context.Background(), 15*time.Minute)

	s.setupRoutes()
	return s, nil
}

func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) setupRoutes() {
	r := s.router

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(resilience.NewLoadShedder(1000).Middleware)
	r.Use(observability.MetricsMiddleware)
	r.Use(security.HeadersMiddleware)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   security.SafeCORSOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-Tenant-ID"},
		AllowCredentials: true,
	}))

	// Infrastructure
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := s.db.Ping(r.Context()); err != nil {
			http.Error(w, `{"status":"error","message":"database unreachable"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	// Prometheus Metrics Endpoint
	r.Handle("/metrics", observability.MetricsHandler())

	// OpenID Connect Provider & OAuth2 Authorization Server.
	//
	// These sit at the root rather than under /api/, which is where the
	// specification puts them and what SSO_ISSUER advertises — the reverse
	// proxy has to route them to this service explicitly.
	r.Get("/.well-known/openid-configuration", s.ssoProvider.HandleOIDCDiscovery)
	r.Get("/.well-known/jwks.json", s.ssoProvider.HandleJWKS)
	// The authorization endpoint is a browser destination: it reads the
	// session cookie itself and answers with redirects, so it must not sit
	// behind the API's bearer-token middleware.
	r.Get("/oauth2/auth", s.ssoProvider.HandleAuthorize)
	r.Post("/oauth2/token", s.ssoProvider.HandleTokenEndpoint)
	r.Post("/oauth2/introspect", s.ssoProvider.HandleIntrospect)
	r.Post("/oauth2/revoke", s.ssoProvider.HandleRevoke)
	r.Get("/oauth2/userinfo", s.ssoProvider.HandleUserInfo)
	r.Post("/oauth2/userinfo", s.ssoProvider.HandleUserInfo)

	// Platform API
	r.Route("/api/v1", func(api chi.Router) {
		// Auth with rate limiting
		api.With(security.RateLimitMiddleware(s.loginLimiter)).Post("/auth/login", s.handleLogin)
		api.With(security.RateLimitMiddleware(s.loginLimiter)).Post("/auth/eid/login", s.handleEIDLogin)
		api.With(security.RateLimitMiddleware(s.loginLimiter)).Post("/auth/eid/start", s.handleEIDStart)
		api.With(security.RateLimitMiddleware(s.loginLimiter)).Post("/auth/eid/start-id", s.handleEIDStartByNationalID)
		// Not the login limiter: a citizen polls for as long as it takes them to
		// reach their phone, and sharing that budget with sign-in attempts made
		// a busy office throttle itself out of signing in at all.
		api.With(security.RateLimitMiddleware(s.pollLimiter)).Post("/auth/eid/poll", s.handleEIDPoll)
		api.With(security.RateLimitMiddleware(s.loginLimiter)).Post("/auth/dan/login", s.handleDANLogin)
		api.Post("/auth/logout", s.handleLogout)

		// Protected endpoints
		api.Group(func(pr chi.Router) {
			pr.Use(s.authMiddleware)

			pr.Get("/auth/me", s.handleMe)
			pr.Get("/menus", s.handleMenus)

			// Consent screen. The browser endpoint at /oauth2/auth redirects
			// here; these two describe the pending grant and record the
			// answer. Both re-validate the request against the database, so
			// the frontend is a renderer rather than a source of truth.
			pr.Get("/oauth2/consent", s.ssoProvider.HandleConsentPrompt)
			pr.Post("/oauth2/consent", s.ssoProvider.HandleConsentDecision)

			// Tenant access control. Mutations are deliberately admin-only;
			// authorization configuration can otherwise be used to self-elevate.
			pr.Route("/admin/access", func(ac chi.Router) {
				ac.Use(s.requireAdmin)
				ac.Get("/overview", s.handleAccessOverview)
				ac.Post("/roles", s.handleCreateRole)
				ac.Put("/roles/{id}", s.handleUpdateRole)
				ac.Delete("/roles/{id}", s.handleDeleteRole)
				ac.Put("/roles/{id}/permissions", s.handleSetRolePermissions)
				ac.Put("/memberships/{id}/roles", s.handleSetMembershipRoles)
			})

			// AI Copilot & Forecasting
			pr.With(security.RateLimitMiddleware(s.aiLimiter)).Post("/ai/copilot", s.handleAICopilot)
			pr.With(security.RateLimitMiddleware(s.aiLimiter)).Post("/ai/chat", s.handleAIChat)
			pr.With(security.RateLimitMiddleware(s.aiLimiter)).Post("/ai/stt", s.handleAISTT)
			pr.With(security.RateLimitMiddleware(s.aiLimiter)).Post("/ai/tts", s.handleAITTS)
			pr.With(security.RateLimitMiddleware(s.aiLimiter)).Post("/ai/translate", s.handleAITranslate)
			pr.Get("/ai/stock-forecast", s.handleAIForecast)
			pr.With(s.requireAdmin).Get("/admin/ai/prompts", s.handleAIListPrompts)
			pr.With(s.requireAdmin).Put("/admin/ai/prompts/{key}", s.handleAIUpdatePrompt)
			pr.With(s.requireAdmin).Get("/admin/ai/knowledge", s.handleAIListKnowledge)
			pr.With(s.requireAdmin).Post("/admin/ai/knowledge", s.handleAICreateKnowledge)

			// XYP State Information Exchange System (xyp.gerege.mn)
			pr.Post("/xyp/citizen", s.handleXYPCitizenQuery)
			pr.Post("/xyp/company", s.handleXYPCompanyQuery)

			// External Integrations Manager (admin-only: a connector target URL
			// makes the server issue arbitrary outbound requests)
			pr.With(s.requireAdmin).Get("/integrations", s.handleListIntegrations)
			pr.With(s.requireAdmin).Post("/integrations", s.handleRegisterIntegration)

			// Store — reads are open to any tenant member, mutations are
			// tenant-administrator only. Previously every authenticated user
			// could install, enable or disable apps for the whole tenant.
			pr.Get("/store/apps", s.handleListStoreApps)
			pr.Get("/store/apps/{slug}", s.handleGetStoreApp)
			pr.Get("/installed-apps", s.handleListInstalledApps)

			pr.Group(func(ar chi.Router) {
				ar.Use(s.requireAdmin)
				ar.Post("/store/apps/{slug}/install", s.handleInstallApp)
				ar.Post("/store/apps/{slug}/enable", s.handleEnableApp)
				ar.Post("/store/apps/{slug}/disable", s.handleDisableApp)
			})
		})
	})

	// Register compile-time Business App Routes with Tenant & App Gate protection
	s.registerAppModuleRoutes()
}

// registerAppModuleRoutes mounts every compile-time business module behind the
// tenant app gate. Billing, Documents and the Developer Portal used to be wired
// straight into the protected group, so their endpoints stayed reachable for
// tenants that had never installed the app.
func (s *Server) registerAppModuleRoutes() {
	s.contactsMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.contacts"))
	s.productsMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.products"))
	s.inventoryMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.inventory"))
	s.billingMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.billing"))
	s.documentsMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.documents"))
	s.govMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.gov_services"))
	s.devPortalMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.developer_portal"))
	s.esignMod.RegisterRoutes(s.router, s.appGateMiddleware("io.example.esign"))
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.TokenFromRequest(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized: missing session token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := s.sessions.Resolve(r.Context(), token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized: invalid or expired session"}`, http.StatusUnauthorized)
			return
		}

		ctx := auth.WithUserContext(r.Context(), claims)
		ctx = tenant.WithTenantID(ctx, claims.TenantID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin gates tenant-administrative endpoints. It must be layered after
// authMiddleware.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := auth.UserFromContext(r.Context())
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if !claims.IsAdmin {
			http.Error(w, `{"error":"forbidden: tenant administrator role required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) appGateMiddleware(appID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, err := tenant.FromContext(r.Context())
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Check if app is installed and enabled for this tenant
			var enabled bool
			err = s.db.QueryRow(r.Context(),
				`SELECT enabled FROM app_installations WHERE tenant_id = $1 AND app_id = $2`,
				tenantID, appID).Scan(&enabled)

			if err != nil || !enabled {
				http.Error(w, `{"error":"forbidden: app module `+appID+` is not installed or enabled for this tenant"}`, http.StatusForbidden)
				return
			}

			// Model-level access rights are additive across all assigned roles,
			// matching Odoo's ir.model.access behaviour. Government workflow has
			// its own action- and unit-aware permission checks.
			if permission := appRequestPermission(appID, r.Method); permission != "" {
				rbac.RequirePermission(s.permissions, permission)(next).ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func appRequestPermission(appID, method string) string {
	prefixes := map[string]string{
		"io.example.contacts": "contacts", "io.example.products": "products",
		"io.example.inventory": "inventory", "io.example.billing": "billing",
		"io.example.documents": "documents", "io.example.developer_portal": "developer",
	}
	prefix := prefixes[appID]
	if prefix == "" {
		return ""
	}
	if method == http.MethodGet || method == http.MethodHead {
		return prefix + ".read"
	}
	return prefix + ".manage"
}

// Handlers
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"invalid login credentials"}`, http.StatusBadRequest)
		return
	}

	var userID, passwordHash, tenantID, name string
	var isAdmin bool
	err := s.db.QueryRow(r.Context(),
		`SELECT u.id, u.password_hash, u.name,
		        (u.is_admin OR EXISTS (
		            SELECT 1 FROM membership_roles mr
		            JOIN roles r ON r.id=mr.role_id
		            WHERE mr.membership_id=m.id AND r.tenant_id=m.tenant_id
		              AND r.code='admin' AND r.active
		        )) AS is_admin,
		        m.tenant_id
		 FROM users u
		 JOIN memberships m ON m.user_id = u.id
		 WHERE u.email = $1 LIMIT 1`, req.Email).Scan(&userID, &passwordHash, &name, &isAdmin, &tenantID)

	if err != nil || !auth.CheckPasswordHash(req.Password, passwordHash) {
		audit.Record(r.Context(), "unknown", "anonymous", "auth.login_failed", "user", map[string]any{"email": req.Email})
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "password")
	if err != nil {
		http.Error(w, `{"error":"failed to establish session"}`, http.StatusInternalServerError)
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.login_success", "user", map[string]any{"email": req.Email})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_at": expiresAt,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      name,
			"email":     req.Email,
			"is_admin":  isAdmin,
		},
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Logout previously only cleared the cookie; the token stayed valid
	// forever for anyone who had captured it.
	if token := auth.TokenFromRequest(r); token != "" {
		if err := s.sessions.Revoke(r.Context(), token); err != nil {
			slog.Error("failed to revoke session", "error", err)
		}
	}

	auth.ClearSessionCookie(w)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "logged_out"})
}

// Sign-in and eID polling are budgeted separately, because they are different
// things happening at different rates.
//
// Signing in is the endpoint worth guessing against, and starting an eID
// session pushes a notification to a real person's phone, so it stays tight.
//
// Polling is neither: it needs a session ID the relying party only ever handed
// to whoever started that session, and it cannot be turned into an attempt at a
// second one. What it does do is repeat — once every eid.PollWindow for as long
// as a citizen takes to reach their phone. Sharing the sign-in budget meant a
// few citizens waiting behind one office or NAT address spent it between them,
// and the next person there could not sign in at all. So this is budgeted by
// how many of them may plausibly be waiting behind a single address at once.
const (
	loginRatePerMinute = 5
	loginBurst         = 5
	pollRatePerMinute  = 60
	pollBurst          = 15
)

func newLoginLimiter() *security.IPRateLimiter {
	return security.NewIPRateLimiter(rate.Limit(float64(loginRatePerMinute)/60.0), loginBurst)
}

func newPollLimiter() *security.IPRateLimiter {
	return security.NewIPRateLimiter(rate.Limit(float64(pollRatePerMinute)/60.0), pollBurst)
}

// issueSession creates a persisted session bound to the caller's IP and agent.
func (s *Server) issueSession(r *http.Request, userID, tenantID, method string) (string, time.Time, error) {
	return s.sessions.Create(r.Context(), userID, tenantID, method,
		r.UserAgent(), security.ClientIP(r))
}

// signInError carries a reason that is meant for the person signing in. Account
// linking also fails for reasons that are ours alone — a missing key, a broken
// query, a rejected hash — and those are logged, never rendered: the citizen
// once saw "bcrypt: password length exceeds 72 bytes" in the eID card.
type signInError struct{ msg string }

func (e signInError) Error() string { return e.msg }

// reportSignInFailure answers with the reason when it is the caller's to act
// on, and with a stable message otherwise.
func reportSignInFailure(w http.ResponseWriter, err error) {
	var visible signInError
	if errors.As(err, &visible) {
		writeJSONError(w, http.StatusForbidden, visible.Error())
		return
	}
	slog.Error("failed to link verified national identity", "error", err)
	writeJSONError(w, http.StatusInternalServerError, "Баталгаажсан eID хэрэглэгчийг ERP бүртгэлтэй холбож чадсангүй")
}

// resolveNationalIdentityUser maps a verified national identity (E-ID / DAN)
// onto a local ERP user.
//
// The previous implementation ran `SELECT id FROM users LIMIT 1` and granted
// is_admin unconditionally, i.e. any successful gateway response logged the
// caller in as an arbitrary — in practice the seeded admin — account.
func (s *Server) resolveNationalIdentityUser(ctx context.Context, email, regNumber string) (userID, tenantID string, err error) {
	if email != "" {
		err = s.db.QueryRow(ctx,
			`SELECT u.id::text, m.tenant_id::text
			   FROM users u
			   JOIN memberships m ON m.user_id = u.id
			  WHERE lower(u.email) = lower($1)
			  LIMIT 1`, email).Scan(&userID, &tenantID)
		if err == nil {
			return userID, tenantID, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", err
		}
	}

	if config.IsProduction() {
		return "", "", signInError{fmt.Sprintf("no ERP user is linked to national identity %s", regNumber)}
	}

	// Development convenience only: fall back to the seeded demo account so
	// the documented mock login flow keeps working locally.
	err = s.db.QueryRow(ctx,
		`SELECT u.id::text, m.tenant_id::text
		   FROM users u
		   JOIN memberships m ON m.user_id = u.id
		  ORDER BY u.created_at
		  LIMIT 1`).Scan(&userID, &tenantID)
	if err != nil {
		return "", "", fmt.Errorf("no ERP user available for national identity login: %w", err)
	}
	slog.Warn("national identity login fell back to the demo account",
		"reg_number", regNumber, "email", email)
	return userID, tenantID, nil
}

// eidLinkingDigest derives the stable, non-PII handle for an eID subject. It
// doubles as the synthetic account's password preimage, so its length is not
// cosmetic: bcrypt rejects anything over 72 bytes outright, and a suffix that
// pushed it to 73 once failed every first-time eID sign-in.
func eidLinkingDigest(linkingKey, subject string) string {
	mac := hmac.New(sha256.New, []byte(linkingKey))
	_, _ = mac.Write([]byte("eid-mn:" + subject))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

// resolveOrProvisionEIDUser links an eID subject to a stable, non-PII local
// identifier. JIT provisioning is opt-in per tenant and always receives the
// standard user role through the membership_default_role database trigger.
func (s *Server) resolveOrProvisionEIDUser(ctx context.Context, identity *eid.EIDIdentity) (userID, tenantID string, err error) {
	if identity.Email != "" {
		if userID, tenantID, err = s.resolveNationalIdentityUser(ctx, identity.Email, identity.RegNumber); err == nil {
			return userID, tenantID, nil
		}
	}
	subject := strings.TrimSpace(identity.CivilID)
	if subject == "" {
		subject = strings.TrimSpace(identity.RegNumber)
	}
	if subject == "" {
		return "", "", errors.New("eID identity carries neither a civil ID nor a registration number")
	}
	linkingKey := os.Getenv("EID_RP_SECRET")
	if linkingKey == "" {
		return "", "", errors.New("EID_RP_SECRET is unset, so no account-linking key is available")
	}
	digest := eidLinkingDigest(linkingKey, subject)
	syntheticEmail := "eid+" + digest[:32] + "@identity.invalid"
	if err = s.db.QueryRow(ctx,
		`SELECT u.id::text, m.tenant_id::text FROM users u JOIN memberships m ON m.user_id=u.id WHERE u.email=$1 LIMIT 1`,
		syntheticEmail).Scan(&userID, &tenantID); err == nil {
		return userID, tenantID, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", err
	}

	tenantSlug := strings.TrimSpace(os.Getenv("EID_JIT_TENANT_SLUG"))
	if tenantSlug == "" {
		return "", "", signInError{"eID identity is verified but account provisioning is disabled"}
	}
	if err = s.db.QueryRow(ctx, `SELECT id::text FROM tenants WHERE slug=$1`, tenantSlug).Scan(&tenantID); err != nil {
		return "", "", fmt.Errorf("eID provisioning tenant %q is unavailable: %w", tenantSlug, err)
	}
	name := strings.TrimSpace(identity.LastName + " " + identity.FirstName)
	if name == "" {
		name = "eID Mongolia хэрэглэгч"
	}
	// The synthetic account has no password login path. Keep the random-looking
	// preimage within bcrypt's strict 72-byte input limit.
	passwordHash, err := auth.HashPassword(digest)
	if err != nil {
		return "", "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx,
		`INSERT INTO users(email,password_hash,name,is_admin) VALUES($1,$2,$3,FALSE)
		 ON CONFLICT(email) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`,
		syntheticEmail, passwordHash, name).Scan(&userID); err != nil {
		return "", "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO memberships(tenant_id,user_id) VALUES($1,$2) ON CONFLICT(tenant_id,user_id) DO NOTHING`, tenantID, userID); err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return userID, tenantID, nil
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var name, email string
	var tenantName string
	_ = s.db.QueryRow(r.Context(), `SELECT name, email FROM users WHERE id = $1`, claims.UserID).Scan(&name, &email)
	_ = s.db.QueryRow(r.Context(), `SELECT name FROM tenants WHERE id = $1`, claims.TenantID).Scan(&tenantName)

	// The effective grant of every role the member holds, so a screen can hide
	// what the caller may not do. Administrators bypass the check, so their
	// list stays empty rather than enumerating the whole catalog.
	granted := make([]string, 0)
	if !claims.IsAdmin {
		if permissions, permErr := s.permissions.GetUserPermissions(r.Context(), claims.TenantID, claims.UserID); permErr == nil {
			for code := range permissions {
				granted = append(granted, code)
			}
			sort.Strings(granted)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          claims.UserID,
		"tenant_id":   claims.TenantID,
		"tenant_name": tenantName,
		"name":        name,
		"email":       email,
		"is_admin":    claims.IsAdmin,
		"permissions": granted,
	})
}

func (s *Server) handleMenus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	menus, err := menu.GetTenantMenus(r.Context(), s.installer, tenantID, config.LocaleFromRequest(r))
	if err != nil {
		http.Error(w, `{"error":"failed to fetch menus"}`, http.StatusInternalServerError)
		return
	}
	claims, _ := auth.UserFromContext(r.Context())
	if !claims.IsAdmin {
		permissions, permissionErr := s.permissions.GetUserPermissions(r.Context(), tenantID, claims.UserID)
		if permissionErr != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to resolve menu access")
			return
		}
		visible := menus[:0]
		for _, item := range menus {
			permission := appReadPermission(item.AppID)
			if permission == "" || permissions[permission] {
				visible = append(visible, item)
			}
		}
		menus = visible
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(menus)
}

func appReadPermission(appID string) string {
	switch appID {
	case "io.example.contacts":
		return "contacts.read"
	case "io.example.products":
		return "products.read"
	case "io.example.inventory":
		return "inventory.read"
	case "io.example.billing":
		return "billing.read"
	case "io.example.documents":
		return "documents.read"
	case "io.example.developer_portal":
		return "developer.read"
	case "io.example.gov_services":
		return "gov.read"
	default:
		return ""
	}
}

func (s *Server) handleListStoreApps(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	catalog := s.installer.GetCatalog()

	// "installed" and "enabled" are distinct states: an app can be installed
	// and then disabled. Deriving both from the enabled-only query reported
	// disabled apps as never installed, so the UI offered "Install" again.
	installedStates, err := s.installer.GetInstallationStatesForTenant(r.Context(), tenantID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load installed apps")
		return
	}

	type StoreAppResponse struct {
		appcatalog.CatalogApp
		Installed bool `json:"installed"`
		Enabled   bool `json:"enabled"`
	}

	locale := config.LocaleFromRequest(r)
	res := make([]StoreAppResponse, 0, len(catalog))
	for _, app := range catalog {
		state, installed := installedStates[app.ID]
		res = append(res, StoreAppResponse{
			CatalogApp: app.Localized(locale),
			Installed:  installed,
			Enabled:    state,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleGetStoreApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		http.Error(w, `{"error":"invalid app slug format"}`, http.StatusBadRequest)
		return
	}

	app, ok := s.installer.GetAppBySlug(slug)
	if !ok {
		http.Error(w, `{"error":"app not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(app.Localized(config.LocaleFromRequest(r)))
}

func (s *Server) handleListInstalledApps(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	rows, err := s.db.Query(r.Context(),
		`SELECT ai.id, ai.app_id, a.slug, a.name, ai.installed_version, ai.status, ai.enabled, ai.installed_at
		 FROM app_installations ai
		 JOIN apps a ON a.id = ai.app_id
		 WHERE ai.tenant_id = $1`, tenantID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type InstalledApp struct {
		ID               string    `json:"id"`
		AppID            string    `json:"app_id"`
		Slug             string    `json:"slug"`
		Name             string    `json:"name"`
		InstalledVersion string    `json:"installed_version"`
		Status           string    `json:"status"`
		Enabled          bool      `json:"enabled"`
		InstalledAt      time.Time `json:"installed_at"`
	}

	list := make([]InstalledApp, 0)
	for rows.Next() {
		var item InstalledApp
		if err := rows.Scan(&item.ID, &item.AppID, &item.Slug, &item.Name, &item.InstalledVersion, &item.Status, &item.Enabled, &item.InstalledAt); err == nil {
			list = append(list, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleInstallApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		http.Error(w, `{"error":"invalid app slug format"}`, http.StatusBadRequest)
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := s.installer.InstallApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "installed", "app": slug})
}

func (s *Server) handleDisableApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		http.Error(w, `{"error":"invalid app slug format"}`, http.StatusBadRequest)
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := s.installer.DisableApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "disabled", "app": slug})
}

func (s *Server) handleEnableApp(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !security.IsValidSlug(slug) {
		http.Error(w, `{"error":"invalid app slug format"}`, http.StatusBadRequest)
		return
	}

	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := s.installer.EnableApp(r.Context(), claims.TenantID, slug, claims.UserID); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "enabled", "app": slug})
}

func (s *Server) handleAICopilot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prompt == "" {
		http.Error(w, `{"error":"invalid prompt"}`, http.StatusBadRequest)
		return
	}

	res, err := s.copilotSvc.Query(r.Context(), ai.CopilotRequest{
		Prompt:   req.Prompt,
		TenantID: tenantID,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req ai.CopilotRequest
	if err := decodeLimitedJSON(r, &req, 1<<20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid AI request")
		return
	}
	req.TenantID = tenantID
	res, err := s.copilotSvc.Query(r.Context(), req)
	if err != nil {
		writeJSONError(w, aiStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAISTT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Audio ai.Audio `json:"audio"`
	}
	if err := decodeLimitedJSON(r, &req, 1<<20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid audio request")
		return
	}
	text, err := s.copilotSvc.Transcribe(r.Context(), req.Audio)
	if err != nil {
		writeJSONError(w, aiStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"text": text})
}

func (s *Server) handleAITTS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := decodeLimitedJSON(r, &req, 16<<10); err != nil || req.Text == "" {
		writeJSONError(w, http.StatusBadRequest, "text is required")
		return
	}
	audio, err := s.copilotSvc.Speak(r.Context(), req.Text)
	if err != nil {
		writeJSONError(w, aiStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, audio)
}

func (s *Server) handleAITranslate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text   string    `json:"text"`
		Audio  *ai.Audio `json:"audio"`
		Target string    `json:"target_lang"`
		Speak  bool      `json:"speak"`
	}
	if err := decodeLimitedJSON(r, &req, 1<<20); err != nil || req.Target == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid translation request")
		return
	}
	if req.Text == "" && req.Audio != nil {
		var err error
		req.Text, err = s.copilotSvc.Transcribe(r.Context(), *req.Audio)
		if err != nil {
			writeJSONError(w, aiStatus(err), err.Error())
			return
		}
	}
	translated, err := s.copilotSvc.Translate(r.Context(), req.Text, req.Target)
	if err != nil {
		writeJSONError(w, aiStatus(err), err.Error())
		return
	}
	result := map[string]any{"source_text": req.Text, "translated": translated}
	if req.Speak {
		if sound, e := s.copilotSvc.Speak(r.Context(), translated); e == nil {
			result["audio"] = sound
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAIListPrompts(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeJSONError(w, 401, "unauthorized")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT prompt_key,content,active,tenant_id IS NULL FROM ai_prompts WHERE tenant_id IS NULL OR tenant_id=$1 ORDER BY prompt_key,tenant_id NULLS FIRST`, tenantID)
	if err != nil {
		writeJSONError(w, 500, "failed to load AI prompts")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var key, content string
		var active, global bool
		if rows.Scan(&key, &content, &active, &global) == nil {
			items = append(items, map[string]any{"key": key, "content": content, "active": active, "global": global})
		}
	}
	writeJSON(w, 200, items)
}
func (s *Server) handleAIUpdatePrompt(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeJSONError(w, 401, "unauthorized")
		return
	}
	key := chi.URLParam(r, "key")
	if key != "scope" && key != "instructions" {
		writeJSONError(w, 400, "invalid prompt key")
		return
	}
	var req struct {
		Content string `json:"content"`
		Active  bool   `json:"active"`
	}
	if decodeLimitedJSON(r, &req, 32<<10) != nil || strings.TrimSpace(req.Content) == "" {
		writeJSONError(w, 400, "content is required")
		return
	}
	_, err = s.db.Exec(r.Context(), `INSERT INTO ai_prompts(tenant_id,prompt_key,content,active) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,prompt_key) DO UPDATE SET content=EXCLUDED.content,active=EXCLUDED.active,updated_at=NOW()`, tenantID, key, req.Content, req.Active)
	if err != nil {
		writeJSONError(w, 500, "failed to save AI prompt")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "saved"})
}
func (s *Server) handleAIListKnowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeJSONError(w, 401, "unauthorized")
		return
	}
	rows, err := s.db.Query(r.Context(), `SELECT id,title,content,source_url,updated_at FROM ai_knowledge WHERE tenant_id=$1 ORDER BY updated_at DESC LIMIT 100`, tenantID)
	if err != nil {
		writeJSONError(w, 500, "failed to load knowledge")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, title, content, url string
		var updated time.Time
		if rows.Scan(&id, &title, &content, &url, &updated) == nil {
			items = append(items, map[string]any{"id": id, "title": title, "content": content, "source_url": url, "updated_at": updated})
		}
	}
	writeJSON(w, 200, items)
}
func (s *Server) handleAICreateKnowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeJSONError(w, 401, "unauthorized")
		return
	}
	var req struct {
		Title     string `json:"title"`
		Content   string `json:"content"`
		SourceURL string `json:"source_url"`
	}
	if decodeLimitedJSON(r, &req, 256<<10) != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		writeJSONError(w, 400, "title and content are required")
		return
	}
	var id string
	err = s.db.QueryRow(r.Context(), `INSERT INTO ai_knowledge(tenant_id,title,content,source_url) VALUES($1,$2,$3,$4) RETURNING id`, tenantID, req.Title, req.Content, req.SourceURL).Scan(&id)
	if err != nil {
		writeJSONError(w, 500, "failed to save knowledge")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}

func decodeLimitedJSON(r *http.Request, dst any, max int64) error {
	return json.NewDecoder(io.LimitReader(r.Body, max)).Decode(dst)
}
func aiStatus(error) int { return http.StatusBadGateway }

func (s *Server) handleAIForecast(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	forecast, err := s.forecaster.AnalyzeTenantStock(r.Context(), tenantID)
	if err != nil {
		http.Error(w, `{"error":"failed to generate forecast"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(forecast)
}

func (s *Server) handleEIDStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CallbackURL string `json:"callbackUrl"`
	}
	if r.Body != nil {
		_ = decodeLimitedJSON(r, &req, 8<<10)
	}
	callback, err := validEIDCallback(req.CallbackURL)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid eID callback URL")
		return
	}
	started, err := s.eidSvc.StartDeviceLink(r.Context(), callback)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "eID Mongolia session could not be started")
		return
	}
	writeJSON(w, http.StatusOK, started)
}

func (s *Server) handleEIDStartByNationalID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NationalID  string `json:"national_id"`
		CallbackURL string `json:"callbackUrl"`
	}
	if decodeLimitedJSON(r, &req, 8<<10) != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	callback, err := validEIDCallback(req.CallbackURL)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid eID callback URL")
		return
	}
	started, err := s.eidSvc.StartByNationalID(r.Context(), req.NationalID, callback)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Регистрийн дугаар олдсонгүй эсвэл eID апп-д бүртгэлгүй байна")
		return
	}
	writeJSON(w, http.StatusOK, started)
}

func validEIDCallback(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	callback, err := url.Parse(raw)
	if err != nil || callback.User != nil || (callback.Scheme != "https" && (config.IsProduction() || callback.Scheme != "http")) {
		return "", errors.New("invalid callback")
	}
	publicOrigin := strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN"))
	if publicOrigin == "" {
		publicOrigin = "http://localhost:3000"
	}
	origin, err := url.Parse(publicOrigin)
	if err != nil || !strings.EqualFold(callback.Host, origin.Host) || callback.Path != "/auth/eid/callback" {
		return "", errors.New("callback not allowed")
	}
	return callback.String(), nil
}

func (s *Server) handleEIDPoll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if decodeLimitedJSON(r, &req, 8<<10) != nil || strings.TrimSpace(req.SessionID) == "" {
		writeJSONError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	result, err := s.eidSvc.Poll(r.Context(), req.SessionID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "eID Mongolia session check failed")
		return
	}
	if result.State != "COMPLETE" {
		writeJSON(w, http.StatusOK, result)
		return
	}
	if result.Identity == nil || !result.Identity.VerifiedStatus {
		writeJSONError(w, http.StatusUnauthorized, "eID identity verification failed")
		return
	}
	userID, tenantID, err := s.resolveOrProvisionEIDUser(r.Context(), result.Identity)
	if err != nil {
		reportSignInFailure(w, err)
		return
	}
	token, expiresAt, err := s.issueSession(r, userID, tenantID, "eid-app")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to establish session")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)
	audit.Record(r.Context(), tenantID, userID, "auth.eid_app_login_success", "eid", map[string]any{"verified": true, "method": "eid-app"})
	writeJSON(w, http.StatusOK, map[string]any{"state": result.State, "token": token, "expires_at": expiresAt, "identity": result.Identity})
}

func (s *Server) handleEIDLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string         `json:"code"`
		RedirectURI string         `json:"redirect_uri"`
		RegNumber   string         `json:"reg_number"`
		OTPCode     string         `json:"otp_code"`
		AuthMethod  eid.AuthMethod `json:"auth_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	var identity *eid.EIDIdentity
	var err error
	if req.Code != "" {
		identity, err = s.eidSvc.ExchangeCode(r.Context(), req.Code, req.RedirectURI)
	} else if req.RegNumber != "" {
		identity, err = s.eidSvc.AuthenticateWithMethod(r.Context(), req.RegNumber, req.OTPCode, req.AuthMethod)
	} else {
		http.Error(w, `{"error":"missing authorization code or registration number"}`, http.StatusBadRequest)
		return
	}

	// err may be nil while identity is nil — calling err.Error() unguarded
	// panicked the request goroutine.
	if err != nil || identity == nil {
		msg := "E-ID verification failed"
		if err != nil {
			msg = "E-ID verification failed: " + err.Error()
		}
		writeJSONError(w, http.StatusUnauthorized, msg)
		return
	}

	userID, tenantID, err := s.resolveNationalIdentityUser(r.Context(), identity.Email, identity.RegNumber)
	if err != nil {
		reportSignInFailure(w, err)
		return
	}

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "eid")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to establish session")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.eid_login_success", "eid", map[string]any{
		"reg_number": identity.RegNumber,
		"civil_id":   identity.CivilID,
	})

	claims, _ := s.sessions.Resolve(r.Context(), token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"expires_at": expiresAt,
		"identity":   identity,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      identity.FirstName + " " + identity.LastName,
			"email":     claims.Email,
			"is_admin":  claims.IsAdmin,
		},
	})
}

func (s *Server) handleDANLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DANToken  string `json:"dan_token"`
		RegNumber string `json:"reg_number"`
		OTPCode   string `json:"otp_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	var profile *dan.DANProfile
	var err error
	if req.DANToken != "" {
		profile, err = s.danSvc.VerifyDANToken(r.Context(), req.DANToken)
	} else if req.RegNumber != "" {
		profile, err = s.danSvc.AuthenticateDANCitizen(r.Context(), req.RegNumber, req.OTPCode)
	} else {
		http.Error(w, `{"error":"missing dan_token or registration number"}`, http.StatusBadRequest)
		return
	}

	if err != nil || profile == nil {
		msg := "dan.gerege.mn verification failed"
		if err != nil {
			msg = "dan.gerege.mn verification failed: " + err.Error()
		}
		writeJSONError(w, http.StatusUnauthorized, msg)
		return
	}

	userID, tenantID, err := s.resolveNationalIdentityUser(r.Context(), profile.Email, profile.RegNumber)
	if err != nil {
		reportSignInFailure(w, err)
		return
	}

	token, expiresAt, err := s.issueSession(r, userID, tenantID, "dan")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to establish session")
		return
	}
	auth.SetSessionCookie(w, token, expiresAt)

	audit.Record(r.Context(), tenantID, userID, "auth.dan_gerege_login_success", "dan", map[string]any{
		"reg_number":  profile.RegNumber,
		"dan_session": profile.DANSessionID,
	})

	claims, _ := s.sessions.Resolve(r.Context(), token)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":       token,
		"expires_at":  expiresAt,
		"dan_profile": profile,
		"user": map[string]any{
			"id":        userID,
			"tenant_id": tenantID,
			"name":      profile.FirstName + " " + profile.LastName,
			"email":     claims.Email,
			"is_admin":  claims.IsAdmin,
		},
	})
}

func (s *Server) handleXYPCitizenQuery(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		RegNumber string `json:"reg_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RegNumber == "" {
		http.Error(w, `{"error":"invalid registration number"}`, http.StatusBadRequest)
		return
	}

	info, err := s.geregeSvc.GetCitizenInfo(r.Context(), req.RegNumber)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "XYP citizen query failed: "+err.Error())
		return
	}

	audit.Record(r.Context(), tenantID, "system", "xyp.citizen_queried", "xyp", map[string]any{"reg_number": req.RegNumber})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleXYPCompanyQuery(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		CompanyReg string `json:"company_reg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CompanyReg == "" {
		http.Error(w, `{"error":"invalid company registration number"}`, http.StatusBadRequest)
		return
	}

	info, err := s.geregeSvc.GetCompanyInfo(r.Context(), req.CompanyReg)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "XYP company query failed: "+err.Error())
		return
	}

	audit.Record(r.Context(), tenantID, "system", "xyp.company_queried", "xyp", map[string]any{"company_reg": req.CompanyReg})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	list := s.integrationMgr.List()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleRegisterIntegration(w http.ResponseWriter, r *http.Request) {
	var cfg integration.IntegrationConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil || cfg.Name == "" {
		http.Error(w, `{"error":"invalid integration configuration"}`, http.StatusBadRequest)
		return
	}

	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("int_%d", time.Now().UnixNano())
	}

	s.integrationMgr.Register(&cfg)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "registered", "integration": cfg})
}

// writeJSONError emits a JSON error body. Interpolating err.Error() straight
// into a hand-written JSON string, as the handlers used to, produced invalid
// JSON as soon as the message contained a quote or newline.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
