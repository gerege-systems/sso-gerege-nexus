/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * HTTP handlers for the configurable workflow: configuration, ingestion,
 * queues, actions and dashboards.
 */

package gov_services

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/audit"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
)

// actorFrom resolves the caller into permissions and organisational scope. It
// is the single place authorisation context is built, so no handler can forget
// to apply it.
func (m *Module) actorFrom(r *http.Request) (string, Actor, error) {
	tenantID, err := tenant.FromContext(r.Context())
	if err != nil {
		return "", Actor{}, &WorkflowError{Code: "UNAUTHORIZED", Message: "unauthorized", Status: http.StatusUnauthorized}
	}
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		return "", Actor{}, &WorkflowError{Code: "UNAUTHORIZED", Message: "unauthorized", Status: http.StatusUnauthorized}
	}

	actor := Actor{UserID: claims.UserID, Email: claims.Email, IsAdmin: claims.IsAdmin}
	if !claims.IsAdmin {
		perms, err := m.perms.GetUserPermissions(r.Context(), tenantID, claims.UserID)
		if err != nil {
			return "", Actor{}, err
		}
		actor.Perms = perms

		units, err := m.store.listUnits(r.Context(), m.store.db, tenantID)
		if err != nil {
			return "", Actor{}, err
		}
		scope, err := m.store.unitIDsForUser(r.Context(), tenantID, claims.UserID, NewUnitGraph(units))
		if err != nil {
			return "", Actor{}, err
		}
		actor.UnitIDs = scope
	}
	return tenantID, actor, nil
}

// require is the guard every handler starts with.
func (m *Module) require(w http.ResponseWriter, r *http.Request, permission string) (string, Actor, bool) {
	tenantID, actor, err := m.actorFrom(r)
	if err != nil {
		writeDomainError(w, err)
		return "", Actor{}, false
	}
	if !actor.can(permission) {
		writeDomainError(w, &WorkflowError{Code: "FORBIDDEN", Message: "permission " + permission + " required"})
		return "", Actor{}, false
	}
	return tenantID, actor, true
}

// ─── Organisational units ────────────────────────────────────────────────────

func (m *Module) listUnitsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	units, err := m.store.listUnits(r.Context(), m.store.db, tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, units)
}

func (m *Module) unitTreeHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	units, err := m.store.listUnits(r.Context(), m.store.db, tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	byID := make(map[string]*OrgUnit, len(units))
	for _, u := range units {
		u.Children = []*OrgUnit{}
		byID[u.ID] = u
	}
	roots := make([]*OrgUnit, 0)
	for _, u := range units {
		if u.ParentID != nil {
			if parent, ok := byID[*u.ParentID]; ok {
				parent.Children = append(parent.Children, u)
				continue
			}
		}
		roots = append(roots, u)
	}
	httpx.JSON(w, http.StatusOK, roots)
}

func (m *Module) createUnitHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}

	var in OrgUnit
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Code == "" || in.Name == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "code and name are required"})
		return
	}
	if in.UnitType == "" {
		in.UnitType = "DEPARTMENT"
	}

	unit, err := m.store.createUnit(r.Context(), tenantID, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.unit_created", unit.ID, map[string]any{"code": unit.Code})
	httpx.JSON(w, http.StatusCreated, unit)
}

