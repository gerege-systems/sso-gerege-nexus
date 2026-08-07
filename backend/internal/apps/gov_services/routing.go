/*
 * Gerege SSO
 * Copyright (c) 2026 Gerege Systems Development Team, Gemini AI & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Routing: choosing which organisational unit executes a step.
 */

package gov_services

import (
	"fmt"
	"strings"
)

// UnitGraph is the slice of the organisational hierarchy routing needs. It is
// loaded once per decision so resolution never issues per-candidate queries.
type UnitGraph struct {
	byID       map[string]*OrgUnit
	childrenOf map[string][]*OrgUnit
}

func NewUnitGraph(units []*OrgUnit) *UnitGraph {
	g := &UnitGraph{
		byID:       make(map[string]*OrgUnit, len(units)),
		childrenOf: make(map[string][]*OrgUnit),
	}
	for _, u := range units {
		g.byID[u.ID] = u
		if u.ParentID != nil {
			g.childrenOf[*u.ParentID] = append(g.childrenOf[*u.ParentID], u)
		}
	}
	return g
}

func (g *UnitGraph) Unit(id string) (*OrgUnit, bool) {
	u, ok := g.byID[id]
	return u, ok
}

func (g *UnitGraph) Children(id string) []*OrgUnit { return g.childrenOf[id] }

// Descendants returns id and every unit below it. Used for supervisory scope
// and for dashboards that aggregate a whole branch.
func (g *UnitGraph) Descendants(id string) []string {
	out := []string{id}
	queue := []string{id}
	seen := map[string]bool{id: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, child := range g.childrenOf[current] {
			if seen[child.ID] {
				continue // defensive: a cycle would otherwise spin forever
			}
			seen[child.ID] = true
			out = append(out, child.ID)
			queue = append(queue, child.ID)
		}
	}
	return out
}

// RoutingRequest carries everything a strategy may look at. Fields is the
// validated request payload; only exact string matching is performed on it.
type RoutingRequest struct {
	CurrentUnitID string
	TargetUnitID  string
	Fields        map[string]string
}

// Resolve picks the executing unit for a strategy.
//
// Deliberately explicit: every strategy is a named branch with a testable
// outcome. There is no expression language, so a tenant cannot configure the
// platform into evaluating arbitrary input.
func (g *UnitGraph) Resolve(strategy string, req RoutingRequest, rules []RoutingRule, serviceID string) (string, error) {
	switch strategy {
	case RouteSelf, "":
		return req.CurrentUnitID, nil

	case RouteParent:
		unit, ok := g.byID[req.CurrentUnitID]
		if !ok {
			return "", &WorkflowError{Code: "UNIT_NOT_FOUND", Message: "current unit is not part of the tenant hierarchy"}
		}
		if unit.ParentID == nil {
			return "", &WorkflowError{Code: "NO_PARENT_UNIT", Message: "unit " + unit.Code + " has no parent to escalate to"}
		}
		return *unit.ParentID, nil

	case RouteChild:
		children := g.activeChildren(req.CurrentUnitID)
		if len(children) == 0 {
			return "", &WorkflowError{Code: "NO_CHILD_UNIT", Message: "unit has no active child unit to delegate to"}
		}
		// An explicit target wins as long as it really is a child.
		if req.TargetUnitID != "" {
			for _, child := range children {
				if child.ID == req.TargetUnitID {
					return child.ID, nil
				}
			}
			return "", &WorkflowError{Code: "TARGET_NOT_CHILD", Message: "the requested unit is not a child of the current unit"}
		}
		if len(children) > 1 {
			return "", &WorkflowError{
				Code:    "TARGET_UNIT_REQUIRED",
				Message: "several child units are available; name the target unit",
			}
		}
		return children[0].ID, nil

	case RouteSpecificUnit:
		if req.TargetUnitID == "" {
			return "", &WorkflowError{Code: "TARGET_UNIT_REQUIRED", Message: "this step routes to a specific unit, which was not configured"}
		}
		if _, ok := g.byID[req.TargetUnitID]; !ok {
			return "", &WorkflowError{Code: "UNIT_NOT_FOUND", Message: "the configured unit does not exist in this tenant"}
		}
		return req.TargetUnitID, nil

	case RouteRegionMatch:
		return g.resolveRegion(req, rules, serviceID)

	default:
		return "", &WorkflowError{Code: "UNKNOWN_STRATEGY", Message: fmt.Sprintf("unknown routing strategy %q", strategy)}
	}
}

// resolveRegion matches a request field against unit region codes. Rules are
// consulted first — most specific (service-scoped, lowest priority number)
// first — then a direct region match on the hierarchy.
func (g *UnitGraph) resolveRegion(req RoutingRequest, rules []RoutingRule, serviceID string) (string, error) {
	for _, rule := range rules {
		if !rule.Active || rule.Strategy != RouteRegionMatch {
			continue
		}
		if rule.ServiceID != nil && *rule.ServiceID != serviceID {
			continue
		}
		if rule.MatchField == "" {
			continue
		}
		value, ok := req.Fields[rule.MatchField]
		if !ok || !strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(rule.MatchValue)) {
			continue
		}
		if rule.TargetUnitID != nil {
			if _, exists := g.byID[*rule.TargetUnitID]; exists {
				return *rule.TargetUnitID, nil
			}
		}
	}

	// Fall back to a unit whose region_code equals the request's region.
	region := strings.TrimSpace(req.Fields["region"])
	if region != "" {
		for _, candidate := range g.Descendants(req.CurrentUnitID) {
			unit := g.byID[candidate]
			if unit != nil && unit.Active && strings.EqualFold(unit.RegionCode, region) {
				return unit.ID, nil
			}
		}
	}

	return "", &WorkflowError{
		Code:    "NO_REGION_MATCH",
		Message: "no unit matches the region carried by the request",
	}
}

func (g *UnitGraph) activeChildren(id string) []*OrgUnit {
	out := make([]*OrgUnit, 0, len(g.childrenOf[id]))
	for _, child := range g.childrenOf[id] {
		if child.Active {
			out = append(out, child)
		}
	}
	return out
}

// SelectRule returns the highest-priority routing rule that matches, or false.
// Service-scoped rules outrank tenant-wide ones at equal priority.
func SelectRule(rules []RoutingRule, serviceID string, fields map[string]string) (RoutingRule, bool) {
	var best RoutingRule
	found := false
	for _, rule := range rules {
		if !rule.Active {
			continue
		}
		if rule.ServiceID != nil && *rule.ServiceID != serviceID {
			continue
		}
		if rule.MatchField != "" {
			value, ok := fields[rule.MatchField]
			if !ok || !strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(rule.MatchValue)) {
				continue
			}
		}
		if !found || betterRule(rule, best) {
			best, found = rule, true
		}
	}
	return best, found
}

func betterRule(candidate, current RoutingRule) bool {
	if candidate.Priority != current.Priority {
		return candidate.Priority < current.Priority
	}
	// At equal priority a service-specific rule beats a tenant-wide one.
	return candidate.ServiceID != nil && current.ServiceID == nil
}
