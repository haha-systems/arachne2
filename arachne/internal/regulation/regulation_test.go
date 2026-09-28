package regulation

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/workspace"
)

func TestDeclaredSignalsAdjustWorkspaceSelectionAndRemainAttributable(t *testing.T) {
	ctx := context.Background()
	eventStore, err := cognition.NewMemoryStore(32)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine("org-a", eventStore)
	if err != nil {
		t.Fatal(err)
	}
	regulator, err := New(DefaultPolicy(), events)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 8, 9, 10, 11, 0, time.UTC)
	regulator.now = func() time.Time { return now }
	baseline, err := regulator.Evaluate(ctx, Input{
		InteractionID: "baseline", Expected: json.RawMessage(`1`), Observed: json.RawMessage(`1`),
		DeclaredSalience: 0.1, SpecialistCapacity: 4, ActionCapacity: 4, WorkspaceCapacity: 3,
	})
	if err != nil {
		t.Fatalf("evaluate baseline: %v", err)
	}
	if baseline.WorkspacePolicy.Capacity != 3 || baseline.WorkspacePolicy.RequireEvidence {
		t.Fatalf("low declared signals changed the baseline policy: %+v", baseline.WorkspacePolicy)
	}

	signals, err := regulator.Evaluate(ctx, Input{
		InteractionID: "regulated", SessionID: "session-1", SourceEventIDs: []string{"sensor-event"},
		Expected: json.RawMessage(`0`), Observed: json.RawMessage(`1`), DeclaredSalience: 0.8,
		ActiveSpecialists: 3, SpecialistCapacity: 4, PendingActions: 3, ActionCapacity: 4, WorkspaceCapacity: 3,
	})
	if err != nil {
		t.Fatalf("evaluate declared signals: %v", err)
	}
	policy := signals.WorkspacePolicy
	if policy.Capacity != 1 || !policy.RequireEvidence || len(policy.SignalEventIDs) != 2 {
		t.Fatalf("explicit signals did not produce bounded evidence policy: %+v", policy)
	}
	if !reflect.DeepEqual(signals.SourceEventIDs, []string{"sensor-event"}) ||
		!reflect.DeepEqual(signals.Prediction.SourceEventIDs, []string{"sensor-event"}) {
		t.Fatalf("regulation signal lost source attribution: %+v", signals)
	}

	coordinator, err := workspace.New(workspace.Config{
		MaxRuns: 2, MaxProposals: 4, Capacity: 3, MaxRecipients: 2, MaxBroadcastBytes: 1024,
	}, events)
	if err != nil {
		t.Fatal(err)
	}
	request := workspace.Request{ID: "regulated-run", InteractionID: "regulated", SessionID: "session-1", SourceEventIDs: []string{"sensor-event"}}
	if err := coordinator.Open(ctx, request); err != nil {
		t.Fatal(err)
	}
	proposals := []agent.Proposal{
		{ID: "unsupported-high", EventID: "proposal-event-high", InteractionID: request.InteractionID, SpecialistID: "a", Status: "candidate", Confidence: 0.95},
		{ID: "supported-mid", EventID: "proposal-event-mid", InteractionID: request.InteractionID, SpecialistID: "b", Status: "candidate", Confidence: 0.8, EvidenceEventIDs: []string{"proposal-evidence"}},
		{ID: "supported-low", EventID: "proposal-event-low", InteractionID: request.InteractionID, SpecialistID: "c", Status: "candidate", Confidence: 0.6, EvidenceEventIDs: []string{"proposal-evidence-2"}},
	}
	for _, proposal := range proposals {
		if err := coordinator.Submit(ctx, request.ID, proposal); err != nil {
			t.Fatalf("submit %q: %v", proposal.ID, err)
		}
	}
	selected, err := coordinator.SelectWithPolicy(ctx, request.ID, policy)
	if err != nil {
		t.Fatalf("select with regulation policy: %v", err)
	}
	if !reflect.DeepEqual(selected.Selection.SelectedIDs, []string{"supported-mid"}) || !selected.Selection.EvidenceRequired {
		t.Fatalf("workspace did not apply declared capacity/evidence policy: %+v", selected.Selection)
	}
	if !reflect.DeepEqual(selected.Selection.SignalEventIDs, policy.SignalEventIDs) ||
		!reflect.DeepEqual(selected.Selection.Reasons, policy.Reasons) {
		t.Fatalf("selection lost regulation provenance: %+v", selected.Selection)
	}

	history, err := eventStore.Read(ctx, 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	var selectionEvent cognition.Event
	for _, event := range history {
		if event.Kind == cognition.KindAction {
			t.Fatalf("workspace selection authorized an action: %+v", event)
		}
		if event.EventID == selected.Selection.EventID {
			selectionEvent = event
		}
	}
	for _, parent := range append(append([]string{}, policy.SignalEventIDs...), "sensor-event") {
		if !slices.Contains(selectionEvent.ParentEventIDs, parent) {
			t.Errorf("selection event omitted signal/source parent %q: %v", parent, selectionEvent.ParentEventIDs)
		}
	}
}
