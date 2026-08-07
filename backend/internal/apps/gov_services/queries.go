/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Read models: queues, request lists and the supervisory dashboard.
 */

package gov_services

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TaskFilter drives the queue endpoints. Empty fields are ignored.
type TaskFilter struct {
	Status    string
	UnitID    string
	ServiceID string
	Overdue   bool
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

func (f *TaskFilter) normalise() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 25
	}
}

// ListTasks returns the queue visible to the actor. Scope is applied in SQL, so
// a user can never page past their organisational boundary.
func (m *Module) ListTasks(ctx context.Context, tenantID string, actor Actor, f TaskFilter) (*Page[Task], error) {
	f.normalise()

	where := []string{"t.tenant_id = $1"}
	args := []any{tenantID}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if !actor.IsAdmin {
		if len(actor.UnitIDs) == 0 {
			// No unit assignment means no queue, rather than the whole tenant.
			return &Page[Task]{Items: []Task{}, Page: f.Page, PageSize: f.PageSize}, nil
		}
		add("t.unit_id = ANY($%d)", actor.UnitIDs)
	}
	if f.Status != "" {
		add("t.status = $%d", f.Status)
	}
	if f.UnitID != "" {
		add("t.unit_id = $%d", f.UnitID)
	}
	if f.ServiceID != "" {
		add("a.service_id = $%d", f.ServiceID)
	}
	if f.From != nil {
		add("t.created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("t.created_at <= $%d", *f.To)
	}
	if f.Overdue {
		// Derived, not a stored status.
		where = append(where, "t.due_at < NOW()", fmt.Sprintf("t.status = ANY($%d)", len(args)+1))
		args = append(args, activeTaskStatuses)
	}

	clause := strings.Join(where, " AND ")

	var total int
	if err := m.store.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM gov_tasks t
		   JOIN gov_applications a ON a.id = t.application_id AND a.tenant_id = t.tenant_id
		  WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, err
	}

	// One query with joins; the queue never issues per-row lookups.
	query := `SELECT ` + taskColumns + `, a.reference, s.name, u.name, COALESCE(st.code, '')
		   FROM gov_tasks t
		   JOIN gov_applications a ON a.id = t.application_id AND a.tenant_id = t.tenant_id
		   JOIN gov_services s ON s.id = a.service_id
		   JOIN gov_org_units u ON u.id = t.unit_id AND u.tenant_id = t.tenant_id
		   LEFT JOIN gov_workflow_steps st ON st.id = t.step_id
		  WHERE ` + clause + `
		  ORDER BY t.created_at DESC
		  LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := m.store.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Task, 0, f.PageSize)
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.ApplicationID, &t.ParentTaskID, &t.UnitID,
			&t.AssignedUserID, &t.WorkflowVersionID, &t.StepID, &t.Status, &t.DueAt,
			&t.ResultCode, &t.ResultDescription, &t.CompletedAt, &t.CompletedBy,
			&t.VerifiedAt, &t.VerifiedBy, &t.RowVersion, &t.CreatedAt, &t.UpdatedAt,
			&t.Reference, &t.ServiceName, &t.UnitName, &t.StepCode); err != nil {
			return nil, err
		}
		t.Overdue = isOverdue(&t)
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &Page[Task]{Items: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// RequestDetail is a request with its tasks and timeline.
type RequestDetail struct {
	Application map[string]any `json:"request"`
	Tasks       []Task         `json:"tasks"`
	Timeline    []Event        `json:"timeline"`
}

// RequestDetail loads one request. Scope is checked against the units that hold
// its tasks, so a unit involved anywhere in the chain can follow the request.
func (m *Module) RequestDetail(ctx context.Context, tenantID string, actor Actor, applicationID string) (*RequestDetail, error) {
	var (
		reference, applicant, status, serviceName, mode, source string
		currentUnit                                             *string
		createdAt                                               time.Time
	)
	err := m.store.db.QueryRow(ctx,
		`SELECT a.reference, a.applicant_name, a.status, s.name, a.fulfillment_mode,
		        a.source_system, a.current_unit_id, a.created_at
		   FROM gov_applications a
		   JOIN gov_services s ON s.id = a.service_id
		  WHERE a.id = $1 AND a.tenant_id = $2`, applicationID, tenantID).
		Scan(&reference, &applicant, &status, &serviceName, &mode, &source, &currentUnit, &createdAt)
	if err != nil {
		return nil, notFound(err, "request")
	}

	rows, err := m.store.db.Query(ctx,
		`SELECT `+taskColumns+`, u.name, COALESCE(st.code, '')
		   FROM gov_tasks t
		   JOIN gov_org_units u ON u.id = t.unit_id AND u.tenant_id = t.tenant_id
		   LEFT JOIN gov_workflow_steps st ON st.id = t.step_id
		  WHERE t.application_id = $1 AND t.tenant_id = $2
		  ORDER BY t.created_at`, applicationID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	visible := actor.IsAdmin
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.TenantID, &t.ApplicationID, &t.ParentTaskID, &t.UnitID,
			&t.AssignedUserID, &t.WorkflowVersionID, &t.StepID, &t.Status, &t.DueAt,
			&t.ResultCode, &t.ResultDescription, &t.CompletedAt, &t.CompletedBy,
			&t.VerifiedAt, &t.VerifiedBy, &t.RowVersion, &t.CreatedAt, &t.UpdatedAt,
			&t.UnitName, &t.StepCode); err != nil {
			return nil, err
		}
		t.Overdue = isOverdue(&t)
		if actor.inScope(t.UnitID) {
			visible = true
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !visible {
		return nil, &WorkflowError{Code: "OUT_OF_SCOPE", Message: "the request is outside your organisational scope"}
	}

	timeline, err := m.Timeline(ctx, tenantID, applicationID)
	if err != nil {
		return nil, err
	}

	return &RequestDetail{
		Application: map[string]any{
			"id": applicationID, "reference": reference, "applicant_name": applicant,
			"status": status, "service_name": serviceName, "fulfillment_mode": mode,
			"source_system": source, "current_unit_id": currentUnit, "created_at": createdAt,
		},
		Tasks:    tasks,
		Timeline: timeline,
	}, nil
}

// Timeline returns the append-only history of a request, scoped by tenant
// through the join on gov_applications.
func (m *Module) Timeline(ctx context.Context, tenantID, applicationID string) ([]Event, error) {
	rows, err := m.store.db.Query(ctx,
		`SELECT e.id, e.event_type, e.message, e.actor, e.created_at,
		        COALESCE(e.from_status, ''), COALESCE(e.to_status, ''), COALESCE(u.name, '')
		   FROM gov_application_events e
		   JOIN gov_applications a ON a.id = e.application_id
		   LEFT JOIN gov_org_units u ON u.id = e.unit_id
		  WHERE e.application_id = $1 AND a.tenant_id = $2
		  ORDER BY e.created_at`, applicationID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Event, 0)
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.Message, &e.Actor, &e.CreatedAt,
			&e.FromStatus, &e.ToStatus, &e.UnitName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Dashboard aggregates the actor's authorised scope in a single pass.
func (m *Module) Dashboard(ctx context.Context, tenantID string, actor Actor) (*Dashboard, error) {
	scope := "tenant"
	args := []any{tenantID}
	unitClause := ""
	if !actor.IsAdmin {
		if len(actor.UnitIDs) == 0 {
			return &Dashboard{Scope: "none"}, nil
		}
		scope = "units"
		args = append(args, actor.UnitIDs)
		unitClause = " AND t.unit_id = ANY($2)"
	}

	var d Dashboard
	err := m.store.db.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE t.status = 'RECEIVED'),
		  COUNT(*) FILTER (WHERE t.status IN ('ASSIGNED','IN_PROGRESS')),
		  COUNT(*) FILTER (WHERE t.status = 'FORWARDED'),
		  COUNT(*) FILTER (WHERE t.status = 'AWAITING_VERIFICATION'),
		  COUNT(*) FILTER (WHERE t.status = 'INFO_REQUESTED'),
		  COUNT(*) FILTER (WHERE t.status = 'RETURNED'),
		  COUNT(*) FILTER (WHERE t.status = 'COMPLETED'),
		  COUNT(*) FILTER (WHERE t.status = 'CLOSED'),
		  COUNT(*) FILTER (WHERE t.status = 'REJECTED'),
		  COUNT(*) FILTER (WHERE t.status = 'CANCELLED'),
		  COUNT(*) FILTER (WHERE t.due_at < NOW() AND t.status = ANY($`+fmt.Sprint(len(args)+1)+`)),
		  COUNT(*) FILTER (WHERE t.status = ANY($`+fmt.Sprint(len(args)+1)+`)),
		  COUNT(DISTINCT t.application_id),
		  COUNT(DISTINCT t.application_id) FILTER (WHERE t.status = ANY($`+fmt.Sprint(len(args)+1)+`)),
		  COUNT(DISTINCT t.application_id) FILTER (WHERE t.status IN ('CLOSED','COMPLETED')),
		  COUNT(*) FILTER (WHERE t.status = 'AWAITING_VERIFICATION' AND t.parent_task_id IS NOT NULL)
		FROM gov_tasks t
		WHERE t.tenant_id = $1`+unitClause,
		append(args, activeTaskStatuses)...).
		Scan(&d.Received, &d.InProgress, &d.Delegated, &d.AwaitingVerification, &d.InfoRequested,
			&d.Returned, &d.Completed, &d.Closed, &d.Rejected, &d.Cancelled, &d.Overdue,
			&d.TotalActive, &d.TotalRequests, &d.UnfulfilledRequests, &d.FulfilledRequests,
			&d.VerificationPendingUp)
	if err != nil {
		return nil, err
	}
	d.Scope = scope
	d.UnitCount = len(actor.UnitIDs)
	return &d, nil
}
