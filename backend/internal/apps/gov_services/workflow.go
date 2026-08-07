/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * The workflow state machine and the reusable workflow templates.
 */

package gov_services

import (
	"fmt"
	"slices"
)

// Rule of the engine: code decides what is *possible*, configuration decides
// what is *offered*.
//
// baseTransitions below encodes the invariants of the domain — you cannot
// verify work that is not awaiting verification, you cannot complete a
// cancelled task. A published workflow version may narrow this set through
// gov_workflow_transitions, never widen it. Keeping the invariants in code
// makes them testable without a database and means a misconfigured tenant can
// never reach an impossible state.
type transitionKey struct {
	from   string
	action string
}

// outcome is the status a transition lands in. When Verified is set the engine
// picks the status depending on whether the step requires verification.
type outcome struct {
	to string
	// verificationTo is used instead of `to` when the step requires
	// upper-level verification before the work counts as done.
	verificationTo string
	// permission the caller must hold, on top of the app gate.
	permission string
	// requiresComment rejects the action when no reason/comment is supplied.
	requiresComment bool
}

var baseTransitions = map[transitionKey]outcome{
	{TaskReceived, ActionAssign}:     {to: TaskAssigned, permission: PermProcess},
	{TaskReturned, ActionAssign}:     {to: TaskAssigned, permission: PermProcess},
	{TaskReceived, ActionStart}:      {to: TaskInProgress, permission: PermProcess},
	{TaskAssigned, ActionStart}:      {to: TaskInProgress, permission: PermProcess},
	{TaskReturned, ActionStart}:      {to: TaskInProgress, permission: PermProcess},
	{TaskInfoRequested, ActionStart}: {to: TaskInProgress, permission: PermProcess},

	{TaskReceived, ActionDelegate}:   {to: TaskForwarded, permission: PermDelegate},
	{TaskAssigned, ActionDelegate}:   {to: TaskForwarded, permission: PermDelegate},
	{TaskInProgress, ActionDelegate}: {to: TaskForwarded, permission: PermDelegate},

	{TaskAssigned, ActionRequestInfo}:   {to: TaskInfoRequested, permission: PermProcess, requiresComment: true},
	{TaskInProgress, ActionRequestInfo}: {to: TaskInfoRequested, permission: PermProcess, requiresComment: true},

	// Completion lands in AWAITING_VERIFICATION when the step demands an
	// upper-level check, so a lower unit finishing work never closes the
	// overall request on its own.
	{TaskAssigned, ActionComplete}:   {to: TaskCompleted, verificationTo: TaskAwaitingVerification, permission: PermProcess},
	{TaskInProgress, ActionComplete}: {to: TaskCompleted, verificationTo: TaskAwaitingVerification, permission: PermProcess},
	{TaskReturned, ActionComplete}:   {to: TaskCompleted, verificationTo: TaskAwaitingVerification, permission: PermProcess},

	{TaskAwaitingVerification, ActionVerify}: {to: TaskCompleted, permission: PermVerify},
	{TaskAwaitingVerification, ActionReturn}: {to: TaskReturned, permission: PermVerify, requiresComment: true},

	{TaskCompleted, ActionClose}: {to: TaskClosed, permission: PermProcess},

	{TaskReceived, ActionReject}:      {to: TaskRejected, permission: PermProcess, requiresComment: true},
	{TaskAssigned, ActionReject}:      {to: TaskRejected, permission: PermProcess, requiresComment: true},
	{TaskInProgress, ActionReject}:    {to: TaskRejected, permission: PermProcess, requiresComment: true},
	{TaskInfoRequested, ActionReject}: {to: TaskRejected, permission: PermProcess, requiresComment: true},
	{TaskReturned, ActionReject}:      {to: TaskRejected, permission: PermProcess, requiresComment: true},

	{TaskReceived, ActionCancel}:      {to: TaskCancelled, permission: PermApply},
	{TaskAssigned, ActionCancel}:      {to: TaskCancelled, permission: PermApply},
	{TaskInProgress, ActionCancel}:    {to: TaskCancelled, permission: PermApply},
	{TaskInfoRequested, ActionCancel}: {to: TaskCancelled, permission: PermApply},
}

// resolveTransition validates an action against the state machine and the
// step configuration. allowed is the set of (status, action) pairs the
// published workflow version permits; a nil map means "no narrowing".
func resolveTransition(from, action string, step *WorkflowStep, allowed map[transitionKey]string) (outcome, error) {
	if slices.Contains(terminalTaskStatuses, from) {
		return outcome{}, &WorkflowError{
			Code:    "TASK_TERMINAL",
			Message: fmt.Sprintf("task is %s and accepts no further transitions", from),
		}
	}

	result, ok := baseTransitions[transitionKey{from, action}]
	if !ok {
		return outcome{}, &WorkflowError{
			Code:    "INVALID_TRANSITION",
			Message: fmt.Sprintf("action %q is not valid from status %s", action, from),
		}
	}

	if allowed != nil {
		configured, ok := allowed[transitionKey{from, action}]
		if !ok {
			return outcome{}, &WorkflowError{
				Code:    "TRANSITION_NOT_CONFIGURED",
				Message: fmt.Sprintf("the published workflow does not offer %q from %s", action, from),
			}
		}
		// A version may pin the resulting status, but only to a status the
		// base machine already allows for this action.
		if configured != "" && configured != result.to && configured != result.verificationTo {
			return outcome{}, &WorkflowError{
				Code:    "TRANSITION_NOT_CONFIGURED",
				Message: fmt.Sprintf("configured target %s is not reachable by %q from %s", configured, action, from),
			}
		}
	}

	// Verification is a property of the step being completed.
	if action == ActionComplete && result.verificationTo != "" {
		if step != nil && step.RequiresVerification {
			result.to = result.verificationTo
		}
	}
	return result, nil
}