func (m *Module) assignUnitMemberHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}

	var in struct {
		UserID   string `json:"user_id"`
		UnitID   string `json:"unit_id"`
		UnitRole string `json:"unit_role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.UserID == "" || in.UnitID == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "user_id and unit_id are required"})
		return
	}
	if in.UnitRole == "" {
		in.UnitRole = "OFFICER"
	}
	if in.UnitRole != "OFFICER" && in.UnitRole != "SUPERVISOR" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "unit_role must be OFFICER or SUPERVISOR"})
		return
	}

	if err := m.store.assignUserToUnit(r.Context(), tenantID, in.UserID, in.UnitID, in.UnitRole); err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.unit_member_assigned", in.UnitID,
		map[string]any{"user_id": in.UserID, "role": in.UnitRole})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// ─── Workflow configuration ──────────────────────────────────────────────────

func (m *Module) listTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := m.require(w, r, PermRead); !ok {
		return
	}
	httpx.JSON(w, http.StatusOK, Templates)
}

func (m *Module) listWorkflowsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	workflows, err := m.store.listWorkflows(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, workflows)
}

func (m *Module) createWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}

	var in struct {
		Template string `json:"template"`
		Code     string `json:"code"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Template == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "template is required"})
		return
	}
	tpl, ok := TemplateByCode(in.Template)
	if !ok {
		writeDomainError(w, &WorkflowError{Code: "TEMPLATE_NOT_FOUND", Message: "unknown workflow template " + in.Template})
		return
	}

	version, err := m.store.createWorkflowFromTemplate(r.Context(), tenantID, actor.Email, tpl, in.Code, in.Name)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.workflow_created", version.WorkflowID,
		map[string]any{"template": in.Template, "version": version.Version})
	httpx.JSON(w, http.StatusCreated, version)
}

func (m *Module) getVersionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	version, err := m.store.loadVersion(r.Context(), m.store.db, tenantID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, notFound(err, "workflow version"))
		return
	}
	httpx.JSON(w, http.StatusOK, version)
}

func (m *Module) publishVersionHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}
	versionID := chi.URLParam(r, "id")
	if err := m.store.publishVersion(r.Context(), tenantID, versionID, actor.Email); err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.workflow_published", versionID, nil)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": VersionPublished, "id": versionID})
}

