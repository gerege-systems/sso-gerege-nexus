/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Package gov_services implements the State Services Go module
 * (io.example.gov_services): a service passport registry, citizen
 * applications with a status timeline, appointments and the officer queue.
 *
 * The domain mirrors the service delivery model of
 * github.com/gerege-systems/open-gerege-mn, rebuilt on this platform's
 * compile-time Module contract.
 */

package gov_services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/appregistry"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/auth"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/httpx"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/integration"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/rbac"
	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Application lifecycle. A citizen may cancel until a decision is recorded;
// officers move an application forward through the remaining states.
const (
	StatusSubmitted     = "SUBMITTED"
	StatusInReview      = "IN_REVIEW"
	StatusInfoRequested = "INFO_REQUESTED"
	StatusApproved      = "APPROVED"
	StatusRejected      = "REJECTED"
	StatusCompleted     = "COMPLETED"
	StatusCancelled     = "CANCELLED"
)

// officerDecisions are the states an officer may move an application into.
var officerDecisions = []string{StatusInReview, StatusInfoRequested, StatusApproved, StatusRejected, StatusCompleted}

type Service struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	Code                string    `json:"code"`
	Name                string    `json:"name"`
	NameEN              string    `json:"name_en,omitempty"`
	Category            string    `json:"category"`
	Description         string    `json:"description"`
	Fee                 float64   `json:"fee"`
	DurationDays        int       `json:"duration_days"`
	RequiredEvidences   []string  `json:"required_evidences"`
	AppointmentRequired bool      `json:"appointment_required"`
	Active              bool      `json:"active"`
	CreatedAt           time.Time `json:"created_at"`

	// Workflow configuration (migration 00007).
	FulfillmentMode   string  `json:"fulfillment_mode"`
	WorkflowVersionID *string `json:"workflow_version_id,omitempty"`
	OwnerUnitID       *string `json:"owner_unit_id,omitempty"`
}

type Application struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	ServiceID       string     `json:"service_id"`
	ServiceName     string     `json:"service_name,omitempty"`
	Reference       string     `json:"reference"`
	ApplicantName   string     `json:"applicant_name"`
	ApplicantReg    string     `json:"applicant_reg"`
	ApplicantPhone  string     `json:"applicant_phone"`
	Status          string     `json:"status"`
	Note            string     `json:"note"`
	AssignedOfficer string     `json:"assigned_officer"`
	DecisionReason  string     `json:"decision_reason"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
}

type Event struct {
	ID         string    `json:"id"`
	EventType  string    `json:"event_type"`
	Message    string    `json:"message"`
	Actor      string    `json:"actor"`
	FromStatus string    `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status,omitempty"`
	UnitName   string    `json:"unit_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Appointment struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	ApplicationID *string   `json:"application_id,omitempty"`
	ServiceID     string    `json:"service_id"`
	ServiceName   string    `json:"service_name,omitempty"`
	CitizenName   string    `json:"citizen_name"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Location      string    `json:"location"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`

	// Mode is IN_PERSON or ONLINE. An online appointment carries a joining
	// link instead of an address.
	Mode string `json:"mode"`
	// MeetingURL is stored rather than derived: it is issued once by the
	// conferencing provider and has to outlive the connector being
	// disconnected. A citizen holding a link for next Tuesday should still be
	// able to attend.
	MeetingURL      string `json:"meeting_url,omitempty"`
	MeetingProvider string `json:"meeting_provider,omitempty"`
	// MeetingError explains why an online booking has no link yet. The
	// appointment is still made — a conferencing outage must not cost the
	// citizen their slot — so the failure is reported rather than thrown.
	MeetingError string `json:"meeting_error,omitempty"`
}

