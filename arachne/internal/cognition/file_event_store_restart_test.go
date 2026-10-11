package cognition_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/development"
	"github.com/haha-systems/arachne2/internal/governance"
)

func TestFileEventStoreRestartRebuildsDevelopmentRoutingProfile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	policy := governance.Policy{
		ID: "development-policy", Version: "1", Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionHighImpact: {
				MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"},
			},
		},
	}
	policyDigest, err := governance.PolicyDigest(policy)
	if err != nil {
		t.Fatal(err)
	}
	initial := map[string]float64{"task:design": 0.5, "task:review": 0.5}

	store, err := cognition.OpenFileEventStore(path, "instance-a", 128)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := cognition.NewSpine("instance-a", store)
	if err != nil {
		t.Fatal(err)
	}
	governor, err := governance.New(policy, spine)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := development.NewEngine(ctx, "instance-a", initial, governor, spine)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := spine.Emit(ctx, cognition.Draft{
		CorrelationID: "experience-design", Kind: cognition.KindPerception,
		Payload: json.RawMessage(`{"experience_id":"experience-a","useful_key":"task:design"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := engine.PrepareRoutingProposal(development.RoutingRequest{
		ID: "proposal-a", InteractionID: "change-a", ProposerID: "development-agent",
		Key: "task:design", Value: 0.9, CreatedAt: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC),
		EvidenceEventIDs: []string{evidence.EventID},
	})
	if err != nil {
		t.Fatal(err)
	}
	proposalDigest, err := governance.ProposalDigest(proposal)
	if err != nil {
		t.Fatal(err)
	}
	review, err := spine.Emit(ctx, cognition.Draft{
		AgentID: "operator", CorrelationID: proposal.InteractionID,
		ParentEventIDs: []string{evidence.EventID}, Kind: cognition.KindDecision,
		Payload: json.RawMessage(`{"decision":"approve evidence-backed routing update"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, decision, err := engine.ApplyRoutingChange(ctx, proposal, []governance.Approval{{
		ID: "approval-a", ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: policyDigest, Decision: governance.ApprovalApprove,
		Reason: "reviewed experience evidence", DecidedAt: time.Date(2026, 10, 11, 0, 1, 0, 0, time.UTC),
		SourceEventID: review.EventID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Outcome != governance.OutcomeApproved {
		t.Fatalf("decision outcome = %q, want approved", decision.Outcome)
	}
	want := engine.Snapshot()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := cognition.OpenFileEventStore(path, "instance-a", 128)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("close reopened store: %v", err)
		}
	}()
	restartedSpine, err := cognition.NewSpine("instance-a", reopened)
	if err != nil {
		t.Fatal(err)
	}
	restartedGovernor, err := governance.New(policy, restartedSpine)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := development.NewEngine(ctx, "instance-a", initial, restartedGovernor, restartedSpine)
	if err != nil {
		t.Fatal(err)
	}
	if got := restarted.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("restarted routing snapshot = %+v, want %+v", got, want)
	}
}
