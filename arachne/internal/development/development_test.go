package development

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
)

var developmentTestTime = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)

func newDevelopmentTestEngine(t *testing.T, eventCapacity int) (*Engine, *governance.Service, *cognition.Spine, *cognition.MemoryStore) {
	t.Helper()
	store, err := cognition.NewMemoryStore(eventCapacity)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine("org-a", store)
	if err != nil {
		t.Fatal(err)
	}
	policy := governance.Policy{
		ID: "development-policy", Version: "1", Approvers: []string{"reviewer"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionHighImpact: {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"}},
		},
	}
	governor, err := governance.New(policy, events)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(context.Background(), "org-a", map[string]float64{"planner": 0.5}, governor, events)
	if err != nil {
		t.Fatal(err)
	}
	engine.now = func() time.Time { return developmentTestTime }
	return engine, governor, events, store
}

func developmentTestProposal(t *testing.T, engine *Engine, id string) governance.Proposal {
	t.Helper()
	proposal, err := engine.PrepareRoutingProposal(RoutingRequest{
		ID: id, InteractionID: "interaction-1", ProposerID: "agent-1", Key: "planner", Value: 0.8,
		CreatedAt: developmentTestTime, SourceEventIDs: []string{"source-1"}, EvidenceEventIDs: []string{"evidence-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return proposal
}

func developmentTestApproval(t *testing.T, proposal governance.Proposal, decision governance.ApprovalDecision) governance.Approval {
	t.Helper()
	digest, err := governance.ProposalDigest(proposal)
	if err != nil {
		t.Fatal(err)
	}
	policyDigest, err := governance.PolicyDigest(governance.Policy{
		ID: "development-policy", Version: "1", Approvers: []string{"reviewer"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionHighImpact: {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return governance.Approval{
		ID: "approval-1", ApproverID: "reviewer", ProposalDigest: digest, PolicyDigest: policyDigest,
		Decision: decision, Reason: "reviewed", DecidedAt: developmentTestTime.Add(time.Minute),
		SourceEventID: "approval-event",
	}
}

func TestApplyRoutingChangeRequiresApprovalAndPreservesEventLineage(t *testing.T) {
	tests := []struct {
		name       string
		decision   governance.ApprovalDecision
		approvals  bool
		wantResult governance.Outcome
		wantChange bool
	}{
		{name: "pending", wantResult: governance.OutcomePending},
		{name: "rejected", decision: governance.ApprovalReject, approvals: true, wantResult: governance.OutcomeRejected},
		{name: "approved", decision: governance.ApprovalApprove, approvals: true, wantResult: governance.OutcomeApproved, wantChange: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine, _, events, _ := newDevelopmentTestEngine(t, 8)
			proposal := developmentTestProposal(t, engine, "proposal-1")
			var approvals []governance.Approval
			if test.approvals {
				approvals = []governance.Approval{developmentTestApproval(t, proposal, test.decision)}
			}
			change, decision, err := engine.ApplyRoutingChange(context.Background(), proposal, approvals)
			if err != nil {
				t.Fatalf("apply routing change: %v", err)
			}
			if decision.Outcome != test.wantResult {
				t.Fatalf("decision = %q, want %q", decision.Outcome, test.wantResult)
			}
			snapshot := engine.Snapshot()
			if test.wantChange {
				if snapshot.Revision != 1 || snapshot.Routing["planner"] != 0.8 || change.EventID == "" {
					t.Fatalf("approved change was not applied: change=%+v snapshot=%+v", change, snapshot)
				}
				if change.GovernanceDecision != decision.ID || change.GovernanceEventID != decision.EventID ||
					!reflect.DeepEqual(change.SourceEventIDs, []string{"source-1"}) ||
					!reflect.DeepEqual(change.EvidenceEventIDs, []string{"evidence-1"}) {
					t.Fatalf("change provenance was not preserved: %+v", change)
				}
				stored, err := events.Read(context.Background(), 0, 8)
				if err != nil {
					t.Fatal(err)
				}
				developmentEvent := stored[len(stored)-1]
				if developmentEvent.EventID != change.EventID || developmentEvent.Kind != cognition.KindDevelopment ||
					!reflect.DeepEqual(developmentEvent.ParentEventIDs, []string{"evidence-1", decision.EventID, "source-1"}) {
					t.Fatalf("development event lineage = %+v", developmentEvent)
				}
			} else if snapshot.Revision != 0 || snapshot.Routing["planner"] != 0.5 || len(snapshot.Changes) != 0 {
				t.Fatalf("non-approved proposal mutated routing: %+v", snapshot)
			}
		})
	}
}

func TestStaleRoutingProposalCannotBeAppliedTwice(t *testing.T) {
	engine, _, _, store := newDevelopmentTestEngine(t, 8)
	proposal := developmentTestProposal(t, engine, "proposal-1")
	approval := developmentTestApproval(t, proposal, governance.ApprovalApprove)
	if _, _, err := engine.ApplyRoutingChange(context.Background(), proposal, []governance.Approval{approval}); err != nil {
		t.Fatalf("apply initial proposal: %v", err)
	}
	before := engine.Snapshot()
	if _, _, err := engine.ApplyRoutingChange(context.Background(), proposal, []governance.Approval{approval}); err == nil {
		t.Fatal("stale proposal was applied a second time")
	}
	after := engine.Snapshot()
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("stale proposal changed routing: before=%+v after=%+v", before, after)
	}
	events, err := store.Read(context.Background(), 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("stale retry emitted %d extra events; history has %d events", len(events)-2, len(events))
	}
}

func TestDevelopmentEventFailureLeavesRoutingUnchanged(t *testing.T) {
	engine, _, _, _ := newDevelopmentTestEngine(t, 1)
	proposal := developmentTestProposal(t, engine, "proposal-1")
	approval := developmentTestApproval(t, proposal, governance.ApprovalApprove)
	_, decision, err := engine.ApplyRoutingChange(context.Background(), proposal, []governance.Approval{approval})
	if err == nil || !strings.Contains(err.Error(), "profile unchanged") {
		t.Fatalf("development event failure = %v, want unchanged-profile error", err)
	}
	if decision.Outcome != governance.OutcomeApproved {
		t.Fatalf("governance decision = %q, want approved", decision.Outcome)
	}
	if snapshot := engine.Snapshot(); snapshot.Revision != 0 || snapshot.Routing["planner"] != 0.5 || len(snapshot.Changes) != 0 {
		t.Fatalf("failed provenance recording mutated routing: %+v", snapshot)
	}
}

func TestNewEngineRestoresValidHistoryAndRejectsInconsistentHistory(t *testing.T) {
	engine, governor, events, _ := newDevelopmentTestEngine(t, 8)
	proposal := developmentTestProposal(t, engine, "proposal-1")
	approval := developmentTestApproval(t, proposal, governance.ApprovalApprove)
	if _, _, err := engine.ApplyRoutingChange(context.Background(), proposal, []governance.Approval{approval}); err != nil {
		t.Fatalf("apply change: %v", err)
	}
	restored, err := NewEngine(context.Background(), "org-a", map[string]float64{"planner": 0.5}, governor, events)
	if err != nil {
		t.Fatalf("restore valid history: %v", err)
	}
	if !reflect.DeepEqual(restored.Snapshot(), engine.Snapshot()) {
		t.Fatalf("restored snapshot differs: got=%+v want=%+v", restored.Snapshot(), engine.Snapshot())
	}

	badStore, err := cognition.NewMemoryStore(4)
	if err != nil {
		t.Fatal(err)
	}
	badPayload, err := json.Marshal(map[string]any{
		"schema_version": schemaVersion, "operation": "routing_change_applied",
		"change": Change{ID: "bad", OrganismID: "org-a", Revision: 1, Key: "planner", Previous: 0.4, Value: 0.8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := badStore.Append(context.Background(), cognition.Event{
		EventID: "org-a:event:1", Sequence: 1, OrganismID: "org-a", Kind: cognition.KindDevelopment, Payload: badPayload,
	}); err != nil {
		t.Fatal(err)
	}
	badEvents, err := cognition.NewSpine("org-a", badStore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewEngine(context.Background(), "org-a", map[string]float64{"planner": 0.5}, governor, badEvents); err == nil {
		t.Fatal("inconsistent recorded before-value was restored")
	}
}