// MeetingBooker is the part of the integration manager this module needs.
//
// An interface rather than the concrete manager so the appointment tests do
// not need a Google account, and so the dependency reads as what gov_services
// wants — a way to get a joining link — rather than as "integrations".
type MeetingBooker interface {
	FirstMeetingConnector(ctx context.Context, tenantID string) (*integration.Connector, error)
	CreateMeeting(ctx context.Context, tenantID, integrationID, title string,
		startsAt time.Time, duration time.Duration, reference string) (*integration.Meeting, error)
}

type Module struct {
	db       *pgxpool.Pool
	store    *store
	perms    *rbac.SQLPermissionStore
	meetings MeetingBooker
}

func New(db *pgxpool.Pool, meetings MeetingBooker) *Module {
	m := &Module{
		db:       db,
		store:    &store{db: db},
		perms:    rbac.NewSQLPermissionStore(db),
		meetings: meetings,
	}
	appregistry.Register(m)
	return m
}

func (m *Module) ID() string      { return "io.example.gov_services" }
func (m *Module) Name() string    { return "State Services" }
func (m *Module) Version() string { return "1.1.0" }

// Dependencies: an application always names a citizen, and Contacts is where
// XYP-verified citizen records live.
func (m *Module) Dependencies() []internal.Dependency {
	return []internal.Dependency{
		{ID: "io.example.contacts", VersionConstraint: "^1.0.0"},
	}
}

func (m *Module) Permissions() []internal.PermissionDefinition {
	return []internal.PermissionDefinition{
		{Code: PermRead, Name: "Read State Services", Description: "View the service registry, requests and dashboards"},
		{Code: PermApply, Name: "Submit Requests", Description: "Submit, ingest and cancel service requests"},
		{Code: PermProcess, Name: "Process Requests", Description: "Assign, start, complete and reject tasks"},
		{Code: PermDelegate, Name: "Delegate Requests", Description: "Forward a request to a lower organisational unit"},
		{Code: PermVerify, Name: "Verify Fulfilment", Description: "Verify or return work completed by a lower unit"},
		{Code: PermConfigure, Name: "Configure Workflows", Description: "Manage units, workflows and service routing"},
		{Code: PermReport, Name: "View Reports", Description: "Read supervisory dashboards across the authorised scope"},
	}
}

func (m *Module) Menus() []internal.MenuDefinition {
	return []internal.MenuDefinition{
		{
			ID: "gov_services", ParentID: "operations", Label: "State Services",
			Path: "/gov-services", Icon: "landmark", Order: 5,
			Labels: map[string]string{"mn": "Төрийн үйлчилгээ", "ar": "الخدمات الحكومية", "zh": "政务服务", "fr": "Services publics", "ru": "Государственные услуги", "es": "Servicios públicos"},
		},
	}
}

func (m *Module) RegisterRoutes(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler) {
	r.Route("/api/v1/gov", func(gr chi.Router) {
		gr.Use(tenantAuthMiddleware)

		// Service passport. Kept at the original paths so existing clients
		// continue to work; configuration is layered on top.
		gr.Get("/services", m.listServicesHandler)
		gr.Post("/services", m.createServiceHandler)
		gr.Put("/services/{id}/configuration", m.configureServiceHandler)

		// Organisational hierarchy.
		gr.Get("/units", m.listUnitsHandler)
		gr.Get("/units/tree", m.unitTreeHandler)
		gr.Post("/units", m.createUnitHandler)
		gr.Post("/units/members", m.assignUnitMemberHandler)

		// Workflow configuration.
		gr.Get("/workflow-templates", m.listTemplatesHandler)
		gr.Get("/workflows", m.listWorkflowsHandler)
		gr.Post("/workflows", m.createWorkflowHandler)
		gr.Get("/workflow-versions/{id}", m.getVersionHandler)
		gr.Post("/workflow-versions/{id}/publish", m.publishVersionHandler)
		gr.Get("/routing-rules", m.listRoutingRulesHandler)
		gr.Post("/routing-rules", m.createRoutingRuleHandler)

		// Upstream ingestion.
		gr.Post("/requests/ingest", m.ingestHandler)

		// Requests, queues and actions.
		gr.Get("/requests/{id}", m.requestDetailHandler)
		gr.Get("/tasks", m.listTasksHandler)
		gr.Post("/tasks/{id}/actions", m.actHandler)
		gr.Get("/dashboard", m.dashboardHandler)
		gr.Get("/outbox", m.listOutboxHandler)

		// Legacy surface, preserved for the original clients. These now run on
		// the workflow underneath (see createApplicationHandler / decideHandler).
		gr.Get("/applications", m.listApplicationsHandler)
		gr.Post("/applications", m.createApplicationHandler)
		gr.Get("/applications/{id}/timeline", m.timelineHandler)
		gr.Post("/applications/{id}/cancel", m.cancelApplicationHandler)

		gr.Get("/appointments", m.listAppointmentsHandler)
		gr.Post("/appointments", m.createAppointmentHandler)

		gr.Get("/officer/queue", m.officerQueueHandler)
		gr.Post("/officer/queue/{id}/decide", m.decideHandler)
	})
}

