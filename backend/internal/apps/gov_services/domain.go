/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Domain types and vocabularies for the configurable government service
 * workflow (io.example.gov_services).
 */

package gov_services

import (
	"encoding/json"
	"time"
)

// Task statuses. The database enforces the same vocabulary with a CHECK
// constraint; keep the two in step.
const (
	TaskReceived             = "RECEIVED"
	TaskAssigned             = "ASSIGNED"
	TaskInProgress           = "IN_PROGRESS"
	TaskForwarded            = "FORWARDED"
	TaskInfoRequested        = "INFO_REQUESTED"
	TaskCompleted            = "COMPLETED"
	TaskAwaitingVerification = "AWAITING_VERIFICATION"
	TaskReturned             = "RETURNED"
	TaskRejected             = "REJECTED"
	TaskClosed               = "CLOSED"
	TaskCancelled            = "CANCELLED"
)

// Actions a client may request. The status a task lands in is decided by the
// engine, never supplied by the caller.
const (
	ActionAssign      = "assign"
	ActionStart       = "start"
	ActionDelegate    = "delegate"
	ActionRequestInfo = "request_info"
	ActionComplete    = "complete"
	ActionVerify      = "verify"
	ActionReturn      = "return"
	ActionReject      = "reject"
	ActionCancel      = "cancel"
	ActionClose       = "close"
)

// Fulfilment modes configured per service.
const (
	ModeLocal    = "LOCAL"
	ModeDelegate = "DELEGATE"
	ModeHybrid   = "HYBRID"
)

// Routing strategies. REGION_MATCH compares a request field against the
// region_code of candidate units; there is no expression evaluation anywhere.
const (
	RouteSelf         = "SELF"
	RouteParent       = "PARENT"
	RouteChild        = "CHILD"
	RouteSpecificUnit = "SPECIFIC_UNIT"
	RouteRegionMatch  = "REGION_MATCH"
)

// Workflow version lifecycle.
const (
	VersionDraft     = "DRAFT"
	VersionPublished = "PUBLISHED"
	VersionArchived  = "ARCHIVED"
)

// Permissions. Must match catalog/manifests/gov-services.json.
const (
	PermRead      = "gov.read"
	PermApply     = "gov.apply"
	PermProcess   = "gov.process"
	PermDelegate  = "gov.delegate"
	PermVerify    = "gov.verify"
	PermConfigure = "gov.configure"
	PermReport    = "gov.report"
)

// terminalTaskStatuses can no longer transition.
var terminalTaskStatuses = []string{TaskClosed, TaskCancelled, TaskRejected}

// activeTaskStatuses are the states that still consume officer attention and
// therefore count towards SLA and queue metrics.
var activeTaskStatuses = []string{
	TaskReceived, TaskAssigned, TaskInProgress, TaskForwarded,
	TaskInfoRequested, TaskAwaitingVerification, TaskReturned,
}

type OrgUnit struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	NameEN     string    `json:"name_en,omitempty"`
	UnitType   string    `json:"unit_type"`
	ParentID   *string   `json:"parent_id,omitempty"`
	RegionCode string    `json:"region_code,omitempty"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	// Children is populated only by the tree endpoint.
	Children []*OrgUnit `json:"children,omitempty"`
}

type Workflow struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	NameEN      string            `json:"name_en,omitempty"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Versions    []WorkflowVersion `json:"versions,omitempty"`
}

type WorkflowVersion struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	WorkflowID  string         `json:"workflow_id"`
	Version     int            `json:"version"`
	Status      string         `json:"status"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	PublishedBy string         `json:"published_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	Steps       []WorkflowStep `json:"steps,omitempty"`
}

type WorkflowStep struct {
	ID                   string          `json:"id"`
	VersionID            string          `json:"version_id"`
	Code                 string          `json:"code"`
	Name                 string          `json:"name"`
	NameEN               string          `json:"name_en,omitempty"`
	StepOrder            int             `json:"step_order"`
	StepType             string          `json:"step_type"`
	ExecutorRule         string          `json:"executor_rule"`
	ExecutorUnitID       *string         `json:"executor_unit_id,omitempty"`
	ExecutorConfig       json.RawMessage `json:"executor_config,omitempty"`
	SLAHours             int             `json:"sla_hours"`
	RequiresVerification bool            `json:"requires_verification"`
	IsTerminal           bool            `json:"is_terminal"`
}

