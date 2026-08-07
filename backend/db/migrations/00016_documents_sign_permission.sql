-- +goose Up

-- The documents module declares documents.sign, but permission rows are only
-- written when an app is installed (appinstaller.InstallApp, ON CONFLICT DO
-- NOTHING) and the boot-time catalog sync does not touch them. A tenant that
-- already had the documents app installed therefore never sees the permission:
-- it is absent from /settings/access, and setRolePermissions refuses a code that
-- is not in the table. Tenant administrators bypass permission checks, so the
-- gap is invisible to whoever tests as one and total for everyone else — the
-- Sign and Reject actions simply never appear.
--
-- On a fresh database this is a no-op: no tenants exist yet, and the install
-- path writes the row itself.
INSERT INTO permissions (id, code, name, description)
VALUES (gen_random_uuid(), 'documents.sign', 'Sign Documents',
        'Apply an E-ID / DAN digital signature or reject a document')
    ON CONFLICT (code) DO NOTHING;

-- Grant it the way an install would: to the admin role of every tenant that has
-- the documents app, and to nobody else. Handing it to each documents.manage
-- holder would undo the separation it exists for — drafting a document is not
-- authority to sign it. A tenant administrator can now pass it on.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
  FROM roles r
  JOIN app_installations ai
    ON ai.tenant_id = r.tenant_id AND ai.app_id = 'io.example.documents'
  JOIN permissions p ON p.code = 'documents.sign'
 WHERE r.code = 'admin'
    ON CONFLICT DO NOTHING;

-- +goose Down

-- Deliberately empty. The Up above cannot tell a row it created from one an
-- install created — both are ON CONFLICT DO NOTHING — so removing the permission
-- or the grant on the way down would delete something this migration may never
-- have added, and with it whatever a tenant administrator granted since.
SELECT 1;
