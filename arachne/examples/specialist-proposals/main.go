// Package main demonstrates attributed specialist proposals without executing them.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
)

func main() {
	ctx := context.Background()
	eventStore, err := cognition.NewMemoryStore(128)
	fatalIf(err)
	events, err := cognition.NewSpine("specialist-demo", eventStore)
	fatalIf(err)
	stimulus, err := events.Emit(ctx, cognition.Draft{
		Kind:    cognition.KindPerception,
		Payload: json.RawMessage(`{"task":"review a change","risk":"medium"}`),
	})
	fatalIf(err)

	coordinator := make(chan []agent.Proposal, 1)
	supervisor, err := agent.NewSupervisor(8)
	fatalIf(err)
	registerSpecialist(supervisor, events, "planner", "Prepare a small reversible plan.", stimulus.EventID)
	registerSpecialist(supervisor, events, "critic", "Inspect for likely regressions.", stimulus.EventID)
	fatalIf(supervisor.Register("coordinator", agent.Func(func(ctx context.Context, inbox <-chan agent.Message, sender agent.Sender) error {
		activation, err := json.Marshal(agent.Activation{
			InteractionID: "interaction-1", SessionID: "session-1",
			Input:          json.RawMessage(`{"task":"review a change","risk":"medium"}`),
			SourceEventIDs: []string{stimulus.EventID},
		})
		if err != nil {
			return err
		}
		for _, specialistID := range []string{"planner", "critic"} {
			if err := sender.Send(ctx, specialistID, agent.SpecialistActivateMessage, activation); err != nil {
				return err
			}
		}
		proposals := make([]agent.Proposal, 0, 2)
		for len(proposals) < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case message := <-inbox:
				if message.Kind != agent.SpecialistProposalMessage {
					continue
				}
				var proposal agent.Proposal
				if err := json.Unmarshal(message.Payload, &proposal); err != nil {
					return err
				}
				proposals = append(proposals, proposal)
			}
		}
		coordinator <- proposals
		return nil
	})))
	fatalIf(supervisor.Start(ctx))
	proposals := <-coordinator
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	fatalIf(supervisor.Stop(stopCtx))
	for _, proposal := range proposals {
		fmt.Printf("%s (%s): %s; requested actions: %d\n", proposal.SpecialistID, proposal.Status, proposal.Summary, len(proposal.RequestedActions))
	}
	allEvents, err := events.Read(ctx, 0, 128)
	fatalIf(err)
	fmt.Printf("shared cognitive events: %d\n", len(allEvents))
}

func registerSpecialist(supervisor *agent.Supervisor, events *cognition.Spine, id, summary, sourceEventID string) {
	implementation := agent.SpecialistFunc(func(_ context.Context, _ agent.Activation) (agent.Proposal, error) {
		return agent.Proposal{
			Summary: summary, Confidence: 0.7,
			EvidenceEventIDs: []string{sourceEventID},
			RequestedActions: []agent.RequestedAction{{
				Kind: "review_note", Payload: json.RawMessage(`{"note":"candidate only"}`),
			}},
		}, nil
	})
	specialist, err := agent.NewSpecialistAgent(id, implementation, events)
	fatalIf(err)
	fatalIf(supervisor.Register(id, specialist))
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
