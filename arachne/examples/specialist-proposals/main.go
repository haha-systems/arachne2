// Package main demonstrates attributed specialist proposals without executing them.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/workspace"
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

	coordinator := make(chan workspace.Report, 1)
	broadcasts := make(chan int, 1)
	supervisor, err := agent.NewSupervisor(8)
	fatalIf(err)
	registerSpecialist(supervisor, events, "planner", "Prepare a small reversible plan.", stimulus.EventID)
	registerSpecialist(supervisor, events, "critic", "Inspect for likely regressions.", stimulus.EventID)
	workspaceConfig := workspace.DefaultConfig()
	workspaceConfig.Capacity = 1
	proposalWorkspace, err := workspace.New(workspaceConfig, events)
	fatalIf(err)
	fatalIf(supervisor.Register("coordinator", agent.Func(func(ctx context.Context, inbox <-chan agent.Message, sender agent.Sender) error {
		if err := proposalWorkspace.Open(ctx, workspace.Request{
			ID: "workspace-1", InteractionID: "interaction-1", SessionID: "session-1",
			SourceEventIDs: []string{stimulus.EventID},
		}); err != nil {
			return err
		}
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
				if err := proposalWorkspace.Submit(ctx, "workspace-1", proposal); err != nil {
					return err
				}
				proposals = append(proposals, proposal)
			}
		}
		if _, err := proposalWorkspace.Select(ctx, "workspace-1"); err != nil {
			return err
		}
		report, err := proposalWorkspace.Broadcast(ctx, "workspace-1", "one entry is available; select the higher-confidence candidate", []string{"observer"}, sender)
		if err != nil {
			return err
		}
		coordinator <- report
		return nil
	})))
	fatalIf(supervisor.Register("observer", agent.Func(func(ctx context.Context, inbox <-chan agent.Message, _ agent.Sender) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message := <-inbox:
			if message.Kind != workspace.BroadcastMessage {
				return fmt.Errorf("unexpected observer message %q", message.Kind)
			}
			var broadcast struct {
				Proposals []agent.Proposal `json:"proposals"`
			}
			if err := json.Unmarshal(message.Payload, &broadcast); err != nil {
				return err
			}
			broadcasts <- len(broadcast.Proposals)
			return nil
		}
	})))
	fatalIf(supervisor.Start(ctx))
	report := <-coordinator
	broadcastCount := <-broadcasts
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	fatalIf(supervisor.Stop(stopCtx))
	for _, entry := range report.Selection.Entries {
		fmt.Printf("%s: selected=%t; reason=%s\n", entry.Specialist, entry.Selected, entry.Reason)
	}
	fmt.Printf("broadcast proposals received: %d\n", broadcastCount)
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
