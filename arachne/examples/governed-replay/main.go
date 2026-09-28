// Package main demonstrates replay cues followed by a separate governance decision.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
	"github.com/haha-systems/arachne2/internal/memory"
)

func main() {
	ctx := context.Background()
	memories, events := newRuntime()
	episodeIDs := recordEpisodes(ctx, memories, events)
	plan, err := memories.ScheduleReplay(ctx, memory.ReplayPlan{
		InteractionID: "interaction-replay", RequestedBy: "planner",
		ScheduledAt: time.Now().UTC(), EpisodeIDs: episodeIDs,
	})
	fatalIf(err)
	run, err := memories.RunReplay(ctx, plan)
	fatalIf(err)
	fmt.Printf("replay event=%s plan=%s cues=%d episode_order=%v\n",
		run.EventID, run.PlanID, len(run.Cues), run.InputEpisodeIDs)
	decision := approveReplayFollowup(ctx, events, run)
	fmt.Printf("separate governance decision=%s event=%s\n", decision.Outcome, decision.EventID)
	fmt.Println("Replay produced attention cues only; the approved proposal remains unexecuted.")
}

func newRuntime() (*memory.Service, *cognition.Spine) {
	store, err := memory.NewMemoryStore("organism-1")
	fatalIf(err)
	eventStore, err := cognition.NewMemoryStore(64)
	fatalIf(err)
	events, err := cognition.NewSpine("governed-replay-example", eventStore)
	fatalIf(err)
	memories, err := memory.NewService("organism-1", store, events)
	fatalIf(err)
	return memories, events
}

func recordEpisodes(ctx context.Context, memories *memory.Service, events *cognition.Spine) []string {
	sourceA := emitSource(ctx, events, "interaction-source-a")
	sourceB := emitSource(ctx, events, "interaction-source-b")
	first, err := memories.RecordEpisode(ctx, memory.Episode{
		ID: "episode-a", Source: memory.SourceRef{Kind: "example", ID: "work-a"},
		SourceEventIDs: []string{sourceA.EventID}, OccurredAt: time.Now().UTC().Add(-2 * time.Hour),
		Kind: "useful-observation", Content: json.RawMessage(`{"result":"stable"}`),
	})
	fatalIf(err)
	second, err := memories.RecordEpisode(ctx, memory.Episode{
		ID: "episode-b", Source: memory.SourceRef{Kind: "example", ID: "work-b"},
		SourceEventIDs: []string{sourceB.EventID}, OccurredAt: time.Now().UTC().Add(-time.Hour),
		Kind: "useful-observation", Content: json.RawMessage(`{"result":"stable"}`),
	})
	fatalIf(err)
	return []string{first.ID, second.ID}
}

func approveReplayFollowup(ctx context.Context, events *cognition.Spine, run memory.ReplayRun) governance.Decision {
	replayEvidence := make([]string, 0)
	for _, cue := range run.Cues {
		replayEvidence = append(replayEvidence, cue.SourceEventIDs...)
	}
	reviewEvent, err := events.Emit(ctx, cognition.Draft{
		AgentID: "operator", CorrelationID: "interaction-replay",
		ParentEventIDs: []string{run.EventID}, Kind: cognition.KindDecision,
		Payload: json.RawMessage(`{"decision":"approve replay-derived proposal"}`),
	})
	fatalIf(err)

	policy := governance.Policy{
		ID: "memory-change", Version: "1", Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionMemoryMutation: {
				MinimumApprovals: 1, RequireEvidence: true,
				AllowedTargets: []string{"memory:organism-1"},
			},
		},
	}
	governor, err := governance.New(policy, events)
	fatalIf(err)
	proposal := governance.Proposal{
		ID: "replay-derived-proposal", InteractionID: "interaction-replay", ProposerID: "planner",
		Class: governance.ActionMemoryMutation, Target: "memory:organism-1",
		Action:    "retain a replay-derived candidate",
		Payload:   json.RawMessage(fmt.Sprintf(`{"replay_run_id":%q}`, run.ID)),
		CreatedAt: time.Now().UTC(), SourceEventIDs: []string{run.EventID},
		EvidenceEventIDs: replayEvidence,
	}
	proposalDigest, err := governance.ProposalDigest(proposal)
	fatalIf(err)
	policyDigest, err := governance.PolicyDigest(policy)
	fatalIf(err)
	decision, err := governor.Evaluate(ctx, proposal, []governance.Approval{{
		ID: "operator-review", ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: policyDigest, Decision: governance.ApprovalApprove,
		Reason: "reviewed replay output", DecidedAt: time.Now().UTC(), SourceEventID: reviewEvent.EventID,
	}})
	fatalIf(err)
	return decision
}

func emitSource(ctx context.Context, events *cognition.Spine, interactionID string) cognition.Event {
	event, err := events.Emit(ctx, cognition.Draft{
		CorrelationID: interactionID, Kind: cognition.KindPerception,
		Payload: json.RawMessage(`{"task":"example observation"}`),
	})
	fatalIf(err)
	return event
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
