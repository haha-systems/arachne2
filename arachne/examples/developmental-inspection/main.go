// Package main demonstrates provenance-linked developmental reports and divergence.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/development"
	"github.com/haha-systems/arachne2/internal/governance"
	"github.com/haha-systems/arachne2/internal/inspection"
	"github.com/haha-systems/arachne2/internal/regulation"
)

func main() {
	ctx := context.Background()
	left, leftEvents, initial := buildOrganism(ctx, "organism-a", "task:design", 0.9)
	right, rightEvents, _ := buildOrganism(ctx, "organism-b", "task:review", -0.2)
	leftInspector, err := inspection.New(leftEvents)
	fatalIf(err)
	rightInspector, err := inspection.New(rightEvents)
	fatalIf(err)
	leftReport, err := leftInspector.Inspect(ctx, left.OrganismID)
	fatalIf(err)
	rightReport, err := rightInspector.Inspect(ctx, right.OrganismID)
	fatalIf(err)
	comparison, err := inspection.CompareReports(leftReport, rightReport)
	fatalIf(err)
	fmt.Printf("%s: revision=%d routing=%v activities=%d\n", left.OrganismID, left.Revision, left.Routing, len(leftReport.Activities))
	fmt.Printf("%s: revision=%d routing=%v activities=%d\n", right.OrganismID, right.Revision, right.Routing, len(rightReport.Activities))
	evolution, err := inspection.ChangesSince(initial, leftReport)
	fatalIf(err)
	fmt.Printf("%s changes since initial sequence %d:\n", left.OrganismID, initial.Through)
	for _, activity := range evolution.Changes {
		fmt.Printf("  %s; event=%s; provenance=%v; governance=%v\n",
			activity.Summary, activity.EventID, activity.ProvenanceEventIDs, activity.GovernanceEventIDs)
	}
	fmt.Printf("shared activities=%d\n", comparison.Diff.Shared)
	for _, activity := range comparison.Diff.LeftOnly {
		fmt.Printf("%s only: %s; event=%s; provenance=%v; governance=%v\n",
			left.OrganismID, activity.Summary, activity.EventID, activity.ProvenanceEventIDs, activity.GovernanceEventIDs)
	}
	for _, activity := range comparison.Diff.RightOnly {
		fmt.Printf("%s only: %s; event=%s; provenance=%v; governance=%v\n",
			right.OrganismID, activity.Summary, activity.EventID, activity.ProvenanceEventIDs, activity.GovernanceEventIDs)
	}
}

func buildOrganism(ctx context.Context, id, route string, weight float64) (development.Snapshot, *cognition.Spine, inspection.Report) {
	store, err := cognition.NewMemoryStore(128)
	fatalIf(err)
	events, err := cognition.NewSpine(id, store)
	fatalIf(err)
	engine, policyDigest := newDevelopmentEngine(ctx, id, events)
	evidence, err := events.Emit(ctx, cognition.Draft{
		Kind:    cognition.KindMemory,
		Payload: json.RawMessage(fmt.Sprintf(`{"operation":"episode_recorded","episode_id":"experience-%s","kind":"task_observation"}`, id)),
	})
	fatalIf(err)
	inspector, err := inspection.New(events)
	fatalIf(err)
	initial, err := inspector.Inspect(ctx, id)
	fatalIf(err)
	proposal, err := engine.PrepareRoutingProposal(development.RoutingRequest{
		ID: "proposal-" + id, InteractionID: "interaction-" + id, ProposerID: "development-agent",
		Key: route, Value: weight, CreatedAt: time.Now().UTC(), EvidenceEventIDs: []string{evidence.EventID},
	})
	fatalIf(err)
	proposalDigest, err := governance.ProposalDigest(proposal)
	fatalIf(err)
	review, err := events.Emit(ctx, cognition.Draft{
		AgentID: "operator", ParentEventIDs: []string{evidence.EventID}, Kind: cognition.KindDecision,
		Payload: json.RawMessage(`{"decision":"approve evidence-backed routing update"}`),
	})
	fatalIf(err)
	_, decision, err := engine.ApplyRoutingChange(ctx, proposal, []governance.Approval{{
		ID: "approval-" + id, ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: policyDigest, Decision: governance.ApprovalApprove,
		Reason: "reviewed experience evidence", DecidedAt: time.Now().UTC(), SourceEventID: review.EventID,
	}})
	fatalIf(err)
	if decision.Outcome != governance.OutcomeApproved {
		panic("routing development was not approved")
	}
	_, err = events.Emit(ctx, cognition.Draft{
		AgentID: "specialist-analyst", ParentEventIDs: []string{evidence.EventID}, Kind: cognition.KindProposal,
		Payload: json.RawMessage(fmt.Sprintf(`{"specialist_id":"specialist-analyst","status":"proposed","summary":"reviewed %s evidence"}`, route)),
	})
	fatalIf(err)
	regulator, err := regulation.New(regulation.DefaultPolicy(), events)
	fatalIf(err)
	_, err = regulator.Evaluate(ctx, regulation.Input{
		InteractionID: "regulation-" + id, SourceEventIDs: []string{evidence.EventID},
		Expected: json.RawMessage(`{"result":1}`), Observed: json.RawMessage(`{"result":0}`),
		DeclaredSalience: 0.5, ActiveSpecialists: 1, SpecialistCapacity: 4,
		PendingActions: 0, ActionCapacity: 4, WorkspaceCapacity: 4,
	})
	fatalIf(err)
	return engine.Snapshot(), events, initial
}

func newDevelopmentEngine(ctx context.Context, id string, events *cognition.Spine) (*development.Engine, string) {
	policy := governance.Policy{
		ID: "inspection-development-policy", Version: "1", Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionHighImpact: {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"}},
		},
	}
	governor, err := governance.New(policy, events)
	fatalIf(err)
	policyDigest, err := governance.PolicyDigest(policy)
	fatalIf(err)
	engine, err := development.NewEngine(ctx, id, map[string]float64{"task:design": 0.5, "task:review": 0.5}, governor, events)
	fatalIf(err)
	return engine, policyDigest
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
