package tenant_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Every module table carrying tenant_id must be protected. This table-driven
// database invariant covers current and future modules without allowing a new
// tenant-scoped table to silently ship without a policy.
func TestEveryTenantTableHasForcedRLS(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	rows, err := pool.Query(context.Background(), `
		SELECT c.table_name, cls.relrowsecurity, cls.relforcerowsecurity,
		       EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname='public' AND p.tablename=c.table_name AND p.policyname='tenant_isolation')
		FROM information_schema.columns c
		JOIN pg_class cls ON cls.relname=c.table_name
		JOIN pg_namespace n ON n.oid=cls.relnamespace AND n.nspname='public'
		WHERE c.table_schema='public' AND c.column_name='tenant_id'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		var enabled, forced, policy bool
		if err := rows.Scan(&table, &enabled, &forced, &policy); err != nil {
			t.Fatal(err)
		}
		if !enabled || !forced || !policy {
			t.Errorf("%s: RLS enabled=%v forced=%v policy=%v", table, enabled, forced, policy)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
