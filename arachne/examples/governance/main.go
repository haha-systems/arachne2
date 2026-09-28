// Package main demonstrates policy-bound decisions for a consequential proposal.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
)

func main() {
	ctx := context.Background()
	store, err := cognition.NewMemoryStore(32)
	fatalIf(err)
	events, err := cognition.NewSpine("governance-example", store)
	fatalIf(err)
	policy := governance.Policy{
		ID: "example-policy", Version: "1",
		Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionMemoryMutation: {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"memory:organism-1"}},
		},
	}
	service, err := governance.New(policy, events)
	fatalIf(err)
	proposal := governance.Proposal{
		ID: "proposal-1", InteractionID: "interaction-1", ProposerID: "planner",
		Class: governance.ActionMemoryMutation, Target: "memory:organism-1",
		Action: "append episode", Payload: json.RawMessage(`{"episode":"example"}`),
		CreatedAt: time.Now().UTC(), SourceEventIDs: []string{"source-1"},
		EvidenceEventIDs: []string{"evidence-1"},
	}
	proposalDigest, err := governance.ProposalDigest(proposal)
	fatalIf(err)
	policyDigest, err := governance.PolicyDigest(policy)
	fatalIf(err)

	pending, err := service.Evaluate(ctx, proposal, nil)
	fatalIf(err)
	printDecision(pending)
	approved, err := service.Evaluate(ctx, proposal, []governance.Approval{{
		ID: "approval-1", ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: policyDigest, Decision: governance.ApprovalApprove,
		Reason: "reviewed evidence", DecidedAt: time.Now().UTC(), SourceEventID: "review-1",
	}})
	fatalIf(err)
	printDecision(approved)
	rejected, err := service.Evaluate(ctx, proposal, []governance.Approval{{
		ID: "approval-2", ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: policyDigest, Decision: governance.ApprovalReject,
		Reason: "target requires more context", DecidedAt: time.Now().UTC(), SourceEventID: "review-2",
	}})
	fatalIf(err)
	printDecision(rejected)
	fmt.Println("No proposal was executed; approval only marks it eligible for a separate authorized executor.")
}

func printDecision(decision governance.Decision) {
	fmt.Printf("%s: outcome=%s policy=%s/%s decision_event=%s reasons=%v\n",
		decision.ProposalID, decision.Outcome, decision.PolicyID, decision.PolicyVersion,
		decision.EventID, decision.Reasons)
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
