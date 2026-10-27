package cognition

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSpineSnapshotRestoresIndependentBranchLineage(t *testing.T) {
	ctx := context.Background()
	parentStore, _ := NewMemoryStore(32)
	parent, _ := NewSpine("org-snapshot", parentStore)
	first, err := parent.EmitJSON(ctx, Draft{Kind: KindPerception}, map[string]int{"tick": 1})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := parent.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	original, err := parent.Read(ctx, 0, 1)
	if err != nil || !json.Valid(original[0].Payload) {
		t.Fatalf("snapshot history is invalid: %v", err)
	}

	makeBranch := func(id string) *Spine {
		t.Helper()
		store, err := NewMemoryStore(32)
		if err != nil {
			t.Fatal(err)
		}
		spine, err := NewSpine("org-snapshot", store)
		if err != nil {
			t.Fatal(err)
		}
		if err := spine.Restore(ctx, snapshot, id); err != nil {
			t.Fatal(err)
		}
		return spine
	}
	branchA, branchB := makeBranch("left"), makeBranch("right")
	snapshot.Events[0].Payload[0] = ' '
	original, err = parent.Read(ctx, 0, 1)
	if err != nil || !json.Valid(original[0].Payload) {
		t.Fatalf("snapshot mutation escaped into parent event store: %v", err)
	}
	nextA, err := branchA.EmitJSON(ctx, Draft{Kind: KindOutcome, ParentEventIDs: []string{first.EventID}}, map[string]int{"branch": 1})
	if err != nil {
		t.Fatal(err)
	}
	nextB, err := branchB.EmitJSON(ctx, Draft{Kind: KindOutcome, ParentEventIDs: []string{first.EventID}}, map[string]int{"branch": 1})
	if err != nil {
		t.Fatal(err)
	}
	if nextA.EventID == nextB.EventID {
		t.Fatal("forks reused a continuation event identity")
	}
	if nextA.Sequence != nextB.Sequence || nextA.ParentEventIDs[0] != nextB.ParentEventIDs[0] {
		t.Fatal("forks did not preserve common causal history")
	}
	if nextA.BranchID != "left" || nextB.BranchID != "right" {
		t.Fatalf("branch labels missing: %q and %q", nextA.BranchID, nextB.BranchID)
	}
	stateA, _ := branchA.Snapshot(ctx)
	stateB, _ := branchB.Snapshot(ctx)
	if len(stateA.Lineage) != 1 || len(stateB.Lineage) != 1 ||
		stateA.Lineage[0].ForkEventID != first.EventID || stateB.Lineage[0].ForkEventID != first.EventID {
		t.Fatal("fork lineage does not point to the common checkpoint")
	}
}

func TestSpineRestoreRejectsMismatchedIdentityAndIncompleteHistory(t *testing.T) {
	ctx := context.Background()
	sourceStore, _ := NewMemoryStore(8)
	source, _ := NewSpine("source", sourceStore)
	_, _ = source.Emit(ctx, Draft{Kind: KindPerception})
	snapshot, _ := source.Snapshot(ctx)

	targetStore, _ := NewMemoryStore(8)
	target, _ := NewSpine("other", targetStore)
	if err := target.Restore(ctx, snapshot, "branch"); err == nil {
		t.Fatal("expected identity mismatch rejection")
	}

	targetStore, _ = NewMemoryStore(8)
	target, _ = NewSpine("source", targetStore)
	snapshot.Events = nil
	if err := target.Restore(ctx, snapshot, "branch"); err == nil {
		t.Fatal("expected missing history rejection")
	}
}
