package appinstaller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appcatalog"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AppInstaller struct {
	db              *pgxpool.Pool
	catalog         []appcatalog.CatalogApp
	platformVersion string
}

func NewAppInstaller(db *pgxpool.Pool, catalog []appcatalog.CatalogApp, platformVersion string) *AppInstaller {
	return &AppInstaller{
		db:              db,
		catalog:         catalog,
		platformVersion: platformVersion,
	}
}

func (ai *AppInstaller) GetCatalog() []appcatalog.CatalogApp {
	return ai.catalog
}

func (ai *AppInstaller) GetAppBySlug(slug string) (appcatalog.CatalogApp, bool) {
	for _, app := range ai.catalog {
		if app.Slug == slug {
			return app, true
		}
	}
	return appcatalog.CatalogApp{}, false
}

func (ai *AppInstaller) GetAppByID(id string) (appcatalog.CatalogApp, bool) {
	for _, app := range ai.catalog {
		if app.ID == id {
			return app, true
		}
	}
	return appcatalog.CatalogApp{}, false
}

// InstallApp handles recursive dependency resolution and tenant installation.
func (ai *AppInstaller) InstallApp(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("app with slug %q not found in store catalog", appSlug)
	}

	// Build dependency graph from catalog
	manifests := make([]appcatalog.Manifest, 0, len(ai.catalog))
	for _, app := range ai.catalog {
		manifests = append(manifests, app.Manifest)
	}
	graph := NewDependencyGraph(manifests)

	installOrderIDs, err := graph.ResolveInstallOrder(targetApp.ID)
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Verify all modules in install order are compiled into binary
	for _, appID := range installOrderIDs {
		if err := appregistry.VerifyModuleExists(appID); err != nil {
			return fmt.Errorf("compile-time module missing for %s: %w", appID, err)
		}
	}

	tx, err := ai.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Install apps in topological order (dependencies first)
	for _, appID := range installOrderIDs {
		app, _ := ai.GetAppByID(appID)

		// Check if already installed
		var existingID string
		err := tx.QueryRow(ctx,
			`SELECT id FROM app_installations WHERE tenant_id = $1 AND app_id = $2`,
			tenantID, app.ID).Scan(&existingID)

		now := time.Now()
		var installID string

		if err != nil {
			// Not installed yet — insert installation
			installID = uuid.New().String()
			_, err = tx.Exec(ctx,
				`INSERT INTO app_installations (id, tenant_id, app_id, installed_version, status, enabled, installed_at, updated_at)
				 VALUES ($1, $2, $3, $4, 'installed', TRUE, $5, $6)`,
				installID, tenantID, app.ID, app.Version, now, now)
			if err != nil {
				return fmt.Errorf("insert app installation for %s: %w", app.ID, err)
			}
		} else {
			installID = existingID
			// Enable and update status
			_, err = tx.Exec(ctx,
				`UPDATE app_installations SET status = 'installed', enabled = TRUE, updated_at = $1
				 WHERE id = $2`, now, installID)
			if err != nil {
				return fmt.Errorf("update app installation for %s: %w", app.ID, err)
			}
		}

		// Register app permissions for tenant. Get returning !ok used to fall
		// through to a nil-interface method call and panic the request.
		mod, ok := appregistry.Get(app.ID)
		if !ok {
			return fmt.Errorf("compile-time module missing for %s", appID)
		}
		for _, perm := range mod.Permissions() {
			permID := uuid.New().String()
			_, _ = tx.Exec(ctx,
				`INSERT INTO permissions (id, code, name, description)
				 VALUES ($1, $2, $3, $4) ON CONFLICT (code) DO NOTHING`,
				permID, perm.Code, perm.Name, perm.Description)

			// Grant to tenant admin role
			var adminRoleID string
			err := tx.QueryRow(ctx,
				`SELECT id FROM roles WHERE tenant_id = $1 AND code = 'admin'`, tenantID).Scan(&adminRoleID)
			if err == nil {
				var pID string
				_ = tx.QueryRow(ctx, `SELECT id FROM permissions WHERE code = $1`, perm.Code).Scan(&pID)
				if pID != "" {
					_, _ = tx.Exec(ctx,
						`INSERT INTO role_permissions (role_id, permission_id)
						 VALUES ($1, $2) ON CONFLICT DO NOTHING`, adminRoleID, pID)
				}
			}

			// Odoo-style additive default groups: managers receive operational
			// read/manage permissions; users receive read/self-service rights.
			// Tenant administrators still bypass checks and their system role is
			// kept complete above.
			for _, grant := range []struct {
				roleCode string
				allowed  bool
			}{
				{roleCode: "manager", allowed: strings.HasSuffix(perm.Code, ".read") || strings.HasSuffix(perm.Code, ".manage") || perm.Code == "gov.process" || perm.Code == "gov.delegate" || perm.Code == "gov.verify" || perm.Code == "gov.report" || perm.Code == "esign.sign"},
				// esign.sign reaches ordinary users deliberately. The authority
				// to sign is the citizen's own — an eID signature is made with
				// their PIN2 on their own phone, and the HSM rail makes them
				// prove a certificate — so withholding it would only stop
				// people signing their own documents. Uploading and configuring
				// remain behind esign.manage.
				{roleCode: "user", allowed: strings.HasSuffix(perm.Code, ".read") || perm.Code == "gov.apply" || perm.Code == "esign.sign"},
			} {
				if !grant.allowed {
					continue
				}
				_, _ = tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id)
					SELECT r.id,p.id FROM roles r JOIN permissions p ON p.code=$3
					WHERE r.tenant_id=$1 AND r.code=$2 AND r.active ON CONFLICT DO NOTHING`, tenantID, grant.roleCode, perm.Code)
			}
		}

		// Log installation event
		evtDetails, _ := json.Marshal(map[string]string{"version": app.Version, "user_id": userID})
		evtID := uuid.New().String()
		_, _ = tx.Exec(ctx,
			`INSERT INTO installation_events (id, installation_id, event_type, details, created_at)
			 VALUES ($1, $2, 'installed', $3, $4)`,
			evtID, installID, evtDetails, now)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit install transaction: %w", err)
	}

	audit.Record(ctx, tenantID, userID, "app.install", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
		"version":  targetApp.Version,
	})

	return nil
}

func (ai *AppInstaller) DisableApp(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("app with slug %q not found", appSlug)
	}

	now := time.Now()
	res, err := ai.db.Exec(ctx,
		`UPDATE app_installations SET enabled = FALSE, status = 'disabled', updated_at = $1
		 WHERE tenant_id = $2 AND app_id = $3`,
		now, tenantID, targetApp.ID)
	if err != nil || res.RowsAffected() == 0 {
		return fmt.Errorf("app %s is not installed for tenant", targetApp.Slug)
	}

	audit.Record(ctx, tenantID, userID, "app.disable", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
	})

	return nil
}

func (ai *AppInstaller) EnableApp(ctx context.Context, tenantID, appSlug, userID string) error {
	targetApp, ok := ai.GetAppBySlug(appSlug)
	if !ok {
		return fmt.Errorf("app with slug %q not found", appSlug)
	}

	now := time.Now()
	res, err := ai.db.Exec(ctx,
		`UPDATE app_installations SET enabled = TRUE, status = 'installed', updated_at = $1
		 WHERE tenant_id = $2 AND app_id = $3`,
		now, tenantID, targetApp.ID)
	if err != nil || res.RowsAffected() == 0 {
		return fmt.Errorf("app %s is not installed for tenant", targetApp.Slug)
	}

	audit.Record(ctx, tenantID, userID, "app.enable", targetApp.ID, map[string]any{
		"app_slug": targetApp.Slug,
	})

	return nil
}

// SyncCatalog mirrors catalog/apps.json into the `apps` table.
//
// app_installations.app_id has a foreign key to apps(id), and the rows used to
// come from a hand-written INSERT in the demo seeder that only listed three of
// the six shipped apps. Installing any of the others — or any app added to the
// catalog later — failed with a foreign-key violation. The catalog file is the
// single source of truth; the table is now derived from it on every boot.
func (ai *AppInstaller) SyncCatalog(ctx context.Context) error {
	for _, app := range ai.catalog {
		_, err := ai.db.Exec(ctx,
			`INSERT INTO apps (id, slug, name, description, icon_url, category, visibility)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT (id) DO UPDATE SET
			     slug        = EXCLUDED.slug,
			     name        = EXCLUDED.name,
			     description = EXCLUDED.description,
			     icon_url    = EXCLUDED.icon_url,
			     category    = EXCLUDED.category,
			     visibility  = EXCLUDED.visibility`,
			app.ID, app.Slug, app.Name, app.Description, app.IconURL, app.Category, app.Visibility)
		if err != nil {
			return fmt.Errorf("sync catalog app %s: %w", app.ID, err)
		}
	}
	return nil
}

// GetInstallationStatesForTenant returns every installed app for the tenant
// mapped to whether it is currently enabled. Presence in the map means
// "installed"; the value means "enabled".
func (ai *AppInstaller) GetInstallationStatesForTenant(ctx context.Context, tenantID string) (map[string]bool, error) {
	rows, err := ai.db.Query(ctx,
		`SELECT app_id, enabled FROM app_installations WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	states := make(map[string]bool)
	for rows.Next() {
		var id string
		var enabled bool
		if err := rows.Scan(&id, &enabled); err != nil {
			return nil, err
		}
		states[id] = enabled
	}
	return states, rows.Err()
}

func (ai *AppInstaller) GetEnabledAppIDsForTenant(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := ai.db.Query(ctx,
		`SELECT app_id FROM app_installations WHERE tenant_id = $1 AND enabled = TRUE`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
