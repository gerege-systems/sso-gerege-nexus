package rbac_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedMember leaves one tenant holding one member with one role, and returns
// the ids plus a way to grant that role a permission.
func seedMember(t *testing.T, pool *pgxpool.Pool) (tenantID, userID, roleID, permission string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1,$1) RETURNING id::text`,
		"rbactest-"+suffix).Scan(&tenantID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1,'x','t') RETURNING id::text`,
		"rbactest-"+suffix+"@example.mn").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var membershipID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO memberships (tenant_id, user_id) VALUES ($1,$2) RETURNING id::text`,
		tenantID, userID).Scan(&membershipID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO roles (tenant_id, code, name) VALUES ($1,'cachetest','Cache Test') RETURNING id::text`,
		tenantID).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO membership_roles (membership_id, role_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`, membershipID, roleID); err != nil {
		t.Fatal(err)
	}

	// The permission is created here rather than borrowed from the seeded set:
	// migrations seed three, and every other code in the platform is registered
	// by the app installer when a module is installed. A test that reached for
	// contacts.read passed or failed on whether an app happened to be installed
	// in whichever database it was pointed at.
	permission = "cachetest." + suffix
	if _, err := pool.Exec(ctx,
		`INSERT INTO permissions (code, name) VALUES ($1,$1)`, permission); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM permissions WHERE code=$1`, permission)
	})
	return tenantID, userID, roleID, permission
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// The grant is cached, so revoking one has to be announced. This is the test
// that matters: without the invalidation an administrator takes a permission
// away, the screen says it is gone, and the member keeps using it.
func TestRevokingAPermissionTakesEffectOnceTheTenantIsInvalidated(t *testing.T) {
	pool := openPool(t)
	tenantID, userID, roleID, permission := seedMember(t, pool)
	ctx := context.Background()
	store := rbac.NewSQLPermissionStore(pool)

	grant := func() {
		if _, err := pool.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id)
			 SELECT $1, id FROM permissions WHERE code=$2 ON CONFLICT DO NOTHING`,
			roleID, permission); err != nil {
			t.Fatal(err)
		}
	}
	revoke := func() {
		if _, err := pool.Exec(ctx,
			`DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
			t.Fatal(err)
		}
	}

	grant()
	rbac.InvalidateTenant(tenantID)
	perms, err := store.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !perms[permission] {
		t.Fatal("the granted permission was not returned")
	}

	// Revoked in the database, but nothing has told the cache.
	revoke()
	stale, err := store.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !stale[permission] {
		t.Log("the revocation was already visible; the entry had expired")
	}

	rbac.InvalidateTenant(tenantID)
	fresh, err := store.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh[permission] {
		t.Error("the permission survived the tenant being invalidated")
	}
}

// The cached map goes out to callers that hold on to it for the length of a
// request. If they shared one, a handler that wrote to it would be granting a
// permission to everybody else holding the same entry.
func TestCallersCannotWriteIntoEachOthersGrants(t *testing.T) {
	pool := openPool(t)
	tenantID, userID, roleID, permission := seedMember(t, pool)
	ctx := context.Background()
	store := rbac.NewSQLPermissionStore(pool)

	if _, err := pool.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission_id)
		 SELECT $1, id FROM permissions WHERE code=$2 ON CONFLICT DO NOTHING`,
		roleID, permission); err != nil {
		t.Fatal(err)
	}
	rbac.InvalidateTenant(tenantID)

	first, err := store.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	first[permission+".written"] = true // a caller mutating what it was handed

	second, err := store.GetUserPermissions(ctx, tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if second[permission+".written"] {
		t.Error("one caller's write to its grant map reached the next caller")
	}
}