// ─── Service passport ────────────────────────────────────────────────────────

func (m *Module) listServicesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT id, tenant_id, code, name, name_en, category, description, fee, duration_days,
		        required_evidences, appointment_required, active, created_at,
		        fulfillment_mode, workflow_version_id, owner_unit_id
		   FROM gov_services
		  WHERE tenant_id = $1
		  ORDER BY category, name`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch services")
		return
	}
	defer rows.Close()

	list := make([]Service, 0)
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Code, &s.Name, &s.NameEN, &s.Category, &s.Description,
			&s.Fee, &s.DurationDays, &s.RequiredEvidences, &s.AppointmentRequired, &s.Active, &s.CreatedAt,
			&s.FulfillmentMode, &s.WorkflowVersionID, &s.OwnerUnitID); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to read services")
			return
		}
		list = append(list, s)
	}
	if rows.Err() != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read services")
		return
	}

	httpx.JSON(w, http.StatusOK, list)
}

func (m *Module) createServiceHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	// The service passport is organisation-wide configuration.
	if !claims.IsAdmin {
		httpx.Error(w, http.StatusForbidden, "tenant administrator role required")
		return
	}

	var req struct {
		Code                string   `json:"code"`
		Name                string   `json:"name"`
		NameEN              string   `json:"name_en"`
		Category            string   `json:"category"`
		Description         string   `json:"description"`
		Fee                 float64  `json:"fee"`
		DurationDays        int      `json:"duration_days"`
		RequiredEvidences   []string `json:"required_evidences"`
		AppointmentRequired bool     `json:"appointment_required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" || req.Name == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid service: code and name are required")
		return
	}
	if req.Fee < 0 {
		httpx.Error(w, http.StatusBadRequest, "fee cannot be negative")
		return
	}
	if req.Category == "" {
		req.Category = "GENERAL"
	}
	if req.DurationDays <= 0 {
		req.DurationDays = 1
	}
	if req.RequiredEvidences == nil {
		req.RequiredEvidences = []string{}
	}

	var s Service
	err = m.db.QueryRow(r.Context(),
		`INSERT INTO gov_services (tenant_id, code, name, name_en, category, description, fee,
		                           duration_days, required_evidences, appointment_required)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, tenant_id, code, name, name_en, category, description, fee, duration_days,
		           required_evidences, appointment_required, active, created_at,
		           fulfillment_mode, workflow_version_id, owner_unit_id`,
		claims.TenantID, req.Code, req.Name, req.NameEN, req.Category, req.Description, req.Fee,
		req.DurationDays, req.RequiredEvidences, req.AppointmentRequired).
		Scan(&s.ID, &s.TenantID, &s.Code, &s.Name, &s.NameEN, &s.Category, &s.Description, &s.Fee,
			&s.DurationDays, &s.RequiredEvidences, &s.AppointmentRequired, &s.Active, &s.CreatedAt,
			&s.FulfillmentMode, &s.WorkflowVersionID, &s.OwnerUnitID)
	if err != nil {
		httpx.Error(w, http.StatusConflict, "service code already exists for this tenant")
		return
	}

	httpx.JSON(w, http.StatusCreated, s)
}