// WorkflowError is a machine-readable domain error. Handlers turn Code into a
// stable error code in the response body alongside the human message.
type WorkflowError struct {
	Code    string
	Message string
	Status  int
}

func (e *WorkflowError) Error() string { return e.Message }

// ─── Templates ───────────────────────────────────────────────────────────────

// TemplateStep describes one step of a reusable workflow template.
type TemplateStep struct {
	Code                 string
	Name                 string
	NameEN               string
	StepType             string
	ExecutorRule         string
	SLAHours             int
	RequiresVerification bool
	IsTerminal           bool
}

// Template is a ready-made workflow an administrator can instantiate instead of
// designing steps by hand.
type Template struct {
	Code        string
	Name        string
	NameEN      string
	Description string
	Mode        string
	Steps       []TemplateStep
}

// Templates covers the three shapes the product requires: fulfil locally,
// delegate one level with verification, and delegate through two levels with
// verification at each level above.
var Templates = []Template{
	{
		Code:        "LOCAL_FULFILMENT",
		Name:        "Байгууллага дээрээ шийдвэрлэх",
		NameEN:      "Local fulfilment",
		Description: "Хүсэлтийг хүлээн авсан нэгж өөрөө шийдвэрлэнэ.",
		Mode:        ModeLocal,
		Steps: []TemplateStep{
			{Code: "FULFILL", Name: "Шийдвэрлэх", NameEN: "Fulfil", StepType: "FULFILL", ExecutorRule: RouteSelf, SLAHours: 72, IsTerminal: true},
		},
	},
	{
		Code:        "DELEGATE_ONE_LEVEL",
		Name:        "Нэг шатны шилжүүлэлт, дээд шатны баталгаажуулалт",
		NameEN:      "One-level delegation with verification",
		Description: "Хүсэлтийг доод нэгж рүү шилжүүлж, дээд нэгж гүйцэтгэлийг баталгаажуулна.",
		Mode:        ModeDelegate,
		Steps: []TemplateStep{
			{Code: "INTAKE", Name: "Хүлээн авах", NameEN: "Intake", StepType: "FULFILL", ExecutorRule: RouteSelf, SLAHours: 24, RequiresVerification: true},
			{Code: "FULFILL", Name: "Доод нэгж шийдвэрлэх", NameEN: "Fulfil at lower unit", StepType: "FULFILL", ExecutorRule: RouteChild, SLAHours: 72, RequiresVerification: true},
			{Code: "VERIFY", Name: "Дээд шатны баталгаажуулалт", NameEN: "Upper-level verification", StepType: "VERIFY", ExecutorRule: RouteParent, SLAHours: 24, IsTerminal: true},
		},
	},
	{
		Code:        "DELEGATE_MULTI_LEVEL",
		Name:        "Олон шатны шилжүүлэлт, баталгаажуулалт",
		NameEN:      "Multi-level delegation with verification",
		Description: "Хоёр ба түүнээс дээш шатаар шилжүүлж, шат бүрт баталгаажуулна.",
		Mode:        ModeDelegate,
		Steps: []TemplateStep{
			{Code: "INTAKE", Name: "Хүлээн авах", NameEN: "Intake", StepType: "FULFILL", ExecutorRule: RouteSelf, SLAHours: 24, RequiresVerification: true},
			{Code: "DISPATCH", Name: "Дунд шатанд шилжүүлэх", NameEN: "Dispatch to middle unit", StepType: "FULFILL", ExecutorRule: RouteChild, SLAHours: 48, RequiresVerification: true},
			{Code: "FULFILL", Name: "Гүйцэтгэх нэгж", NameEN: "Executing unit", StepType: "FULFILL", ExecutorRule: RouteChild, SLAHours: 72, RequiresVerification: true},
			{Code: "VERIFY", Name: "Эцсийн баталгаажуулалт", NameEN: "Final verification", StepType: "VERIFY", ExecutorRule: RouteParent, SLAHours: 24, IsTerminal: true},
		},
	},
}

// TemplateByCode looks a template up for instantiation.
func TemplateByCode(code string) (Template, bool) {
	for _, tpl := range Templates {
		if tpl.Code == code {
			return tpl, true
		}
	}
	return Template{}, false
}
