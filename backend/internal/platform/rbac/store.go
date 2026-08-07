/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * PostgreSQL-backed permission lookup.
 */

package rbac

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLPermissionStore resolves the permission codes granted to a user inside one
// tenant, walking membership → membership_roles → role_permissions.
//
// RequirePermission has existed since the first migration but had no
// implementation behind its PermissionStore interface, so nothing could
// actually enforce a permission. This is that implementation.
type SQLPermissionStore struct {
	db *pgxpool.Pool
}

func NewSQLPermissionStore(db *pgxpool.Pool) *SQLPermissionStore {
	return &SQLPermissionStore{db: db}
}

func (s *SQLPermissionStore) GetUserPermissions(ctx context.Context, tenantID, userID string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx,
		`SELECT p.code
		   FROM memberships m
		   JOIN membership_roles mr ON mr.membership_id = m.id
		   JOIN role_permissions rp ON rp.role_id = mr.role_id
		   JOIN permissions p ON p.id = rp.permission_id
		   JOIN roles r ON r.id = mr.role_id AND r.tenant_id = m.tenant_id AND r.active
		  WHERE m.tenant_id = $1 AND m.user_id = $2`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make(map[string]bool)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms[code] = true
	}
	return perms, rows.Err()
}