// ─── Applications ────────────────────────────────────────────────────────────

func (m *Module) listApplicationsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	query := `SELECT a.id, a.tenant_id, a.service_id, s.name, a.reference, a.applicant_name,
	                 a.applicant_reg, a.applicant_phone, a.status, a.note, a.assigned_officer,
	                 a.decision_reason, a.created_at, a.updated_at, a.decided_at
	            FROM gov_applications a
	            JOIN gov_services s ON s.id = a.service_id
	           WHERE a.tenant_id = $1`
	args := []any{tenantID}
	if status := r.URL.Query().Get("status"); status != "" {
		query += ` AND a.status = $2`
		args = append(args, status)
	}
	query += ` ORDER BY a.created_at DESC`

	list, err := m.queryApplications(r.Context(), query, args...)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch applications")
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

// createApplicationHandler is the original portal submission endpoint. It now
// runs on the workflow underneath: the request is created and its first task is
// placed on the configured step, so old clients keep working while their
// requests become visible to the queues, dashboards and delegation.
func (m *Module) createApplicationHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermApply)
	if !ok {
		return
	}

	var req struct {
		ServiceID      string            `json:"service_id"`
		ApplicantName  string            `json:"applicant_name"`
		ApplicantReg   string            `json:"applicant_reg"`
		ApplicantPhone string            `json:"applicant_phone"`
		Note           string            `json:"note"`
		Fields         map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ServiceID == "" || req.ApplicantName == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "service_id and applicant_name are required"})
		return
	}

	ctx := r.Context()
	tx, err := m.db.Begin(ctx)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	svc, err := m.store.loadServiceConfig(ctx, tx, tenantID, req.ServiceID)
	if err != nil {
		writeDomainError(w, notFound(err, "service"))
		return
	}
	if !svc.Active {
		writeDomainError(w, &WorkflowError{Code: "SERVICE_INACTIVE", Message: "service is no longer offered"})
		return
	}

	fields, _ := json.Marshal(req.Fields)
	reference := fmt.Sprintf("GS-%d", time.Now().UnixNano()/1e6)

	var app Application
	err = tx.QueryRow(ctx,
		`INSERT INTO gov_applications (tenant_id, service_id, reference, applicant_name,
		                               applicant_reg, applicant_phone, note, workflow_version_id,
		                               fulfillment_mode, source_system, request_payload,
		                               origin_unit_id, current_unit_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'PORTAL', $10, $11, $11)
		 RETURNING id, tenant_id, service_id, reference, applicant_name, applicant_reg,
		           applicant_phone, status, note, assigned_officer, decision_reason,
		           created_at, updated_at, decided_at`,
		tenantID, req.ServiceID, reference, req.ApplicantName, req.ApplicantReg,
		req.ApplicantPhone, req.Note, svc.WorkflowVersionID, svc.FulfillmentMode, fields, svc.OwnerUnitID).
		Scan(&app.ID, &app.TenantID, &app.ServiceID, &app.Reference, &app.ApplicantName,
			&app.ApplicantReg, &app.ApplicantPhone, &app.Status, &app.Note, &app.AssignedOfficer,
			&app.DecisionReason, &app.CreatedAt, &app.UpdatedAt, &app.DecidedAt)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// A service configured with a workflow gets a task; one that predates
	// configuration still records the request and its timeline.
	if svc.WorkflowVersionID != nil && svc.OwnerUnitID != nil {
		if err := m.createInitialTask(ctx, tx, tenantID, app.ID, *svc.OwnerUnitID, *svc, req.Fields, actor.Email); err != nil {
			writeDomainError(w, err)
			return
		}
	} else if err := m.store.recordEvent(ctx, tx, timelineEvent{
		ApplicationID: app.ID, TenantID: tenantID, EventType: "submitted",
		Message: "Хүсэлт хүлээн авлаа", Actor: actor.Email, ToStatus: "SUBMITTED",
	}); err != nil {
		writeDomainError(w, err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeDomainError(w, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, app)
}

// cancelApplicationHandler cancels a request. The cancellation runs through the
// state machine on the open task so the timeline, the derived request status
// and the outbox all stay consistent.
func (m *Module) cancelApplicationHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, err := m.actorFrom(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	applicationID := chi.URLParam(r, "id")

	var taskID string
	var rowVersion int
	err = m.db.QueryRow(r.Context(),
		`SELECT id, row_version FROM gov_tasks
		  WHERE application_id = $1 AND tenant_id = $2 AND status = ANY($3)
		  ORDER BY created_at DESC LIMIT 1`,
		applicationID, tenantID, activeTaskStatuses).Scan(&taskID, &rowVersion)
	if err != nil {
		writeDomainError(w, notFound(err, "open task for this request"))
		return
	}

	task, err := m.Act(r.Context(), tenantID, actor, taskID, ActionInput{
		Action: ActionCancel, RowVersion: rowVersion,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"status": applicationStatusFor(task.Status), "id": applicationID,
	})
}
func (m *Module) timelineHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")

	// Join through the application so one tenant cannot read another's timeline.
	rows, err := m.db.Query(r.Context(),
		`SELECT e.id, e.event_type, e.message, e.actor, e.created_at
		   FROM gov_application_events e
		   JOIN gov_applications a ON a.id = e.application_id
		  WHERE e.application_id = $1 AND a.tenant_id = $2
		  ORDER BY e.created_at`, id, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch the timeline")
		return
	}
	defer rows.Close()

	list := make([]Event, 0)
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.EventType, &e.Message, &e.Actor, &e.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to read the timeline")
			return
		}
		list = append(list, e)
	}
	if rows.Err() != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read the timeline")
		return
	}

	httpx.JSON(w, http.StatusOK, list)
}

