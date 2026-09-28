package governance

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

var governanceTestTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func newGovernanceTestService(t *testing.T, capacity int) (*Service, *cognition.Spine, *cognition.MemoryStore, Policy) {
	t.Helper()
	store, err := cognition.NewMemoryStore(capacity)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine("org-a", store)
	if err != nil {
		t.Fatal(err)
	}
	policy := Policy{
		ID: "host-policy", Version: "3",
		Approvers: []string{"alice", "bob"},
		Rules: map[ActionClass]Rule{
			ActionHighImpact: {
				MinimumApprovals: 2, RequireEvidence: true,
				AllowedTargets: []string{"org:org-a:routing:planner"},
			},
		},
	}
	service, err := New(policy, events)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return governanceTestTime }
	return service, events, store, policy
}

func governanceTestProposal() Proposal {
	return Proposal{
		ID: "proposal-1", InteractionID: "interaction-1", ProposerID: "agent-1",
		Class: ActionHighImpact, Target: "org:org-a:routing:planner",
		Action: "change routing weight", Payload: json.RawMessage(`{"weight":0.8}`),
		CreatedAt:      governanceTestTime,
		SourceEventIDs: []string{"source-1"}, EvidenceEventIDs: []string{"evidence-1"},
	}
}

func governanceTestApproval(t *testing.T, service *Service, proposal Proposal, id, approver string, decision ApprovalDecision) Approval {
	t.Helper()
	digest, err := ProposalDigest(proposal)
	if err != nil {
		t.Fatal(err)
	}
	return Approval{
		ID: id, ApproverID: approver, ProposalDigest: digest, PolicyDigest: service.digest,
		Decision: decision, Reason: "reviewed evidence", DecidedAt: governanceTestTime.Add(time.Minute),
		SourceEventID: "review-" + approver,
	}
}

func TestEvaluateEnforcesApprovalThresholdAndPreservesProvenance(t *testing.T) {
	service, _, store, _ := newGovernanceTestService(t, 8)
	proposal := governanceTestProposal()
	alice := governanceTestApproval(t, service, proposal, "approval-a", "alice", ApprovalApprove)
	bob := governanceTestApproval(t, service, proposal, "approval-b", "bob", ApprovalApprove)

	pending, err := service.Evaluate(context.Background(), proposal, []Approval{alice})
	if err != nil {
		t.Fatalf("evaluate one approval: %v", err)
	}
	if pending.Outcome != OutcomePending {
		t.Fatalf("one approval outcome = %q, want pending", pending.Outcome)
	}

	approved, err := service.Evaluate(context.Background(), proposal, []Approval{bob, alice})
	if err != nil {
		t.Fatalf("evaluate complete approvals: %v", err)
	}
	if approved.Outcome != OutcomeApproved {
		t.Fatalf("two approval outcome = %q, want approved", approved.Outcome)
	}
	if !reflect.DeepEqual(approved.ApproverIDs, []string{"alice", "bob"}) ||
		!reflect.DeepEqual(approved.ApprovalIDs, []string{"approval-a", "approval-b"}) {
		t.Fatalf("approval provenance was not normalized: %+v", approved)
	}
	wantSources := []string{"evidence-1", "review-alice", "review-bob", "source-1"}
	if !reflect.DeepEqual(approved.SourceEventIDs, wantSources) {
		t.Fatalf("source event IDs = %v, want %v", approved.SourceEventIDs, wantSources)
	}
	if len(approved.Approvals) != 2 || approved.Approvals[0].Reason != "reviewed evidence" {
		t.Fatalf("review records were not preserved: %+v", approved.Approvals)
	}

	events, err := store.Read(context.Background(), 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	decisionEvent := events[len(events)-1]
	if decisionEvent.EventID != approved.EventID || decisionEvent.Kind != cognition.KindGovernance ||
		decisionEvent.AgentID != proposal.ProposerID || decisionEvent.CorrelationID != proposal.InteractionID {
		t.Fatalf("decision event lost attribution: %+v", decisionEvent)
	}
	if !reflect.DeepEqual(decisionEvent.ParentEventIDs, wantSources) {
		t.Fatalf("decision event parents = %v, want %v", decisionEvent.ParentEventIDs, wantSources)
	}
}

func TestEvaluateFailsClosedForRejectedOutOfScopeMissingEvidenceAndStaleApproval(t *testing.T) {
	tests := []struct {
		name     string
		proposal func(Proposal) Proposal
		approval func(*testing.T, *Service, Proposal) []Approval
		reason   string
	}{
		{
			name: "authorized rejection",
			approval: func(t *testing.T, service *Service, proposal Proposal) []Approval {
				return []Approval{governanceTestApproval(t, service, proposal, "approval-a", "alice", ApprovalReject)}
			},
			reason: "an authorized reviewer rejected",
		},
		{
			name:     "target outside policy scope",
			proposal: func(proposal Proposal) Proposal { proposal.Target = "org:org-b:routing:planner"; return proposal },
			reason:   "outside the policy scope",
		},
		{
			name:     "required evidence missing",
			proposal: func(proposal Proposal) Proposal { proposal.EvidenceEventIDs = nil; return proposal },
			reason:   "requires source evidence",
		},
		{
			name:     "approval bound to stale proposal",
			proposal: func(proposal Proposal) Proposal { proposal.Action = "different action"; return proposal },
			approval: func(t *testing.T, service *Service, proposal Proposal) []Approval {
				stale := proposal
				stale.Action = "original action"
				return []Approval{governanceTestApproval(t, service, stale, "approval-a", "alice", ApprovalApprove)}
			},
			reason: "different proposal or policy",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _, _, _ := newGovernanceTestService(t, 4)
			proposal := governanceTestProposal()
			if test.proposal != nil {
				proposal = test.proposal(proposal)
			}
			var approvals []Approval
			if test.approval != nil {
				approvals = test.approval(t, service, proposal)
			}
			decision, err := service.Evaluate(context.Background(), proposal, approvals)
			if err != nil {
				t.Fatalf("evaluate proposal: %v", err)
			}
			if decision.Outcome != OutcomeRejected || len(decision.Reasons) == 0 || !strings.Contains(decision.Reasons[0], test.reason) {
				t.Fatalf("proposal did not fail closed as expected: %+v", decision)
			}
		})
	}
}

func TestEvaluateRejectsMalformedProposalWithoutRecordingDecision(t *testing.T) {
	service, _, store, _ := newGovernanceTestService(t, 4)
	proposal := governanceTestProposal()
	proposal.Payload = json.RawMessage(`{"weight":`)
	if _, err := service.Evaluate(context.Background(), proposal, nil); err == nil {
		t.Fatal("malformed proposal was accepted")
	}
	events, err := store.Read(context.Background(), 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("malformed proposal created %d events", len(events))
	}
}
