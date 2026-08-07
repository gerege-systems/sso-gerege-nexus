package platform

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,62}[a-z0-9]$`)

type accessRole struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Active      bool     `json:"active"`
	System      bool     `json:"system"`
	Permissions []string `json:"permissions"`
}

type accessPermission struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	App         string `json:"app"`
}

type accessMember struct {
	MembershipID string   `json:"membership_id"`
	UserID       string   `json:"user_id"`
	Name         string   `json:"name"`
	Email        string   `json:"email"`
	IsAdmin      bool     `json:"is_admin"`
	Roles        []string `json:"roles"`
}

func (s *Server) handleAccessOverview(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roles := make([]accessRole, 0)
	rows, err := s.db.Query(r.Context(), `
		SELECT r.id::text,r.code,r.name,r.description,r.active,r.is_system,
		       COALESCE(array_agg(p.code ORDER BY p.code) FILTER (WHERE p.code IS NOT NULL),'{}')
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id=r.id
		LEFT JOIN permissions p ON p.id=rp.permission_id
		WHERE r.tenant_id=$1 GROUP BY r.id ORDER BY r.is_system DESC,r.name`, tenantID)
	if err != nil {
		writeJSONError(w, 500, "failed to load roles")
		return
	}
	for rows.Next() {
		var v accessRole
		if err := rows.Scan(&v.ID, &v.Code, &v.Name, &v.Description, &v.Active, &v.System, &v.Permissions); err != nil {
			rows.Close()
			writeJSONError(w, 500, "failed to read roles")
			return
		}
		roles = append(roles, v)
	}
	rows.Close()

	permissions := make([]accessPermission, 0)
	rows, err = s.db.Query(r.Context(), `SELECT code,name,description,split_part(code,'.',1) FROM permissions ORDER BY split_part(code,'.',1),code`)
	if err != nil {
		writeJSONError(w, 500, "failed to load permissions")
		return
	}
	for rows.Next() {
		var v accessPermission
		if err := rows.Scan(&v.Code, &v.Name, &v.Description, &v.App); err != nil {
			rows.Close()
			writeJSONError(w, 500, "failed to read permissions")
			return
		}
		permissions = append(permissions, v)
	}
	rows.Close()

	members := make([]accessMember, 0)
	rows, err = s.db.Query(r.Context(), `
		SELECT m.id::text,u.id::text,u.name,u.email,u.is_admin,
		       COALESCE(array_agg(r.id::text ORDER BY r.name) FILTER (WHERE r.id IS NOT NULL),'{}')
		FROM memberships m JOIN users u ON u.id=m.user_id
		LEFT JOIN membership_roles mr ON mr.membership_id=m.id
		LEFT JOIN roles r ON r.id=mr.role_id AND r.tenant_id=m.tenant_id
		WHERE m.tenant_id=$1 GROUP BY m.id,u.id ORDER BY u.name,u.email`, tenantID)
	if err != nil {
		writeJSONError(w, 500, "failed to load members")
		return
	}
	for rows.Next() {
		var v accessMember
		if err := rows.Scan(&v.MembershipID, &v.UserID, &v.Name, &v.Email, &v.IsAdmin, &v.Roles); err != nil {
			rows.Close()
			writeJSONError(w, 500, "failed to read members")
			return
		}
		members = append(members, v)
	}
	rows.Close()

	writeJSON(w, http.StatusOK, map[string]any{"roles": roles, "permissions": permissions, "members": members})
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSONError(w, 400, "invalid role payload")
		return
	}
	req.Code = strings.ToLower(strings.TrimSpace(req.Code))
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if !roleCodePattern.MatchString(req.Code) || req.Name == "" {
		writeJSONError(w, 400, "role code or name is invalid")
		return
	}
	id := uuid.NewString()
	_, err := s.db.Exec(r.Context(), `INSERT INTO roles(id,tenant_id,code,name,description) VALUES($1,$2,$3,$4,$5)`, id, tenantID, req.Code, req.Name, req.Description)
	if err != nil {
		writeJSONError(w, 409, "role code already exists")
		return
	}
	s.recordAccessChange(r, claims.UserID, "role.create", "role", id, nil, req)
	writeJSON(w, 201, map[string]string{"id": id})
}

func (s *Server) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Active      bool   `json:"active"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		writeJSONError(w, 400, "invalid role payload")
		return
	}
	var system bool
	var beforeName, beforeDesc string
	var beforeActive bool
	if s.db.QueryRow(r.Context(), `SELECT is_system,name,description,active FROM roles WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&system, &beforeName, &beforeDesc, &beforeActive) != nil {
		writeJSONError(w, 404, "role not found")
		return
	}
	if system && !req.Active {
		writeJSONError(w, 409, "system roles cannot be disabled")
		return
	}
	_, err := s.db.Exec(r.Context(), `UPDATE roles SET name=$1,description=$2,active=$3 WHERE id=$4 AND tenant_id=$5`, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), req.Active, id, tenantID)
	if err != nil {
		writeJSONError(w, 500, "failed to update role")
		return
	}
	s.recordAccessChange(r, claims.UserID, "role.update", "role", id, map[string]any{"name": beforeName, "description": beforeDesc, "active": beforeActive}, req)
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var system bool
	var code string
	if s.db.QueryRow(r.Context(), `SELECT is_system,code FROM roles WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&system, &code) != nil {
		writeJSONError(w, 404, "role not found")
		return
	}
	if system {
		writeJSONError(w, 409, "system roles cannot be deleted")
		return
	}
	if _, err := s.db.Exec(r.Context(), `DELETE FROM roles WHERE id=$1 AND tenant_id=$2`, id, tenantID); err != nil {
		writeJSONError(w, 500, "failed to delete role")
		return
	}
	s.recordAccessChange(r, claims.UserID, "role.delete", "role", id, map[string]string{"code": code}, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleSetRolePermissions(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSONError(w, 400, "invalid permission payload")
		return
	}
	var code string
	if s.db.QueryRow(r.Context(), `SELECT code FROM roles WHERE id=$1 AND tenant_id=$2 AND active`, id, tenantID).Scan(&code) != nil {
		writeJSONError(w, 404, "active role not found")
		return
	}
	if code == "admin" {
		writeJSONError(w, 409, "administrator permissions are managed automatically")
		return
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(req.Permissions))
	for _, p := range req.Permissions {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			clean = append(clean, p)
		}
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeJSONError(w, 500, "failed to start permission update")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var valid int
	if len(clean) > 0 {
		if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM permissions WHERE code=ANY($1)`, clean).Scan(&valid); err != nil || valid != len(clean) {
			writeJSONError(w, 400, "one or more permissions are unknown")
			return
		}
	}
	rows, _ := tx.Query(r.Context(), `SELECT p.code FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 ORDER BY p.code`, id)
	before := []string{}
	if rows != nil {
		for rows.Next() {
			var p string
			_ = rows.Scan(&p)
			before = append(before, p)
		}
		rows.Close()
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM role_permissions WHERE role_id=$1`, id); err != nil {
		writeJSONError(w, 500, "failed to clear permissions")
		return
	}
	if len(clean) > 0 {
		_, err = tx.Exec(r.Context(), `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,id FROM permissions WHERE code=ANY($2)`, id, clean)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeJSONError(w, 500, "failed to save permissions")
		return
	}
	s.recordAccessChange(r, claims.UserID, "role.permissions", "role", id, before, clean)
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (s *Server) handleSetMembershipRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, _ := tenant.FromContext(r.Context())
	claims, _ := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Roles []string `json:"roles"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSONError(w, 400, "invalid role assignment payload")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		writeJSONError(w, 500, "failed to start role assignment")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var exists bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM memberships WHERE id=$1 AND tenant_id=$2)`, id, tenantID).Scan(&exists) != nil || !exists {
		writeJSONError(w, 404, "membership not found")
		return
	}
	seen := map[string]bool{}
	clean := []string{}
	for _, v := range req.Roles {
		if !seen[v] {
			seen[v] = true
			clean = append(clean, v)
		}
	}
	var valid int
	if len(clean) > 0 {
		_ = tx.QueryRow(r.Context(), `SELECT count(*) FROM roles WHERE tenant_id=$1 AND active AND id=ANY($2)`, tenantID, clean).Scan(&valid)
		if valid != len(clean) {
			writeJSONError(w, 400, "one or more roles are invalid or inactive")
			return
		}
	}
	rows, _ := tx.Query(r.Context(), `SELECT role_id::text FROM membership_roles WHERE membership_id=$1 ORDER BY role_id`, id)
	before := []string{}
	if rows != nil {
		for rows.Next() {
			var v string
			_ = rows.Scan(&v)
			before = append(before, v)
		}
		rows.Close()
	}
	var removedAdmin bool
	if err = tx.QueryRow(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM membership_roles mr JOIN roles r ON r.id=mr.role_id WHERE mr.membership_id=$1 AND r.code='admin')
		   AND NOT EXISTS(SELECT 1 FROM roles r WHERE r.id=ANY($2) AND r.tenant_id=$3 AND r.code='admin')`, id, clean, tenantID).Scan(&removedAdmin); err != nil {
		writeJSONError(w, 500, "failed to validate administrator assignment")
		return
	}
	if removedAdmin {
		var otherAdmins int
		if err = tx.QueryRow(r.Context(), `SELECT count(DISTINCT mr.membership_id) FROM membership_roles mr JOIN roles r ON r.id=mr.role_id JOIN memberships m ON m.id=mr.membership_id WHERE r.tenant_id=$1 AND r.code='admin' AND r.active AND m.id<>$2`, tenantID, id).Scan(&otherAdmins); err != nil {
			writeJSONError(w, 500, "failed to validate administrators")
			return
		}
		if otherAdmins == 0 {
			writeJSONError(w, http.StatusConflict, "the tenant must keep at least one administrator")
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM membership_roles WHERE membership_id=$1`, id); err == nil && len(clean) > 0 {
		_, err = tx.Exec(r.Context(), `INSERT INTO membership_roles(membership_id,role_id) SELECT $1,id FROM roles WHERE tenant_id=$2 AND active AND id=ANY($3)`, id, tenantID, clean)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeJSONError(w, 500, "failed to assign roles")
		return
	}
	s.recordAccessChange(r, claims.UserID, "membership.roles", "membership", id, before, clean)
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

func (s *Server) recordAccessChange(r *http.Request, actor, action, resource, resourceID string, before, after any) {
	tenantID, _ := tenant.FromContext(r.Context())
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	_, _ = s.db.Exec(r.Context(), `INSERT INTO access_change_events(tenant_id,actor_user_id,action,resource_type,resource_id,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7)`, tenantID, actor, action, resource, resourceID, beforeJSON, afterJSON)
	audit.Record(r.Context(), tenantID, actor, "access."+action, resource, map[string]any{"resource_id": resourceID})
}