// ─── Officer queue ───────────────────────────────────────────────────────────

func (m *Module) officerQueueHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	list, err := m.queryApplications(r.Context(),
		`SELECT a.id, a.tenant_id, a.service_id, s.name, a.reference, a.applicant_name,
		        a.applicant_reg, a.applicant_phone, a.status, a.note, a.assigned_officer,
		        a.decision_reason, a.created_at, a.updated_at, a.decided_at
		   FROM gov_applications a
		   JOIN gov_services s ON s.id = a.service_id
		  WHERE a.tenant_id = $1 AND a.status = ANY($2)
		  ORDER BY a.created_at`,
		tenantID, []string{StatusSubmitted, StatusInReview, StatusInfoRequested})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch the officer queue")
		return
	}
	httpx.JSON(w, http.StatusOK, list)
}

// decideHandler is the original officer decision endpoint. Legacy decisions are
// translated into workflow actions on the request's open task, so both APIs
// drive the same state machine and can never disagree.
func (m *Module) decideHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, err := m.actorFrom(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	applicationID := chi.URLParam(r, "id")

	var req struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "invalid decision payload"})
		return
	}
	if !slices.Contains(officerDecisions, req.Status) {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "unsupported decision status " + req.Status})
		return
	}

	// Find the task the decision applies to: the open one, deepest first.
	var taskID string
	var rowVersion int
	err = m.db.QueryRow(r.Context(),
		`SELECT id, row_version FROM gov_tasks
		  WHERE application_id = $1 AND tenant_id = $2 AND status = ANY($3)
		  ORDER BY created_at DESC LIMIT 1`,
		applicationID, tenantID, activeTaskStatuses).Scan(&taskID, &rowVersion)
	if err != nil {
		writeDomainError(w, notFound(err, "open task for this request"))
		return
	}

	action := map[string]string{
		StatusInReview:      ActionStart,
		StatusInfoRequested: ActionRequestInfo,
		StatusApproved:      ActionComplete,
		StatusRejected:      ActionReject,
		StatusCompleted:     ActionClose,
	}[req.Status]
	if action == "" {
		writeDomainError(w, &WorkflowError{Code: "INVALID_INPUT", Message: "unsupported decision status " + req.Status})
		return
	}

	task, err := m.Act(r.Context(), tenantID, actor, taskID, ActionInput{
		Action:     action,
		Comment:    req.Reason,
		RowVersion: rowVersion,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// COMPLETED in the legacy vocabulary means "closed"; close it in one call
	// when the task is already finished.
	if req.Status == StatusCompleted && task.Status == TaskCompleted {
		if task, err = m.Act(r.Context(), tenantID, actor, taskID, ActionInput{
			Action: ActionClose, RowVersion: task.RowVersion,
		}); err != nil {
			writeDomainError(w, err)
			return
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]string{
		"status": applicationStatusFor(task.Status), "id": applicationID, "task_status": task.Status,
	})
}
func (m *Module) listAppointmentsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenant.Require(w, r)
	if !ok {
		return
	}

	rows, err := m.db.Query(r.Context(),
		`SELECT ap.id, ap.tenant_id, ap.application_id, ap.service_id, s.name, ap.citizen_name,
		        ap.scheduled_at, ap.location, ap.status, ap.created_at,
		        ap.mode, ap.meeting_url, ap.meeting_provider
		   FROM gov_appointments ap
		   JOIN gov_services s ON s.id = ap.service_id
		  WHERE ap.tenant_id = $1
		  ORDER BY ap.scheduled_at`, tenantID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to fetch appointments")
		return
	}
	defer rows.Close()

	list := make([]Appointment, 0)
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(&a.ID, &a.TenantID, &a.ApplicationID, &a.ServiceID, &a.ServiceName,
			&a.CitizenName, &a.ScheduledAt, &a.Location, &a.Status, &a.CreatedAt,
			&a.Mode, &a.MeetingURL, &a.MeetingProvider); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "failed to read appointments")
			return
		}
		list = append(list, a)
	}
	if rows.Err() != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to read appointments")
		return
	}

	httpx.JSON(w, http.StatusOK, list)
}

