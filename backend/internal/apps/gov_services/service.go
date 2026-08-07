/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Transactional orchestration: ingestion, the workflow driver and reporting.
 */

package gov_services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Actor is the authenticated caller, already resolved to its permissions and
// organisational scope.
type Actor struct {
	UserID  string
	Email   string
	IsAdmin bool
	Perms   map[string]bool
	// UnitIDs is the set of units the actor may act for. Empty for an admin,
	// who is scoped to the whole tenant instead.
	UnitIDs []string
}

func (a Actor) can(permission string) bool {
	if a.IsAdmin {
		return true
	}
	return a.Perms[permission]
}

// inScope reports whether the actor may act on work held by unitID.
func (a Actor) inScope(unitID string) bool {
	if a.IsAdmin {
		return true
	}
	return slices.Contains(a.UnitIDs, unitID)
}

// ─── Ingestion ───────────────────────────────────────────────────────────────

// IngestInput is one upstream service request.
type IngestInput struct {
	SourceSystem      string            `json:"source_system"`
	ExternalRequestID string            `json:"external_request_id"`
	ServiceCode       string            `json:"service_code"`
	ApplicantName     string            `json:"applicant_name"`
	ApplicantReg      string            `json:"applicant_reg"`
	ApplicantPhone    string            `json:"applicant_phone"`
	Note              string            `json:"note"`
	OriginUnitCode    string            `json:"origin_unit_code"`
	Fields            map[string]string `json:"fields"`
}

