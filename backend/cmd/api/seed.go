/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, @craftzbay, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Idempotent demo data seeder for local development environments.
 */

package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/config"
)

// resolveCatalogPath locates catalog/apps.json.
//
// The documented quick start is `cd backend && go run ./cmd/api`, but the
// catalog lives at the repository root, so the relative default resolved to
// backend/catalog/apps.json and the server refused to start. The parent
// directory is therefore tried as a fallback before giving up.
func resolveCatalogPath(configured string) string {
	if configured != "" {
		return configured
	}

	const defaultPath = "catalog/apps.json"
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	if parent := filepath.Join("..", defaultPath); fileExists(parent) {
		slog.Info("using app catalog from parent directory", "path", parent)
		return parent
	}
	return defaultPath
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

const (
	demoTenantID = "00000000-0000-0000-0000-000000000001"
	demoUserID   = "00000000-0000-0000-0000-000000000002"
	demoRoleID   = "00000000-0000-0000-0000-000000000003"
	demoEmail    = "admin@example.com"
	demoPassword = "Password123!"
)

// seedingEnabled reports whether the documented demo account should be
// created. The account has a published password, so it is never seeded into a
// production environment unless the operator asks for it explicitly.
func seedingEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SEED_DEMO_DATA"))) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return !config.IsProduction()
}

// seedInitialData creates the demo tenant, admin user, and admin role if they
// are missing. Every statement is idempotent, so it is safe on every boot.
func seedInitialData(ctx context.Context, db *pgxpool.Pool) {
	if !seedingEnabled() {
		return
	}

	tenantID := demoTenantID
	err := db.QueryRow(ctx, `SELECT id::text FROM tenants WHERE slug = 'demo'`).Scan(&tenantID)
	if err != nil {
		if _, err := db.Exec(ctx,
			`INSERT INTO tenants (id, slug, name) VALUES ($1, 'demo', 'Demo Corporation')
			 ON CONFLICT (slug) DO NOTHING`, demoTenantID); err != nil {
			slog.Error("failed to seed demo tenant", "error", err)
			return
		}
		tenantID = demoTenantID
	}

	var userID string
	if err := db.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, demoEmail).Scan(&userID); err == nil {
		ensureDemoMembership(ctx, db, tenantID, userID)
		return
	}

	passHash, err := auth.HashPassword(demoPassword)
	if err != nil {
		slog.Error("failed to hash demo password", "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, name, is_admin)
		 VALUES ($1, $2, $3, 'System Admin', TRUE)
		 ON CONFLICT (email) DO NOTHING`, demoUserID, demoEmail, passHash); err != nil {
		slog.Error("failed to seed admin user", "error", err)
		return
	}

	ensureDemoMembership(ctx, db, tenantID, demoUserID)
	slog.Info("seeded demo account", "email", demoEmail, "tenant", "demo")
}

func ensureDemoMembership(ctx context.Context, db *pgxpool.Pool, tenantID, userID string) {
	if _, err := db.Exec(ctx,
		`INSERT INTO memberships (tenant_id, user_id) VALUES ($1, $2)
		 ON CONFLICT (tenant_id, user_id) DO NOTHING`, tenantID, userID); err != nil {
		slog.Error("failed to seed membership", "error", err)
		return
	}

	if _, err := db.Exec(ctx,
		`INSERT INTO roles (id, tenant_id, code, name) VALUES ($1, $2, 'admin', 'Tenant Admin')
		 ON CONFLICT (tenant_id, code) DO NOTHING`, demoRoleID, tenantID); err != nil {
		slog.Error("failed to seed admin role", "error", err)
		return
	}

	var membershipID, roleID string
	if err := db.QueryRow(ctx,
		`SELECT id::text FROM memberships WHERE tenant_id = $1 AND user_id = $2`,
		tenantID, userID).Scan(&membershipID); err != nil {
		return
	}
	if err := db.QueryRow(ctx,
		`SELECT id::text FROM roles WHERE tenant_id = $1 AND code = 'admin'`, tenantID).Scan(&roleID); err != nil {
		return
	}

	if _, err := db.Exec(ctx,
		`INSERT INTO membership_roles (membership_id, role_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, membershipID, roleID); err != nil {
		slog.Error("failed to grant admin role", "error", err)
	}
}
