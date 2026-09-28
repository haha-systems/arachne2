package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestSelectionIsBoundedAndDoesNotAuthorizeRequestedActions(t *testing.T) {
	ctx := context.Background()
	eventStore, err := cognition.NewMemoryStore(16)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine("org-a", eventStore)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{MaxRuns: 1, MaxProposals: 3, Capacity: 2, MaxRecipients: 2, MaxBroadcastBytes: 1024}
	workspace, err := New(config, events)
	if err != nil {
		t.Fatal(err)
	}
	request := Request{ID: "run-1", InteractionID: "interaction-1", SourceEventIDs: []string{"request-source"}}
	if err := workspace.Open(ctx, request); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	if err := workspace.Open(ctx, Request{ID: "run-2", InteractionID: "interaction-2"}); !errors.Is(err, ErrFull) {
		t.Fatalf("second run error = %v, want capacity error", err)
	}
	proposals := []agent.Proposal{
		{ID: "p-low", EventID: "event-low", InteractionID: request.InteractionID, SpecialistID: "specialist-c", Status: "candidate", Confidence: 0.4,
			RequestedActions: []agent.RequestedAction{{Kind: "protected.write", Payload: json.RawMessage(`{"value":1}`)}}},
		{ID: "p-high", EventID: "event-high", InteractionID: request.InteractionID, SpecialistID: "specialist-a", Status: "candidate", Confidence: 0.9},
		{ID: "p-mid", EventID: "event-mid", InteractionID: request.InteractionID, SpecialistID: "specialist-b", Status: "candidate", Confidence: 0.7},
	}
	for _, proposal := range proposals {
		if err := workspace.Submit(ctx, request.ID, proposal); err != nil {
			t.Fatalf("submit proposal %q: %v", proposal.ID, err)
		}
	}
	if err := workspace.Submit(ctx, request.ID, agent.Proposal{
		ID: "p-overflow", EventID: "event-overflow", InteractionID: request.InteractionID, SpecialistID: "specialist-d", Status: "candidate",
	}); !errors.Is(err, ErrFull) {
		t.Fatalf("proposal overflow error = %v, want capacity error", err)
	}
	if _, err := workspace.SelectWithPolicy(ctx, request.ID, SelectionPolicy{Capacity: 3}); err == nil {
		t.Fatal("selection policy increased capacity above the configured bound")
	}

	report, err := workspace.Select(ctx, request.ID)
	if err != nil {
		t.Fatalf("select proposals: %v", err)
	}
	if !reflect.DeepEqual(report.Selection.SelectedIDs, []string{"p-high", "p-mid"}) || report.Selection.Capacity != 2 {
		t.Fatalf("selection exceeded capacity or lost ranking: %+v", report.Selection)
	}
	if len(report.Proposals) != 3 || len(report.Proposals[0].RequestedActions) != 1 {
		t.Fatalf("selection converted or dropped proposal content: %+v", report.Proposals)
	}
	history, err := eventStore.Read(ctx, 0, 16)
	if err != nil {
		t.Fatal(err)
	}
	var selectionEvent cognition.Event
	for _, event := range history {
		if event.Kind == cognition.KindAction {
			t.Fatalf("proposal admission or selection emitted an action event: %+v", event)
		}
		if event.EventID == report.Selection.EventID {
			selectionEvent = event
		}
	}
	if selectionEvent.EventID == "" || selectionEvent.Kind != cognition.KindSelection {
		t.Fatalf("selection event was not recorded: %+v", selectionEvent)
	}
	for _, parent := range []string{"request-source", "event-low", "event-high", "event-mid"} {
		if !slices.Contains(selectionEvent.ParentEventIDs, parent) {
			t.Errorf("selection event omitted proposal/source parent %q: %v", parent, selectionEvent.ParentEventIDs)
		}
	}
}
