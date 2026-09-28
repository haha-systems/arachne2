// Package main demonstrates divergent routing profiles with approval and provenance.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/development"
	"github.com/haha-systems/arachne2/internal/governance"
)

type experiment struct {
	events       *cognition.Spine
	policyDigest string
}

type experience struct {
	id    string
	key   string
	value float64
}

func main() {
	ctx := context.Background()
	events := newSpine()
	policy := governance.Policy{
		ID: "development-policy", Version: "1", Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionHighImpact: {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"}},
		},
	}
	governor, err := governance.New(policy, events)
	fatalIf(err)
	policyDigest, err := governance.PolicyDigest(policy)
	fatalIf(err)
	initial := map[string]float64{"task:design": 0.5, "task:review": 0.5}
	first := newEngine(ctx, "instance-a", initial, governor, events)
	second := newEngine(ctx, "instance-b", initial, governor, events)
	run := experiment{events: events, policyDigest: policyDigest}
	run.applyExperience(ctx, first, experience{id: "experience-a", key: "task:design", value: 0.9})
	run.applyExperience(ctx, second, experience{id: "experience-b", key: "task:review", value: -0.2})
	printSnapshot("after different experiences", first.Snapshot(), second.Snapshot())

	first = newEngine(ctx, "instance-a", initial, governor, events)
	second = newEngine(ctx, "instance-b", initial, governor, events)
	printSnapshot("recovered from event history", first.Snapshot(), second.Snapshot())
}

func newSpine() *cognition.Spine {
	store, err := cognition.NewMemoryStore(128)
	fatalIf(err)
	events, err := cognition.NewSpine("development-example", store)
	fatalIf(err)
	return events
}

func newEngine(ctx context.Context, id string, initial map[string]float64, governor *governance.Service, events *cognition.Spine) *development.Engine {
	engine, err := development.NewEngine(ctx, id, initial, governor, events)
	fatalIf(err)
	return engine
}

func (run experiment) applyExperience(ctx context.Context, engine *development.Engine, experience experience) {
	evidence, err := run.events.Emit(ctx, cognition.Draft{
		CorrelationID: "experience-" + experience.id, Kind: cognition.KindPerception,
		Payload: json.RawMessage(fmt.Sprintf(`{"experience_id":%q,"useful_key":%q}`, experience.id, experience.key)),
	})
	fatalIf(err)
	request := development.RoutingRequest{
		ID: "proposal-" + experience.id, InteractionID: "change-" + experience.id,
		ProposerID: "development-agent", Key: experience.key, Value: experience.value,
		CreatedAt: time.Now().UTC(), EvidenceEventIDs: []string{evidence.EventID},
	}
	proposal, err := engine.PrepareRoutingProposal(request)
	fatalIf(err)
	proposalDigest, err := governance.ProposalDigest(proposal)
	fatalIf(err)
	review, err := run.events.Emit(ctx, cognition.Draft{
		AgentID: "operator", CorrelationID: proposal.InteractionID,
		ParentEventIDs: []string{evidence.EventID}, Kind: cognition.KindDecision,
		Payload: json.RawMessage(`{"decision":"approve evidence-backed routing update"}`),
	})
	fatalIf(err)
	change, decision, err := engine.ApplyRoutingChange(ctx, proposal, []governance.Approval{{
		ID: "approval-" + experience.id, ApproverID: "operator",
		ProposalDigest: proposalDigest, PolicyDigest: run.policyDigest,
		Decision: governance.ApprovalApprove, Reason: "reviewed experience evidence",
		DecidedAt: time.Now().UTC(), SourceEventID: review.EventID,
	}})
	fatalIf(err)
	fmt.Printf("%s: outcome=%s change=%s revision=%d event=%s\n",
		engine.Snapshot().OrganismID, decision.Outcome, change.ID, change.Revision, change.EventID)
}

func printSnapshot(label string, first, second development.Snapshot) {
	fmt.Printf("%s: %s=%v (revision %d), %s=%v (revision %d)\n",
		label, first.OrganismID, first.Routing, first.Revision,
		second.OrganismID, second.Routing, second.Revision)
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
