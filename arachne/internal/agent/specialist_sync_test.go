package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestSpecialistAgentActivateRecordsAttributedProposal(t *testing.T) {
	store, err := cognition.NewMemoryStore(32)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := cognition.NewSpine("sync-activate-test", store)
	if err != nil {
		t.Fatal(err)
	}
	specialist, err := NewSpecialistAgent("sync-specialist", SpecialistFunc(func(context.Context, Activation) (Proposal, error) {
		return Proposal{Summary: "candidate", Confidence: 0.75, RequestedActions: []RequestedAction{{Kind: "wait", Payload: json.RawMessage(`{}`)}}}, nil
	}), spine)
	if err != nil {
		t.Fatal(err)
	}

	proposal, activationEvent, err := specialist.Activate(context.Background(), Activation{
		InteractionID: "interaction-1", Input: json.RawMessage(`{"features":[0.2]}`),
	}, []string{"perception-1"})
	if err != nil {
		t.Fatal(err)
	}
	if activationEvent.Kind != cognition.KindActivation || len(activationEvent.ParentEventIDs) != 1 || activationEvent.ParentEventIDs[0] != "perception-1" {
		t.Fatalf("activation event lost its attribution: %+v", activationEvent)
	}
	if proposal.Status != "candidate" || proposal.SpecialistID != "sync-specialist" || proposal.EventID == "" {
		t.Fatalf("proposal was not attributed and recorded: %+v", proposal)
	}
}