type Transition struct {
	ID                 string  `json:"id"`
	VersionID          string  `json:"version_id"`
	FromStepID         *string `json:"from_step_id,omitempty"`
	FromStatus         string  `json:"from_status"`
	Action             string  `json:"action"`
	ToStepID           *string `json:"to_step_id,omitempty"`
	ToStatus           string  `json:"to_status"`
	RequiredPermission string  `json:"required_permission,omitempty"`
}

type RoutingRule struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ServiceID    *string   `json:"service_id,omitempty"`
	Priority     int       `json:"priority"`
	MatchField   string    `json:"match_field,omitempty"`
	MatchValue   string    `json:"match_value,omitempty"`
	Strategy     string    `json:"strategy"`
	TargetUnitID *string   `json:"target_unit_id,omitempty"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

type Task struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	ApplicationID     string     `json:"application_id"`
	ParentTaskID      *string    `json:"parent_task_id,omitempty"`
	UnitID            string     `json:"unit_id"`
	UnitName          string     `json:"unit_name,omitempty"`
	AssignedUserID    *string    `json:"assigned_user_id,omitempty"`
	WorkflowVersionID *string    `json:"workflow_version_id,omitempty"`
	StepID            *string    `json:"step_id,omitempty"`
	StepCode          string     `json:"step_code,omitempty"`
	Status            string     `json:"status"`
	DueAt             *time.Time `json:"due_at,omitempty"`
	Overdue           bool       `json:"overdue"`
	ResultCode        string     `json:"result_code,omitempty"`
	ResultDescription string     `json:"result_description,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CompletedBy       string     `json:"completed_by,omitempty"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
	VerifiedBy        string     `json:"verified_by,omitempty"`
	RowVersion        int        `json:"row_version"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	// Reference and ServiceName are joined in for queue rendering.
	Reference   string `json:"reference,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// Dashboard is the per-scope summary every level uses to monitor its work.
type Dashboard struct {
	Scope                 string `json:"scope"`
	UnitCount             int    `json:"unit_count"`
	Received              int    `json:"received"`
	InProgress            int    `json:"in_progress"`
	Delegated             int    `json:"delegated"`
	AwaitingVerification  int    `json:"awaiting_verification"`
	InfoRequested         int    `json:"info_requested"`
	Returned              int    `json:"returned"`
	Completed             int    `json:"completed"`
	Closed                int    `json:"closed"`
	Rejected              int    `json:"rejected"`
	Cancelled             int    `json:"cancelled"`
	Overdue               int    `json:"overdue"`
	TotalActive           int    `json:"total_active"`
	TotalRequests         int    `json:"total_requests"`
	UnfulfilledRequests   int    `json:"unfulfilled_requests"`
	FulfilledRequests     int    `json:"fulfilled_requests"`
	VerificationPendingUp int    `json:"verification_pending_upper"`
}

// Page is the envelope returned by every list endpoint.
type Page[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// applicationStatusFor maps the root task status onto the compatibility status
// carried by gov_applications, which pre-dates the workflow model and is still
// consumed by the original /applications clients. gov_tasks is the source of
// truth; this column is derived from it inside the same transaction.
func applicationStatusFor(taskStatus string) string {
	switch taskStatus {
	case TaskReceived, TaskAssigned:
		return "SUBMITTED"
	case TaskInProgress, TaskForwarded, TaskAwaitingVerification, TaskReturned:
		return "IN_REVIEW"
	case TaskInfoRequested:
		return "INFO_REQUESTED"
	case TaskCompleted:
		return "APPROVED"
	case TaskClosed:
		return "COMPLETED"
	case TaskRejected:
		return "REJECTED"
	case TaskCancelled:
		return "CANCELLED"
	default:
		return "SUBMITTED"
	}
}
