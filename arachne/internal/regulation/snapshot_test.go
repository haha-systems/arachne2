package regulation

import (
	"context"
	"testing"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestRegulatorStateSnapshotPreservesConditionsWithoutAliasing(t *testing.T) {
	ctx := context.Background()
	events, _ := cognition.NewSpine("regulation-snapshot", mustEventStore(t))
	regulator, err := New(DefaultPolicy(), events)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := regulator.Evaluate(ctx, Input{
		InteractionID:      "interaction-1",
		ConditionDeltas:    map[string]float64{"internal.energy": 0.25},
		SpecialistCapacity: 1, ActionCapacity: 1, WorkspaceCapacity: 4,
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := regulator.SnapshotState()
	copyForRestore := regulator.SnapshotState()
	snapshot.Conditions[0].Value = -10
	if got := regulator.SnapshotState().Conditions[0].Value; got != 0.25 {
		t.Fatalf("snapshot mutation changed regulator state: %v", got)
	}

	target, err := New(DefaultPolicy(), events)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.RestoreState(copyForRestore); err != nil {
		t.Fatal(err)
	}
	if got := target.SnapshotState().Conditions[0].Value; got != 0.25 {
		t.Fatalf("regulator condition was not restored: %v", got)
	}
}

func mustEventStore(t *testing.T) *cognition.MemoryStore {
	t.Helper()
	store, err := cognition.NewMemoryStore(16)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
