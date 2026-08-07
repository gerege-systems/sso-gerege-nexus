/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Data access for the government service workflow. Hand-written SQL on pgx;
 * every statement is scoped by tenant_id.
 */

package gov_services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// querier lets a helper run inside or outside a transaction. Both
// *pgxpool.Pool and pgx.Tx satisfy it.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type store struct {
	db *pgxpool.Pool
}

// ─── Organisational units ────────────────────────────────────────────────────

func (s *store) listUnits(ctx context.Context, q querier, tenantID string) ([]*OrgUnit, error) {
	rows, err := q.Query(ctx,
		`SELECT id, tenant_id, code, name, name_en, unit_type, parent_id, region_code, active, created_at
		   FROM gov_org_units WHERE tenant_id = $1 ORDER BY unit_type, code`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	units := make([]*OrgUnit, 0)
	for rows.Next() {
		var u OrgUnit
		if err := rows.Scan(&u.ID, &u.TenantID, &u.Code, &u.Name, &u.NameEN, &u.UnitType,
			&u.ParentID, &u.RegionCode, &u.Active, &u.CreatedAt); err != nil {
			return nil, err
		}
		units = append(units, &u)
	}
	return units, rows.Err()
}

func (s *store) createUnit(ctx context.Context, tenantID string, in OrgUnit) (*OrgUnit, error) {
	var u OrgUnit
	err := s.db.QueryRow(ctx,
		`INSERT INTO gov_org_units (tenant_id, code, name, name_en, unit_type, parent_id, region_code)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, tenant_id, code, name, name_en, unit_type, parent_id, region_code, active, created_at`,
		tenantID, in.Code, in.Name, in.NameEN, in.UnitType, in.ParentID, in.RegionCode).
		Scan(&u.ID, &u.TenantID, &u.Code, &u.Name, &u.NameEN, &u.UnitType, &u.ParentID,
			&u.RegionCode, &u.Active, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// unitIDsForUser returns the units a user may act for. A SUPERVISOR also covers
// every descendant of its unit, which is what makes supervisory dashboards and
// verification possible without granting tenant-wide access.
func (s *store) unitIDsForUser(ctx context.Context, tenantID, userID string, graph *UnitGraph) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT unit_id, unit_role FROM gov_unit_members WHERE tenant_id = $1 AND user_id = $2`,
		tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	out := make([]string, 0)
	for rows.Next() {
		var unitID, role string
		if err := rows.Scan(&unitID, &role); err != nil {
			return nil, err
		}
		ids := []string{unitID}
		if role == "SUPERVISOR" {
			ids = graph.Descendants(unitID)
		}
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out, rows.Err()
}

func (s *store) assignUserToUnit(ctx context.Context, tenantID, userID, unitID, role string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO gov_unit_members (tenant_id, user_id, unit_id, unit_role)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (tenant_id, user_id, unit_id) DO UPDATE SET unit_role = EXCLUDED.unit_role`,
		tenantID, userID, unitID, role)
	return err
}

// ─── Workflows ───────────────────────────────────────────────────────────────

func (s *store) listWorkflows(ctx context.Context, tenantID string) ([]Workflow, error) {
	rows, err := s.db.Query(ctx,
		`SELECT w.id, w.tenant_id, w.code, w.name, w.name_en, w.description, w.created_at,
		        v.id, v.version, v.status, v.published_at, v.published_by, v.created_at
		   FROM gov_workflows w
		   LEFT JOIN gov_workflow_versions v ON v.workflow_id = w.id
		  WHERE w.tenant_id = $1
		  ORDER BY w.code, v.version`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// One pass, grouped in memory: no per-workflow follow-up query.
	order := make([]string, 0)
	byID := make(map[string]*Workflow)
	for rows.Next() {
		var w Workflow
		var (
			versionID   *string
			versionNum  *int
			status      *string
			publishedAt *time.Time
			publishedBy *string
			versionMade *time.Time
		)
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Code, &w.Name, &w.NameEN, &w.Description, &w.CreatedAt,
			&versionID, &versionNum, &status, &publishedAt, &publishedBy, &versionMade); err != nil {
			return nil, err
		}
		existing, ok := byID[w.ID]
		if !ok {
			copied := w
			copied.Versions = make([]WorkflowVersion, 0, 1)
			byID[w.ID] = &copied
			order = append(order, w.ID)
			existing = &copied
		}
		if versionID != nil {
			existing.Versions = append(existing.Versions, WorkflowVersion{
				ID: *versionID, TenantID: w.TenantID, WorkflowID: w.ID, Version: *versionNum,
				Status: *status, PublishedAt: publishedAt, PublishedBy: deref(publishedBy),
				CreatedAt: derefTime(versionMade),
			})
		}
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	out := make([]Workflow, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

func (s *store) loadVersion(ctx context.Context, q querier, tenantID, versionID string) (*WorkflowVersion, error) {
	var v WorkflowVersion
	err := q.QueryRow(ctx,
		`SELECT id, tenant_id, workflow_id, version, status, published_at, published_by, created_at
		   FROM gov_workflow_versions WHERE id = $1 AND tenant_id = $2`, versionID, tenantID).
		Scan(&v.ID, &v.TenantID, &v.WorkflowID, &v.Version, &v.Status, &v.PublishedAt, &v.PublishedBy, &v.CreatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(ctx,
		`SELECT id, version_id, code, name, name_en, step_order, step_type, executor_rule,
		        executor_unit_id, executor_config, sla_hours, requires_verification, is_terminal
		   FROM gov_workflow_steps WHERE version_id = $1 AND tenant_id = $2 ORDER BY step_order`,
		versionID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var st WorkflowStep
		if err := rows.Scan(&st.ID, &st.VersionID, &st.Code, &st.Name, &st.NameEN, &st.StepOrder,
			&st.StepType, &st.ExecutorRule, &st.ExecutorUnitID, &st.ExecutorConfig, &st.SLAHours,
			&st.RequiresVerification, &st.IsTerminal); err != nil {
			return nil, err
		}
		v.Steps = append(v.Steps, st)
	}
	return &v, rows.Err()
}

// allowedTransitions returns the narrowing configured by a version, or nil when
// the version configures none (meaning the base state machine applies as is).
func (s *store) allowedTransitions(ctx context.Context, q querier, tenantID, versionID string) (map[transitionKey]string, error) {
	rows, err := q.Query(ctx,
		`SELECT from_status, action, to_status FROM gov_workflow_transitions
		  WHERE version_id = $1 AND tenant_id = $2`, versionID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[transitionKey]string{}
	for rows.Next() {
		var from, action, to string
		if err := rows.Scan(&from, &action, &to); err != nil {
			return nil, err
		}
		out[transitionKey{from, action}] = to
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (s *store) createWorkflowFromTemplate(ctx context.Context, tenantID, actor string, tpl Template, code, name string) (*WorkflowVersion, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if code == "" {
		code = tpl.Code
	}
	if name == "" {
		name = tpl.Name
	}

	var workflowID string
	err = tx.QueryRow(ctx,
		`INSERT INTO gov_workflows (tenant_id, code, name, name_en, description)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (tenant_id, code) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, tenantID, code, name, tpl.NameEN, tpl.Description).Scan(&workflowID)
	if err != nil {
		return nil, err
	}

	// Versions are append-only: a new instantiation always adds the next one.
	var version int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM gov_workflow_versions WHERE workflow_id = $1`,
		workflowID).Scan(&version); err != nil {
		return nil, err
	}

	var versionID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO gov_workflow_versions (tenant_id, workflow_id, version, status)
		 VALUES ($1, $2, $3, 'DRAFT') RETURNING id`, tenantID, workflowID, version).Scan(&versionID); err != nil {
		return nil, err
	}

	for i, step := range tpl.Steps {
		if _, err := tx.Exec(ctx,
			`INSERT INTO gov_workflow_steps (tenant_id, version_id, code, name, name_en, step_order,
			                                 step_type, executor_rule, sla_hours, requires_verification, is_terminal)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			tenantID, versionID, step.Code, step.Name, step.NameEN, i+1, step.StepType,
			step.ExecutorRule, step.SLAHours, step.RequiresVerification, step.IsTerminal); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.loadVersion(ctx, s.db, tenantID, versionID)
}

func (s *store) publishVersion(ctx context.Context, tenantID, versionID, actor string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE gov_workflow_versions
		    SET status = 'PUBLISHED', published_at = NOW(), published_by = $1
		  WHERE id = $2 AND tenant_id = $3 AND status = 'DRAFT'`, actor, versionID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &WorkflowError{Code: "VERSION_NOT_DRAFT", Message: "only a draft version can be published"}
	}
	return nil
}

// ─── Services ────────────────────────────────────────────────────────────────

type serviceConfig struct {
	ID                string
	TenantID          string
	Name              string
	FulfillmentMode   string
	WorkflowVersionID *string
	OwnerUnitID       *string
	Active            bool
}

func (s *store) loadServiceConfig(ctx context.Context, q querier, tenantID, serviceID string) (*serviceConfig, error) {
	var c serviceConfig
	err := q.QueryRow(ctx,
		`SELECT id, tenant_id, name, fulfillment_mode, workflow_version_id, owner_unit_id, active
		   FROM gov_services WHERE id = $1 AND tenant_id = $2`, serviceID, tenantID).
		Scan(&c.ID, &c.TenantID, &c.Name, &c.FulfillmentMode, &c.WorkflowVersionID, &c.OwnerUnitID, &c.Active)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *store) configureService(ctx context.Context, tenantID, serviceID, mode string, versionID, ownerUnitID *string) error {
	// A published version is the only thing a live service may point at.
	if versionID != nil {
		var status string
		if err := s.db.QueryRow(ctx,
			`SELECT status FROM gov_workflow_versions WHERE id = $1 AND tenant_id = $2`,
			*versionID, tenantID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &WorkflowError{Code: "VERSION_NOT_FOUND", Message: "workflow version not found"}
			}
			return err
		}
		if status != VersionPublished {
			return &WorkflowError{Code: "VERSION_NOT_PUBLISHED", Message: "a service can only use a published workflow version"}
		}
	}

	tag, err := s.db.Exec(ctx,
		`UPDATE gov_services
		    SET fulfillment_mode = $1,
		        workflow_version_id = COALESCE($2, workflow_version_id),
		        owner_unit_id = COALESCE($3, owner_unit_id),
		        updated_at = NOW()
		  WHERE id = $4 AND tenant_id = $5`,
		mode, versionID, ownerUnitID, serviceID, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &WorkflowError{Code: "SERVICE_NOT_FOUND", Message: "service not found"}
	}
	return nil
}

// ─── Routing rules ───────────────────────────────────────────────────────────

func (s *store) listRoutingRules(ctx context.Context, q querier, tenantID string) ([]RoutingRule, error) {
	rows, err := q.Query(ctx,
		`SELECT id, tenant_id, service_id, priority, match_field, match_value, strategy,
		        target_unit_id, active, created_at
		   FROM gov_routing_rules WHERE tenant_id = $1 ORDER BY priority, created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RoutingRule, 0)
	for rows.Next() {
		var r RoutingRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.ServiceID, &r.Priority, &r.MatchField, &r.MatchValue,
			&r.Strategy, &r.TargetUnitID, &r.Active, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *store) createRoutingRule(ctx context.Context, tenantID string, in RoutingRule) (*RoutingRule, error) {
	var r RoutingRule
	err := s.db.QueryRow(ctx,
		`INSERT INTO gov_routing_rules (tenant_id, service_id, priority, match_field, match_value,
		                                strategy, target_unit_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, tenant_id, service_id, priority, match_field, match_value, strategy,
		           target_unit_id, active, created_at`,
		tenantID, in.ServiceID, in.Priority, in.MatchField, in.MatchValue, in.Strategy, in.TargetUnitID).
		Scan(&r.ID, &r.TenantID, &r.ServiceID, &r.Priority, &r.MatchField, &r.MatchValue,
			&r.Strategy, &r.TargetUnitID, &r.Active, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ─── Tasks ───────────────────────────────────────────────────────────────────

const taskColumns = `t.id, t.tenant_id, t.application_id, t.parent_task_id, t.unit_id,
	t.assigned_user_id, t.workflow_version_id, t.step_id, t.status, t.due_at,
	t.result_code, t.result_description, t.completed_at, t.completed_by,
	t.verified_at, t.verified_by, t.row_version, t.created_at, t.updated_at`

func scanTask(row pgx.Row, t *Task) error {
	return row.Scan(&t.ID, &t.TenantID, &t.ApplicationID, &t.ParentTaskID, &t.UnitID,
		&t.AssignedUserID, &t.WorkflowVersionID, &t.StepID, &t.Status, &t.DueAt,
		&t.ResultCode, &t.ResultDescription, &t.CompletedAt, &t.CompletedBy,
		&t.VerifiedAt, &t.VerifiedBy, &t.RowVersion, &t.CreatedAt, &t.UpdatedAt)
}

func (s *store) lockTask(ctx context.Context, q querier, tenantID, taskID string) (*Task, error) {
	var t Task
	err := scanTask(q.QueryRow(ctx,
		`SELECT `+taskColumns+` FROM gov_tasks t
		  WHERE t.id = $1 AND t.tenant_id = $2 FOR UPDATE`, taskID, tenantID), &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *store) insertTask(ctx context.Context, q querier, t Task) (string, error) {
	var id string
	err := q.QueryRow(ctx,
		`INSERT INTO gov_tasks (tenant_id, application_id, parent_task_id, unit_id, assigned_user_id,
		                        workflow_version_id, step_id, status, due_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		t.TenantID, t.ApplicationID, t.ParentTaskID, t.UnitID, t.AssignedUserID,
		t.WorkflowVersionID, t.StepID, t.Status, t.DueAt).Scan(&id)
	return id, err
}

// updateTaskStatus applies a transition guarded by the row version the caller
// read. A zero row count means someone else moved the task first.
func (s *store) updateTaskStatus(ctx context.Context, q querier, taskID string, expectedVersion int, patch taskPatch) error {
	tag, err := q.Exec(ctx,
		`UPDATE gov_tasks
		    SET status = $1,
		        assigned_user_id = COALESCE($2, assigned_user_id),
		        result_code = COALESCE($3, result_code),
		        result_description = COALESCE($4, result_description),
		        completed_at = CASE WHEN $5 THEN NOW() ELSE completed_at END,
		        completed_by = CASE WHEN $5 THEN $6 ELSE completed_by END,
		        verified_at = CASE WHEN $7 THEN NOW() ELSE verified_at END,
		        verified_by = CASE WHEN $7 THEN $6 ELSE verified_by END,
		        row_version = row_version + 1,
		        updated_at = NOW()
		  WHERE id = $8 AND row_version = $9`,
		patch.Status, patch.AssignedUserID, patch.ResultCode, patch.ResultDescription,
		patch.MarkCompleted, patch.Actor, patch.MarkVerified, taskID, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return &WorkflowError{
			Code:    "CONFLICT_VERSION",
			Message: "the task changed since it was read; reload and retry",
		}
	}
	return nil
}

type taskPatch struct {
	Status            string
	AssignedUserID    *string
	ResultCode        *string
	ResultDescription *string
	MarkCompleted     bool
	MarkVerified      bool
	Actor             string
}

func (s *store) recordEvent(ctx context.Context, q querier, e timelineEvent) error {
	_, err := q.Exec(ctx,
		`INSERT INTO gov_application_events (application_id, tenant_id, task_id, unit_id,
		                                     event_type, message, actor, from_status, to_status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		e.ApplicationID, e.TenantID, e.TaskID, e.UnitID, e.EventType, e.Message, e.Actor,
		e.FromStatus, e.ToStatus)
	return err
}

type timelineEvent struct {
	ApplicationID string
	TenantID      string
	TaskID        *string
	UnitID        *string
	EventType     string
	Message       string
	Actor         string
	FromStatus    string
	ToStatus      string
}

// syncApplication derives the compatibility status on gov_applications from the
// root task, inside the caller's transaction.
func (s *store) syncApplication(ctx context.Context, q querier, tenantID, applicationID string) error {
	var rootStatus string
	var unitID *string
	err := q.QueryRow(ctx,
		`SELECT status, unit_id FROM gov_tasks
		  WHERE application_id = $1 AND tenant_id = $2 AND parent_task_id IS NULL
		  ORDER BY created_at LIMIT 1`, applicationID, tenantID).Scan(&rootStatus, &unitID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // legacy application without tasks; nothing to derive
		}
		return err
	}

	// The unit currently holding the request is the deepest open task's unit.
	var currentUnit *string
	err = q.QueryRow(ctx,
		`SELECT unit_id FROM gov_tasks
		  WHERE application_id = $1 AND tenant_id = $2 AND status = ANY($3)
		  ORDER BY created_at DESC LIMIT 1`, applicationID, tenantID, activeTaskStatuses).Scan(&currentUnit)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if currentUnit == nil {
		currentUnit = unitID
	}

	status := applicationStatusFor(rootStatus)
	closed := rootStatus == TaskClosed || rootStatus == TaskRejected || rootStatus == TaskCancelled

	_, err = q.Exec(ctx,
		`UPDATE gov_applications
		    SET status = $1,
		        current_unit_id = $2,
		        updated_at = NOW(),
		        decided_at = CASE WHEN $3 AND decided_at IS NULL THEN NOW() ELSE decided_at END,
		        closed_at = CASE WHEN $3 THEN NOW() ELSE closed_at END
		  WHERE id = $4 AND tenant_id = $5`,
		status, currentUnit, closed, applicationID, tenantID)
	return err
}

// enqueueDelivery writes a status update for upstream systems. It runs inside
// the state-change transaction; delivery itself happens later, so a slow or
// broken endpoint can never hold a workflow transition open.
func (s *store) enqueueDelivery(ctx context.Context, q querier, tenantID, applicationID, eventType string, payload []byte, targetURL, secretRef string) error {
	if targetURL == "" {
		return nil
	}
	_, err := q.Exec(ctx,
		`INSERT INTO gov_delivery_outbox (tenant_id, application_id, event_type, payload, target_url, signing_secret_ref)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		tenantID, applicationID, eventType, payload, targetURL, secretRef)
	return err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// notFound turns pgx.ErrNoRows into a domain error with a stable code.
func notFound(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return &WorkflowError{Code: "NOT_FOUND", Message: fmt.Sprintf("%s not found", what)}
	}
	return err
}
