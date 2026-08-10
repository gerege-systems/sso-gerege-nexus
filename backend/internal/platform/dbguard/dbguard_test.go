package dbguard_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/dbguard"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A guard that has not been probed must not touch the connection at all. A
// deployment whose migrations have not reached 00029 keeps working exactly as
// it did, on the application filter alone.
func TestDormantGuardLeavesConnectionsAlone(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:1/none")
	if err != nil {
		t.Fatal(err)
	}
	guard := &dbguard.Guard{}
	guard.Install(cfg)
	if guard.Enabled() {
		t.Fatal("a freshly built guard reports itself enabled")
	}
	if cfg.PrepareConn == nil {
		t.Fatal("Install did not attach a PrepareConn hook")
	}
	// nil connection would panic if the hook tried to talk to it.
	ok, err := cfg.PrepareConn(context.Background(), nil)
	if err != nil {
		t.Fatalf("a dormant guard returned an error: %v", err)
	}
	if !ok {
		t.Fatal("a dormant guard refused a connection")
	}
}

// openGuardedPool builds the pool the API builds, against a migrated database.
func openGuardedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	guard := &dbguard.Guard{}
	guard.Install(cfg)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := guard.Probe(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if !guard.Enabled() {
		t.Skip("the database is not carrying the tenant policies; run migrations to 00029")
	}
	return pool
}

// seedTwoTenants leaves one contact in each of two tenants and returns their ids.
func seedTwoTenants(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var first, second string
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	for i, target := range []*string{&first, &second} {
		slug := "guardtest-" + suffix + "-" + string(rune('a'+i))
		if err := pool.QueryRow(ctx,
			`INSERT INTO tenants (name, slug) VALUES ($1,$1) RETURNING id::text`, slug).Scan(target); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO contacts (id, tenant_id, name, email, phone, company, active)
			 VALUES ($1,$2,$3,'','','',true)`,
			uuid.NewString(), *target, "contact-"+slug); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		// Deleting through the platform path: the sweep is not acting for a tenant.
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = ANY($1)`,
			[]string{first, second})
	})
	return first, second
}

// The whole point, stated as a test: a query with no tenant clause at all —
// the mistake this layer exists to survive — returns only the caller's rows.
func TestQueryWithoutATenantClauseSeesOnlyItsOwnTenant(t *testing.T) {
	pool := openGuardedPool(t)
	first, second := seedTwoTenants(t, pool)

	const noFilter = `SELECT count(*) FROM contacts WHERE name LIKE 'contact-guardtest-%'`

	var visible int
	ctx := tenant.WithTenantID(context.Background(), first)
	if err := pool.QueryRow(ctx, noFilter).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Errorf("acting for the first tenant, an unfiltered query saw %d rows, want 1", visible)
	}

	ctx = tenant.WithTenantID(context.Background(), second)
	if err := pool.QueryRow(ctx, noFilter).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Errorf("acting for the second tenant, an unfiltered query saw %d rows, want 1", visible)
	}

	// No tenant in context is the platform path — signing in, sweeping, syncing
	// the catalogue. It must still see everything, or half the platform breaks.
	if err := pool.QueryRow(context.Background(), noFilter).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 2 {
		t.Errorf("the platform path saw %d rows, want 2", visible)
	}
}

// Reading another tenant's rows is one half; writing into them is the other.
func TestWritingIntoAnotherTenantIsRefused(t *testing.T) {
	pool := openGuardedPool(t)
	first, second := seedTwoTenants(t, pool)

	ctx := tenant.WithTenantID(context.Background(), first)
	_, err := pool.Exec(ctx,
		`INSERT INTO contacts (id, tenant_id, name, email, phone, company, active)
		 VALUES ($1,$2,'planted','','','',true)`, uuid.NewString(), second)
	if err == nil {
		t.Fatal("a tenant inserted a row belonging to another tenant")
	}
	if !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("refused, but not by the policy: %v", err)
	}

	tag, err := pool.Exec(ctx,
		`UPDATE contacts SET name='rewritten' WHERE tenant_id=$1`, second)
	if err != nil {
		t.Fatal(err)
	}
	if tag.RowsAffected() != 0 {
		t.Errorf("updated %d of another tenant's rows, want 0", tag.RowsAffected())
	}
}

// ai_prompts and ai_knowledge carry a NULL tenant_id for the rows the platform
// publishes to everybody, and the copilot reads them with
// `tenant_id IS NULL OR tenant_id = $1`. A policy of bare equality would have
// hidden them — the shared prompt would have stopped being applied, quietly,
// with the copilot still answering.
func TestPlatformWideRowsStayReadableAndCannotBeForged(t *testing.T) {
	pool := openGuardedPool(t)
	first, _ := seedTwoTenants(t, pool)
	key := "guardtest-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10]

	if _, err := pool.Exec(context.Background(),
		`INSERT INTO ai_prompts (tenant_id, prompt_key, content, active) VALUES (NULL,$1,'shared',true)`,
		key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ai_prompts WHERE prompt_key=$1`, key)
	})

	ctx := tenant.WithTenantID(context.Background(), first)
	var content string
	if err := pool.QueryRow(ctx,
		`SELECT content FROM ai_prompts WHERE prompt_key=$1 AND tenant_id IS NULL`, key).Scan(&content); err != nil {
		t.Fatalf("a tenant could not read the shared prompt: %v", err)
	}

	// Readable, but not writable: a tenant that could create a NULL-tenant row
	// would be publishing a system prompt to every other tenant.
	if _, err := pool.Exec(ctx,
		`INSERT INTO ai_prompts (tenant_id, prompt_key, content, active) VALUES (NULL,$1,'planted',true)`,
		key+"-forged"); err == nil {
		t.Error("a tenant created a platform-wide prompt")
	}
}

// Connections are shared, so the binding has to be re-established on every
// acquisition rather than once per connection. Interleaving two tenants over a
// pool small enough to force reuse is what catches a binding that leaks.
func TestConnectionReuseDoesNotLeakTheBinding(t *testing.T) {
	pool := openGuardedPool(t)
	first, second := seedTwoTenants(t, pool)

	for round := range 12 {
		want := first
		if round%2 == 1 {
			want = second
		}
		ctx := tenant.WithTenantID(context.Background(), want)
		var seen string
		if err := pool.QueryRow(ctx,
			`SELECT tenant_id::text FROM contacts WHERE name LIKE 'contact-guardtest-%'`).Scan(&seen); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		if seen != want {
			t.Fatalf("round %d: bound to %s but read a row of %s", round, want, seen)
		}
	}
}