func (m *Module) createAppointmentHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.UserFromContext(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ServiceID     string    `json:"service_id"`
		ApplicationID *string   `json:"application_id"`
		CitizenName   string    `json:"citizen_name"`
		ScheduledAt   time.Time `json:"scheduled_at"`
		Location      string    `json:"location"`
		Mode          string    `json:"mode"`
		// DurationMinutes sizes the conference slot. Ignored for an in-person
		// visit, which has no end time to publish.
		DurationMinutes int `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ServiceID == "" || req.CitizenName == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid appointment: service_id and citizen_name are required")
		return
	}
	if req.ScheduledAt.IsZero() || req.ScheduledAt.Before(time.Now()) {
		httpx.Error(w, http.StatusBadRequest, "scheduled_at must be in the future")
		return
	}
	mode := strings.ToUpper(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = AppointmentInPerson
	}
	if mode != AppointmentInPerson && mode != AppointmentOnline {
		httpx.Error(w, http.StatusBadRequest, "mode must be IN_PERSON or ONLINE")
		return
	}

	var a Appointment
	err = m.db.QueryRow(r.Context(),
		`INSERT INTO gov_appointments (tenant_id, application_id, service_id, citizen_name, scheduled_at, location, mode)
		 SELECT $1, $2, $3, $4, $5, $6, $7
		  WHERE EXISTS (SELECT 1 FROM gov_services WHERE id = $3 AND tenant_id = $1 AND active)
		 RETURNING id, tenant_id, application_id, service_id, citizen_name, scheduled_at, location, status,
		           created_at, mode, meeting_url, meeting_provider`,
		claims.TenantID, req.ApplicationID, req.ServiceID, req.CitizenName, req.ScheduledAt, req.Location, mode).
		Scan(&a.ID, &a.TenantID, &a.ApplicationID, &a.ServiceID, &a.CitizenName, &a.ScheduledAt,
			&a.Location, &a.Status, &a.CreatedAt, &a.Mode, &a.MeetingURL, &a.MeetingProvider)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "service not found or no longer offered")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "failed to book the appointment")
		return
	}

	if mode == AppointmentOnline {
		m.attachMeeting(r.Context(), claims.TenantID, &a, req.DurationMinutes)
	}

	httpx.JSON(w, http.StatusCreated, a)
}

// Appointment modes. An online appointment is held over a conferencing link
// rather than at an address.
const (
	AppointmentInPerson = "IN_PERSON"
	AppointmentOnline   = "ONLINE"
)

// defaultMeetingDuration is how long a service appointment is assumed to take
// when the caller does not say. It only sizes the calendar slot; nothing ends
// the meeting.
const defaultMeetingDuration = 30 * time.Minute

// attachMeeting books a conferencing link and records it against the
// appointment.
//
// The booking is deliberately not allowed to fail the appointment. A citizen
// who has taken a slot has taken it; if the conferencing provider is
// unreachable, or nobody has connected one, the right outcome is a booked
// appointment that says why it has no link yet — not a lost slot and a 500.
func (m *Module) attachMeeting(ctx context.Context, tenantID string, a *Appointment, durationMinutes int) {
	if m.meetings == nil {
		a.MeetingError = "no conferencing provider is connected"
		return
	}
	conn, err := m.meetings.FirstMeetingConnector(ctx, tenantID)
	if err != nil {
		a.MeetingError = "no conferencing provider is connected"
		return
	}

	duration := time.Duration(durationMinutes) * time.Minute
	if duration <= 0 {
		duration = defaultMeetingDuration
	}

	title := a.CitizenName
	if a.ServiceName != "" {
		title = a.ServiceName + " — " + a.CitizenName
	}

	meeting, err := m.meetings.CreateMeeting(ctx, tenantID, conn.ID, title, a.ScheduledAt, duration, a.ID)
	if err != nil {
		slog.Warn("gov_services: could not create a meeting link",
			"tenant", tenantID, "appointment", a.ID, "error", err)
		a.MeetingError = err.Error()
		return
	}

	if _, err := m.db.Exec(ctx,
		`UPDATE gov_appointments SET meeting_url = $3, meeting_provider = $4
		  WHERE id = $1 AND tenant_id = $2`,
		a.ID, tenantID, meeting.JoinURL, meeting.Provider); err != nil {
		// The link exists but could not be recorded. Returning it anyway means
		// the operator on this screen can still pass it on, and the delivery
		// log holds the copy.
		slog.Error("gov_services: meeting link created but not stored",
			"appointment", a.ID, "error", err)
		a.MeetingError = "the meeting link could not be saved — copy it before leaving this screen"
	}
	a.MeetingURL = meeting.JoinURL
	a.MeetingProvider = meeting.Provider
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (m *Module) queryApplications(ctx context.Context, query string, args ...any) ([]Application, error) {
	rows, err := m.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Application, 0)
	for rows.Next() {
		var a Application
		if err := rows.Scan(&a.ID, &a.TenantID, &a.ServiceID, &a.ServiceName, &a.Reference,
			&a.ApplicantName, &a.ApplicantReg, &a.ApplicantPhone, &a.Status, &a.Note,
			&a.AssignedOfficer, &a.DecisionReason, &a.CreatedAt, &a.UpdatedAt, &a.DecidedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}
