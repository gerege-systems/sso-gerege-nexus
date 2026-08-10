-- +goose Up

-- A session is authority inside one tenant, so the database must guarantee
-- that its user actually belongs to that tenant.
DELETE FROM sessions s
WHERE NOT EXISTS (
    SELECT 1 FROM memberships m
    WHERE m.tenant_id = s.tenant_id AND m.user_id = s.user_id
);
ALTER TABLE sessions
    ADD CONSTRAINT sessions_membership_fk
    FOREIGN KEY (tenant_id, user_id)
    REFERENCES memberships (tenant_id, user_id) ON DELETE CASCADE;

INSERT INTO permissions (code, name, description) VALUES
    ('xyp.citizen.read', 'Query citizen registry', 'Query authoritative citizen data through XYP'),
    ('xyp.company.read', 'Query company registry', 'Query authoritative company data through XYP')
ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code IN ('xyp.citizen.read','xyp.company.read')
ON CONFLICT DO NOTHING;

-- Legacy users.is_admin was global and therefore made a user administrator in
-- every tenant they later joined. Tenant administration now comes only from
-- that membership's active admin role. The legacy value is retained as data
-- for a later explicit platform-admin migration, but no authorization query
-- consumes it anymore.

-- +goose Down
DELETE FROM role_permissions rp USING permissions p
WHERE rp.permission_id=p.id AND p.code IN ('xyp.citizen.read','xyp.company.read');
DELETE FROM permissions WHERE code IN ('xyp.citizen.read','xyp.company.read');
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_membership_fk;