// fingerprint is the identity of the payload behind an external request id. A
// retry carrying the same id must carry the same fingerprint; anything else is
// a conflicting replay rather than a duplicate delivery.
func (in IngestInput) fingerprint() string {
	keys := make([]string, 0, len(in.Fields))
	for k := range in.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s|%s|%s|%s|%s",
		in.SourceSystem, in.ExternalRequestID, in.ServiceCode, in.ApplicantName,
		in.ApplicantReg, in.ApplicantPhone, in.OriginUnitCode)
	for _, k := range keys {
		fmt.Fprintf(&b, "|%s=%s", k, in.Fields[k])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// IngestResult reports whether the call created the request or replayed one.
type IngestResult struct {
	Application map[string]any `json:"request"`
	Created     bool           `json:"created"`
}

// Ingest creates exactly one request and its initial task for a given
// (tenant, source system, external id). Retries return the existing request
// without creating a second task or timeline entry.
func (m *Module) Ingest(ctx context.Context, tenantID string, actor Actor, in IngestInput) (*IngestResult, error) {
	if !actor.can(PermApply) {
		return nil, &WorkflowError{Code: "FORBIDDEN", Message: "permission " + PermApply + " required"}
	}
	if in.ServiceCode == "" || in.ApplicantName == "" {
		return nil, &WorkflowError{Code: "INVALID_INPUT", Message: "service_code and applicant_name are required"}
	}
	if in.SourceSystem == "" {
		in.SourceSystem = "UPSTREAM"
	}
	if in.ExternalRequestID == "" {
		return nil, &WorkflowError{Code: "INVALID_INPUT", Message: "external_request_id is required for upstream ingestion"}
	}

	fingerprint := in.fingerprint()

	tx, err := m.store.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Idempotency: the unique index on (tenant, source, external id) is the
	// authority; this read makes the happy path cheap and lets us tell a retry
	// apart from a conflicting replay.
	var existingID, existingFingerprint, existingRef string
	err = tx.QueryRow(ctx,
		`SELECT id, payload_fingerprint, reference FROM gov_applications
		  WHERE tenant_id = $1 AND source_system = $2 AND external_request_id = $3`,
		tenantID, in.SourceSystem, in.ExternalRequestID).Scan(&existingID, &existingFingerprint, &existingRef)
	if err == nil {
		if existingFingerprint != "" && existingFingerprint != fingerprint {
			return nil, &WorkflowError{
				Code:    "IDEMPOTENCY_CONFLICT",
				Message: "external_request_id was already used with a different payload",
			}
		}
		return &IngestResult{
			Application: map[string]any{"id": existingID, "reference": existingRef},
			Created:     false,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var svc serviceConfig
	err = tx.QueryRow(ctx,
		`SELECT id, tenant_id, name, fulfillment_mode, workflow_version_id, owner_unit_id, active
		   FROM gov_services WHERE tenant_id = $1 AND code = $2`, tenantID, in.ServiceCode).
		Scan(&svc.ID, &svc.TenantID, &svc.Name, &svc.FulfillmentMode, &svc.WorkflowVersionID, &svc.OwnerUnitID, &svc.Active)
	if err != nil {
		return nil, notFound(err, "service "+in.ServiceCode)
	}
	if !svc.Active {
		return nil, &WorkflowError{Code: "SERVICE_INACTIVE", Message: "service is no longer offered"}
	}
	if svc.WorkflowVersionID == nil || svc.OwnerUnitID == nil {
		return nil, &WorkflowError{
			Code:    "SERVICE_NOT_CONFIGURED",
			Message: "service has no published workflow or owning unit configured",
		}
	}

	fields, _ := json.Marshal(in.Fields)
	reference := fmt.Sprintf("GS-%d", time.Now().UnixNano()/1e6)

	// Where the request enters the hierarchy.
	originUnit := *svc.OwnerUnitID
	if in.OriginUnitCode != "" {
		var id string
		if err := tx.QueryRow(ctx,
			`SELECT id FROM gov_org_units WHERE tenant_id = $1 AND code = $2 AND active`,
			tenantID, in.OriginUnitCode).Scan(&id); err != nil {
			return nil, notFound(err, "origin unit "+in.OriginUnitCode)
		}
		originUnit = id
	}

	var applicationID string
	err = tx.QueryRow(ctx,
		`INSERT INTO gov_applications (tenant_id, service_id, reference, applicant_name, applicant_reg,
		                               applicant_phone, note, status, workflow_version_id, fulfillment_mode,
		                               source_system, external_request_id, payload_fingerprint,
		                               request_payload, origin_unit_id, current_unit_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'SUBMITTED', $8, $9, $10, $11, $12, $13, $14, $14)
		 RETURNING id`,
		tenantID, svc.ID, reference, in.ApplicantName, in.ApplicantReg, in.ApplicantPhone, in.Note,
		*svc.WorkflowVersionID, svc.FulfillmentMode, in.SourceSystem, in.ExternalRequestID,
		fingerprint, fields, originUnit).Scan(&applicationID)
	if err != nil {
		return nil, err
	}

	if err := m.createInitialTask(ctx, tx, tenantID, applicationID, originUnit, svc, in.Fields, actor.Email); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &IngestResult{
		Application: map[string]any{"id": applicationID, "reference": reference},
		Created:     true,
	}, nil
}

// createInitialTask places the request on the first step of its workflow.
func (m *Module) createInitialTask(ctx context.Context, tx pgx.Tx, tenantID, applicationID, originUnit string, svc serviceConfig, fields map[string]string, actor string) error {
	version, err := m.store.loadVersion(ctx, tx, tenantID, *svc.WorkflowVersionID)
	if err != nil {
		return notFound(err, "workflow version")
	}
	if len(version.Steps) == 0 {
		return &WorkflowError{Code: "WORKFLOW_EMPTY", Message: "the workflow version has no steps"}
	}
	first := version.Steps[0]

	units, err := m.store.listUnits(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	graph := NewUnitGraph(units)
	rules, err := m.store.listRoutingRules(ctx, tx, tenantID)
	if err != nil {
		return err
	}

	// HYBRID lets a routing rule pick the route; LOCAL and DELEGATE follow the
	// step's own executor rule.
	strategy := first.ExecutorRule
	target := ""
	if svc.FulfillmentMode == ModeHybrid {
		if rule, ok := SelectRule(rules, svc.ID, fields); ok {
			strategy = rule.Strategy
			if rule.TargetUnitID != nil {
				target = *rule.TargetUnitID
			}
		}
	}
	if first.ExecutorUnitID != nil && target == "" {
		target = *first.ExecutorUnitID
	}

	unitID, err := graph.Resolve(strategy, RoutingRequest{
		CurrentUnitID: originUnit,
		TargetUnitID:  target,
		Fields:        fields,
	}, rules, svc.ID)
	if err != nil {
		return err
	}

	due := time.Now().Add(time.Duration(first.SLAHours) * time.Hour)
	taskID, err := m.store.insertTask(ctx, tx, Task{
		TenantID:          tenantID,
		ApplicationID:     applicationID,
		UnitID:            unitID,
		WorkflowVersionID: &version.ID,
		StepID:            &first.ID,
		Status:            TaskReceived,
		DueAt:             &due,
	})
	if err != nil {
		return err
	}

	return m.store.recordEvent(ctx, tx, timelineEvent{
		ApplicationID: applicationID, TenantID: tenantID, TaskID: &taskID, UnitID: &unitID,
		EventType: "received", Message: "Хүсэлт хүлээн авлаа", Actor: actor,
		FromStatus: "", ToStatus: TaskReceived,
	})
}

// ─── Workflow driver ─────────────────────────────────────────────────────────

// ActionInput carries the client's intent. The resulting status is decided
// here, never taken from the request body.
type ActionInput struct {
	Action       string `json:"action"`
	Comment      string `json:"comment"`
	ResultCode   string `json:"result_code"`
	TargetUnitID string `json:"target_unit_id"`
	AssignUserID string `json:"assigned_user_id"`
	// RowVersion is the version the client last read. Omitting it is refused,
	// so a stale UI can never overwrite a concurrent decision.
	RowVersion int `json:"row_version"`
}

// Act runs one transition atomically: validate, apply, cascade, record.
func (m *Module) Act(ctx context.Context, tenantID string, actor Actor, taskID string, in ActionInput) (*Task, error) {
	if in.RowVersion <= 0 {
		return nil, &WorkflowError{Code: "ROW_VERSION_REQUIRED", Message: "row_version is required for a state change"}
	}

	tx, err := m.store.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	task, err := m.store.lockTask(ctx, tx, tenantID, taskID)
	if err != nil {
		return nil, notFound(err, "task")
	}

	// Verification and return are exercised by the level above, so the scope
	// check follows the parent task when there is one.
	scopeUnit := task.UnitID
	if in.Action == ActionVerify || in.Action == ActionReturn {
		if task.ParentTaskID != nil {
			parent, err := m.store.lockTask(ctx, tx, tenantID, *task.ParentTaskID)
			if err != nil {
				return nil, notFound(err, "parent task")
			}
			scopeUnit = parent.UnitID
		}
	}
	if !actor.inScope(scopeUnit) {
		return nil, &WorkflowError{Code: "OUT_OF_SCOPE", Message: "the task is outside your organisational scope"}
	}

	var step *WorkflowStep
	var allowed map[transitionKey]string
	if task.WorkflowVersionID != nil {
		version, err := m.store.loadVersion(ctx, tx, tenantID, *task.WorkflowVersionID)
		if err != nil {
			return nil, notFound(err, "workflow version")
		}
		for i := range version.Steps {
			if task.StepID != nil && version.Steps[i].ID == *task.StepID {
				step = &version.Steps[i]
				break
			}
		}
		if allowed, err = m.store.allowedTransitions(ctx, tx, tenantID, *task.WorkflowVersionID); err != nil {
			return nil, err
		}
	}

	result, err := resolveTransition(task.Status, in.Action, step, allowed)
	if err != nil {
		return nil, err
	}
	if !actor.can(result.permission) {
		return nil, &WorkflowError{Code: "FORBIDDEN", Message: "permission " + result.permission + " required"}
	}
	if result.requiresComment && strings.TrimSpace(in.Comment) == "" {
		return nil, &WorkflowError{Code: "COMMENT_REQUIRED", Message: "this action requires a comment"}
	}

	patch := taskPatch{Status: result.to, Actor: actor.Email}
	if in.ResultCode != "" {
		patch.ResultCode = &in.ResultCode
	}
	if in.Comment != "" {
		patch.ResultDescription = &in.Comment
	}
	if in.Action == ActionAssign {
		user := in.AssignUserID
		if user == "" {
			user = actor.UserID
		}
		patch.AssignedUserID = &user
	}
	patch.MarkCompleted = result.to == TaskCompleted || result.to == TaskAwaitingVerification
	patch.MarkVerified = in.Action == ActionVerify

	if err := m.store.updateTaskStatus(ctx, tx, task.ID, in.RowVersion, patch); err != nil {
		return nil, err
	}

	if err := m.store.recordEvent(ctx, tx, timelineEvent{
		ApplicationID: task.ApplicationID, TenantID: tenantID, TaskID: &task.ID, UnitID: &task.UnitID,
		EventType: in.Action, Message: in.Comment, Actor: actor.Email,
		FromStatus: task.Status, ToStatus: result.to,
	}); err != nil {
		return nil, err
	}

	if in.Action == ActionDelegate {
		if err := m.delegate(ctx, tx, tenantID, actor, task, step, in); err != nil {
			return nil, err
		}
	}

	// A finished task lets the level above continue.
	if result.to == TaskCompleted {
		if err := m.propagateCompletion(ctx, tx, tenantID, actor, task); err != nil {
			return nil, err
		}
	}

	if err := m.store.syncApplication(ctx, tx, tenantID, task.ApplicationID); err != nil {
		return nil, err
	}

	// Upstream systems learn about terminal outcomes through the outbox.
	if slices.Contains([]string{TaskClosed, TaskRejected, TaskCancelled, TaskCompleted}, result.to) {
		if err := m.enqueueStatus(ctx, tx, tenantID, task.ApplicationID, result.to); err != nil {
			return nil, err
		}
	}

	updated, err := m.store.lockTask(ctx, tx, tenantID, task.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	updated.Overdue = isOverdue(updated)
	return updated, nil
}

// delegate creates the child task the next level works on.
func (m *Module) delegate(ctx context.Context, tx pgx.Tx, tenantID string, actor Actor, parent *Task, step *WorkflowStep, in ActionInput) error {
	if parent.WorkflowVersionID == nil {
		return &WorkflowError{Code: "NO_WORKFLOW", Message: "task has no workflow version to delegate within"}
	}
	version, err := m.store.loadVersion(ctx, tx, tenantID, *parent.WorkflowVersionID)
	if err != nil {
		return err
	}

	next := nextStep(version.Steps, step)
	if next == nil {
		return &WorkflowError{Code: "NO_NEXT_STEP", Message: "the workflow has no step to delegate to"}
	}

	units, err := m.store.listUnits(ctx, tx, tenantID)
	if err != nil {
		return err
	}
	graph := NewUnitGraph(units)
	rules, err := m.store.listRoutingRules(ctx, tx, tenantID)
	if err != nil {
		return err
	}

	target := in.TargetUnitID
	if target == "" && next.ExecutorUnitID != nil {
		target = *next.ExecutorUnitID
	}
	fields, err := m.requestFields(ctx, tx, tenantID, parent.ApplicationID)
	if err != nil {
		return err
	}

	childUnit, err := graph.Resolve(next.ExecutorRule, RoutingRequest{
		CurrentUnitID: parent.UnitID,
		TargetUnitID:  target,
		Fields:        fields,
	}, rules, "")
	if err != nil {
		return err
	}

	due := time.Now().Add(time.Duration(next.SLAHours) * time.Hour)
	childID, err := m.store.insertTask(ctx, tx, Task{
		TenantID:          tenantID,
		ApplicationID:     parent.ApplicationID,
		ParentTaskID:      &parent.ID,
		UnitID:            childUnit,
		WorkflowVersionID: parent.WorkflowVersionID,
		StepID:            &next.ID,
		Status:            TaskReceived,
		DueAt:             &due,
	})
	if err != nil {
		return err
	}

	return m.store.recordEvent(ctx, tx, timelineEvent{
		ApplicationID: parent.ApplicationID, TenantID: tenantID, TaskID: &childID, UnitID: &childUnit,
		EventType: "delegated", Message: in.Comment, Actor: actor.Email,
		FromStatus: "", ToStatus: TaskReceived,
	})
}

// propagateCompletion walks up the delegation chain. A parent waiting on a
// child finishes when the child is done — unless the parent's own step demands
// verification from the level above it, in which case it waits in turn.
func (m *Module) propagateCompletion(ctx context.Context, tx pgx.Tx, tenantID string, actor Actor, completed *Task) error {
	current := completed
	for current.ParentTaskID != nil {
		parent, err := m.store.lockTask(ctx, tx, tenantID, *current.ParentTaskID)
		if err != nil {
			return err
		}
		if parent.Status != TaskForwarded {
			return nil // the parent is not waiting on this child
		}

		// Any sibling still open keeps the parent waiting.
		var open int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM gov_tasks
			  WHERE tenant_id = $1 AND parent_task_id = $2 AND status = ANY($3)`,
			tenantID, parent.ID, activeTaskStatuses).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			return nil
		}

		next := TaskCompleted
		if parentStep, err := m.stepOf(ctx, tx, tenantID, parent); err == nil && parentStep != nil && parentStep.RequiresVerification && parent.ParentTaskID != nil {
			next = TaskAwaitingVerification
		}

		if err := m.store.updateTaskStatus(ctx, tx, parent.ID, parent.RowVersion, taskPatch{
			Status: next, Actor: actor.Email, MarkCompleted: true,
		}); err != nil {
			return err
		}
		if err := m.store.recordEvent(ctx, tx, timelineEvent{
			ApplicationID: parent.ApplicationID, TenantID: tenantID, TaskID: &parent.ID, UnitID: &parent.UnitID,
			EventType: "child_completed", Message: "Доод нэгжийн ажил дууссан", Actor: actor.Email,
			FromStatus: parent.Status, ToStatus: next,
		}); err != nil {
			return err
		}
		if next != TaskCompleted {
			return nil
		}

		parent.Status = next
		current = parent
	}
	return nil
}

func (m *Module) stepOf(ctx context.Context, tx pgx.Tx, tenantID string, task *Task) (*WorkflowStep, error) {
	if task.WorkflowVersionID == nil || task.StepID == nil {
		return nil, nil
	}
	version, err := m.store.loadVersion(ctx, tx, tenantID, *task.WorkflowVersionID)
	if err != nil {
		return nil, err
	}
	for i := range version.Steps {
		if version.Steps[i].ID == *task.StepID {
			return &version.Steps[i], nil
		}
	}
	return nil, nil
}

func (m *Module) requestFields(ctx context.Context, tx pgx.Tx, tenantID, applicationID string) (map[string]string, error) {
	var raw []byte
	if err := tx.QueryRow(ctx,
		`SELECT request_payload FROM gov_applications WHERE id = $1 AND tenant_id = $2`,
		applicationID, tenantID).Scan(&raw); err != nil {
		return nil, err
	}
	fields := map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &fields)
	}
	return fields, nil
}

func nextStep(steps []WorkflowStep, current *WorkflowStep) *WorkflowStep {
	if current == nil {
		if len(steps) > 0 {
			return &steps[0]
		}
		return nil
	}
	for i := range steps {
		if steps[i].ID == current.ID && i+1 < len(steps) {
			return &steps[i+1]
		}
	}
	return nil
}

func isOverdue(t *Task) bool {
	// Overdue is derived, never written into the business status.
	return t.DueAt != nil && t.DueAt.Before(time.Now()) && !slices.Contains(terminalTaskStatuses, t.Status) && t.Status != TaskCompleted
}

// enqueueStatus writes the upstream status update into the outbox, addressed to
// the connector configured for the request's source system.
func (m *Module) enqueueStatus(ctx context.Context, tx pgx.Tx, tenantID, applicationID, status string) error {
	var (
		reference    string
		sourceSystem string
		externalID   *string
		targetURL    string
		secretRef    string
	)
	err := tx.QueryRow(ctx,
		`SELECT a.reference, a.source_system, a.external_request_id,
		        COALESCE(c.target_url, ''), COALESCE(c.secret_ref, '')
		   FROM gov_applications a
		   LEFT JOIN gov_upstream_connectors c
		          ON c.tenant_id = a.tenant_id AND c.source_system = a.source_system AND c.active
		  WHERE a.id = $1 AND a.tenant_id = $2`, applicationID, tenantID).
		Scan(&reference, &sourceSystem, &externalID, &targetURL, &secretRef)
	if err != nil {
		return err
	}
	if targetURL == "" || externalID == nil {
		return nil // nothing upstream is waiting for this request
	}

	payload, err := json.Marshal(map[string]any{
		"reference":           reference,
		"external_request_id": *externalID,
		"status":              status,
		"occurred_at":         time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return m.store.enqueueDelivery(ctx, tx, tenantID, applicationID, "status_changed", payload, targetURL, secretRef)
}
