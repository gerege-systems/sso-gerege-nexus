-- Tenant isolation in the database, underneath the WHERE tenant_id every query
-- already carries.
--
-- The application filter is the primary control and stays. This is the second
-- layer: one forgotten clause in one new module handler must not be able to
-- return another tenant's rows. A request runs as gerege_nexus_app with
-- app.current_tenant set (see internal/platform/dbguard); everything that
-- legitimately has no tenant — signing in, resolving a session, the OAuth
-- callback, the housekeeping sweeps — runs as the login role, which is not
-- subject to these policies.
--
-- Every statement below is wrapped in goose statement markers. Goose splits a
-- migration on semicolons unless told not to, and the semicolons inside a
-- DO $$ ... $$ body made the first version of this file fail to apply at all:
-- `migrate up` exited non-zero, and in the production compose the API waits on
-- that container completing successfully, so the whole stack refused to start.

-- +goose Up

-- +goose StatementBegin
DO $role$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gerege_nexus_app') THEN
        CREATE ROLE gerege_nexus_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
    END IF;
END
$role$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO gerege_nexus_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gerege_nexus_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gerege_nexus_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO gerege_nexus_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO gerege_nexus_app;

-- The policy reads, but does not write, rows the platform owns rather than a
-- tenant: ai_prompts and ai_knowledge carry a NULL tenant_id for the entries
-- every tenant shares, and the copilot selects them with
-- `tenant_id IS NULL OR tenant_id = $1`. A USING clause of bare equality would
-- have hidden them — the shared prompt would silently stop being applied.
-- WITH CHECK stays strict equality, so a tenant can never create a row that
-- every other tenant would then read.
-- +goose StatementBegin
DO $rls$
DECLARE target RECORD;
BEGIN
    FOR target IN
        SELECT c.table_name
          FROM information_schema.columns c
          JOIN information_schema.tables t
            ON t.table_schema = c.table_schema AND t.table_name = c.table_name
         WHERE c.table_schema = 'public'
           AND c.column_name = 'tenant_id'
           AND t.table_type = 'BASE TABLE'
    LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', target.table_name);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY', target.table_name);
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON public.%I', target.table_name);
        EXECUTE format(
            'CREATE POLICY tenant_isolation ON public.%I TO gerege_nexus_app '
            'USING (tenant_id IS NULL OR tenant_id = NULLIF(current_setting(''app.current_tenant'', true), '''')::uuid) '
            'WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant'', true), '''')::uuid)',
            target.table_name);
    END LOOP;
END
$rls$;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DO $rls$
DECLARE target RECORD;
BEGIN
    FOR target IN
        SELECT c.table_name
          FROM information_schema.columns c
          JOIN information_schema.tables t
            ON t.table_schema = c.table_schema AND t.table_name = c.table_name
         WHERE c.table_schema = 'public'
           AND c.column_name = 'tenant_id'
           AND t.table_type = 'BASE TABLE'
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON public.%I', target.table_name);
        EXECUTE format('ALTER TABLE public.%I NO FORCE ROW LEVEL SECURITY', target.table_name);
        EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY', target.table_name);
    END LOOP;
END
$rls$;
-- +goose StatementEnd

ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM gerege_nexus_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE USAGE, SELECT ON SEQUENCES FROM gerege_nexus_app;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM gerege_nexus_app;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM gerege_nexus_app;
REVOKE USAGE ON SCHEMA public FROM gerege_nexus_app;

-- A role is a property of the cluster, not of this database, and this cluster
-- hosts sibling stacks. If another database still grants it anything, the DROP
-- fails and takes the rest of this rollback down with it — so the role is left
-- in place and reported instead. It owns nothing and cannot log in.
-- +goose StatementBegin
DO $drop$
BEGIN
    DROP ROLE IF EXISTS gerege_nexus_app;
EXCEPTION WHEN dependent_objects_still_exist OR insufficient_privilege THEN
    RAISE NOTICE 'left role gerege_nexus_app in place: %', SQLERRM;
END
$drop$;
-- +goose StatementEnd