func (m *Module) configureServiceHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}

	var in struct {
		FulfillmentMode   string  `json:"fulfillment_mode"`
		WorkflowVersionID *string `json:"workflow_version_id"`
		OwnerUnitID       *string `json:"owner_unit_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "invalid configuration payload"})
		return
	}
	switch in.FulfillmentMode {
	case ModeLocal, ModeDelegate, ModeHybrid:
	default:
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "fulfillment_mode must be LOCAL, DELEGATE or HYBRID"})
		return
	}

	serviceID := chi.URLParam(r, "id")
	if err := m.store.configureService(r.Context(), tenantID, serviceID, in.FulfillmentMode, in.WorkflowVersionID, in.OwnerUnitID); err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.service_configured", serviceID,
		map[string]any{"mode": in.FulfillmentMode})
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "configured", "id": serviceID})
}

func (m *Module) listRoutingRulesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	rules, err := m.store.listRoutingRules(r.Context(), m.store.db, tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rules)
}

func (m *Module) createRoutingRuleHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}

	var in RoutingRule
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Strategy == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "strategy is required"})
		return
	}
	switch in.Strategy {
	case RouteSelf, RouteParent, RouteChild, RouteSpecificUnit, RouteRegionMatch:
	default:
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "unknown routing strategy"})
		return
	}
	if in.Strategy == RouteSpecificUnit && in.TargetUnitID == nil {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "SPECIFIC_UNIT requires target_unit_id"})
		return
	}

	rule, err := m.store.createRoutingRule(r.Context(), tenantID, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.routing_rule_created", rule.ID, nil)
	httpx.JSON(w, http.StatusCreated, rule)
}

// ─── Ingestion ───────────────────────────────────────────────────────────────

func (m *Module) ingestHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermApply)
	if !ok {
		return
	}

	var in IngestInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "invalid ingestion payload"})
		return
	}

	result, err := m.Ingest(r.Context(), tenantID, actor, in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
		audit.Record(r.Context(), tenantID, actor.Email, "gov.request_ingested", in.ExternalRequestID,
			map[string]any{"source": in.SourceSystem, "service": in.ServiceCode})
	}
	httpx.JSON(w, status, result)
}

// ─── Queues, detail and actions ──────────────────────────────────────────────

func (m *Module) listTasksHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}

	q := r.URL.Query()
	filter := TaskFilter{
		Status:    q.Get("status"),
		UnitID:    q.Get("unit_id"),
		ServiceID: q.Get("service_id"),
		Overdue:   q.Get("overdue") == "true",
	}
	filter.Page, _ = strconv.Atoi(q.Get("page"))
	filter.PageSize, _ = strconv.Atoi(q.Get("page_size"))
	if from := q.Get("from"); from != "" {
		if parsed, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = &parsed
		}
	}
	if to := q.Get("to"); to != "" {
		if parsed, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = &parsed
		}
	}

	page, err := m.ListTasks(r.Context(), tenantID, actor, filter)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, page)
}

func (m *Module) requestDetailHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	detail, err := m.RequestDetail(r.Context(), tenantID, actor, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, detail)
}

func (m *Module) actHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, err := m.actorFrom(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var in ActionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Action == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "action is required"})
		return
	}

	task, err := m.Act(r.Context(), tenantID, actor, chi.URLParam(r, "id"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	audit.Record(r.Context(), tenantID, actor.Email, "gov.task_"+in.Action, task.ID,
		map[string]any{"status": task.Status, "unit_id": task.UnitID})
	httpx.JSON(w, http.StatusOK, task)
}

func (m *Module) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermReport)
	if !ok {
		return
	}
	summary, err := m.Dashboard(r.Context(), tenantID, actor)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

// ─── Outbox ──────────────────────────────────────────────────────────────────

func (m *Module) listOutboxHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermConfigure)
	if !ok {
		return
	}

	rows, err := m.store.db.Query(r.Context(),
		`SELECT id, application_id, event_type, status, attempts, last_error, created_at, delivered_at
		   FROM gov_delivery_outbox WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 100`, tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer rows.Close()

	type outboxRow struct {
		ID            string     `json:"id"`
		ApplicationID string     `json:"application_id"`
		EventType     string     `json:"event_type"`
		Status        string     `json:"status"`
		Attempts      int        `json:"attempts"`
		LastError     string     `json:"last_error,omitempty"`
		CreatedAt     time.Time  `json:"created_at"`
		DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	}
	// The payload and the secret reference are deliberately not returned.
	list := make([]outboxRow, 0)
	for rows.Next() {
		var o outboxRow
		if err := rows.Scan(&o.ID, &o.ApplicationID, &o.EventType, &o.Status, &o.Attempts,
			&o.LastError, &o.CreatedAt, &o.DeliveredAt); err != nil {
			writeDomainError(w, err)
			return
		}
		list = append(list, o)
	}
	if err := rows.Err(); err != nil {
		writeDomainError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

// ─── Error rendering ─────────────────────────────────────────────────────────

// writeDomainError renders a stable machine code next to the human message.
func writeDomainError(w http.ResponseWriter, err error) {
	var domain *WorkflowError
	if errors.As(err, &domain) {
		status := domain.Status
		if status == 0 {
			status = statusForCode(domain.Code)
		}
		httpx.JSON(w, status, map[string]string{"error": domain.Message, "code": domain.Code})
		return
	}
	httpx.JSON(w, http.StatusInternalServerError, map[string]string{
		"error": "internal error", "code": "INTERNAL",
	})
}

func statusForCode(code string) int {
	switch code {
	case "UNAUTHORIZED":
		return http.StatusUnauthorized
	case "FORBIDDEN", "OUT_OF_SCOPE":
		return http.StatusForbidden
	case "NOT_FOUND", "SERVICE_NOT_FOUND", "VERSION_NOT_FOUND", "TEMPLATE_NOT_FOUND", "UNIT_NOT_FOUND":
		return http.StatusNotFound
	case "CONFLICT_VERSION", "IDEMPOTENCY_CONFLICT", "TASK_TERMINAL":
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
