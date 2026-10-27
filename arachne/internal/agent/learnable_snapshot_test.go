package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestLearnableSpecialistSnapshotRestoresParametersAndRNGIndependently(t *testing.T) {
	config := LearnableSpecialistConfig{
		ID: "snapshot-model", Seed: 71, FeatureDimension: 3,
		Actions: []ActionCapability{{Kind: "observe"}, {Kind: "wait"}},
	}
	source, err := NewLearnableSpecialist(config)
	if err != nil {
		t.Fatal(err)
	}
	features := []float64{0.2, 0.7, 0.4}
	for index := 0; index < 4; index++ {
		input := []float64{features[0] + float64(index)/10, features[1], features[2]}
		if _, err := source.Propose(context.Background(), Activation{Features: input}); err != nil {
			t.Fatal(err)
		}
		if _, err := source.Update(input, "wait", 0.2+float64(index)/10); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := source.Snapshot()
	restore := source.Snapshot()
	snapshot.Parameters[0].Score.Weights[0] += 100
	if source.Snapshot().Parameters[0].Score.Weights[0] == snapshot.Parameters[0].Score.Weights[0] {
		t.Fatal("snapshot parameters alias source model state")
	}

	target, err := NewLearnableSpecialist(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := target.RestoreSnapshot(restore); err != nil {
		t.Fatal(err)
	}
	if got := target.Snapshot(); got.RNGState != restore.RNGState || got.UpdateCount != restore.UpdateCount {
		t.Fatalf("RNG or update counter was not restored: %#v", got)
	}
	futureInputs := [][]float64{
		features,
		{0.8, 0.1, 0.5},
		{0.3, 0.4, 0.9},
	}
	for index, input := range futureInputs {
		left, err := source.Propose(context.Background(), Activation{Features: input})
		if err != nil {
			t.Fatal(err)
		}
		right, err := target.Propose(context.Background(), Activation{Features: input})
		if err != nil {
			t.Fatal(err)
		}
		leftJSON, _ := json.Marshal(left)
		rightJSON, _ := json.Marshal(right)
		if !bytes.Equal(leftJSON, rightJSON) {
			t.Fatalf("restored proposal/confidence/sampling decision %d differs: %s != %s", index, leftJSON, rightJSON)
		}
		leftUpdate, err := source.UpdateOutcome(context.Background(), input, "wait", 0.3+float64(index)/10)
		if err != nil {
			t.Fatal(err)
		}
		rightUpdate, err := target.UpdateOutcome(context.Background(), input, "wait", 0.3+float64(index)/10)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(leftUpdate, rightUpdate) {
			t.Fatalf("restored update %d differs: %s != %s", index, leftUpdate, rightUpdate)
		}
	}
	leftState, _ := json.Marshal(source.Snapshot())
	rightState, _ := json.Marshal(target.Snapshot())
	if !bytes.Equal(leftState, rightState) {
		t.Fatal("restored model did not preserve future learning continuity")
	}
}

func TestSpecialistAgentSnapshotContractsNameOpaqueState(t *testing.T) {
	store, err := cognition.NewMemoryStore(64)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := cognition.NewSpine("specialist-contract-test", store)
	if err != nil {
		t.Fatal(err)
	}
	config := LearnableSpecialistConfig{
		ID: "contract-model", Seed: 87, FeatureDimension: 2,
		Actions: []ActionCapability{{Kind: "wait"}},
	}
	source, err := NewLearnableSpecialist(config)
	if err != nil {
		t.Fatal(err)
	}
	sourceAgent, err := NewSpecialistAgent("contract-model", source, spine)
	if err != nil {
		t.Fatal(err)
	}
	if status := sourceAgent.Restorability(); status.Restorability != RestorabilityExact {
		t.Fatalf("learnable specialist status = %+v", status)
	}
	snapshot, err := sourceAgent.SnapshotState()
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewLearnableSpecialist(config)
	if err != nil {
		t.Fatal(err)
	}
	targetAgent, err := NewSpecialistAgent("contract-model", target, spine)
	if err != nil {
		t.Fatal(err)
	}
	if err := targetAgent.RestoreState(snapshot); err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(source.Snapshot())
	right, _ := json.Marshal(target.Snapshot())
	if !bytes.Equal(left, right) {
		t.Fatal("specialist agent restore did not preserve model state")
	}

	opaque, err := NewSpecialistAgent("silk-provider", SpecialistFunc(func(context.Context, Activation) (Proposal, error) {
		return Proposal{}, nil
	}), spine)
	if err != nil {
		t.Fatal(err)
	}
	if status := opaque.Restorability(); status.Restorability != RestorabilityOpaque || status.ID != "silk-provider" {
		t.Fatalf("opaque specialist status = %+v", status)
	}
	if _, err := opaque.SnapshotState(); err == nil {
		t.Fatal("snapshot contract accepted an opaque specialist")
	}
}

func TestLearnableSpecialistRestoreRejectsIncompatibleOrMissingState(t *testing.T) {
	config := LearnableSpecialistConfig{
		ID: "snapshot-model", Seed: 71, FeatureDimension: 2,
		Actions: []ActionCapability{{Kind: "wait"}},
	}
	source, _ := NewLearnableSpecialist(config)
	snapshot := source.Snapshot()
	target, _ := NewLearnableSpecialist(config)
	snapshot.Parameters = nil
	if err := target.RestoreSnapshot(snapshot); err == nil {
		t.Fatal("expected missing model state rejection")
	}
	config.ID = "different-model"
	other, _ := NewLearnableSpecialist(config)
	if err := other.RestoreSnapshot(source.Snapshot()); err == nil {
		t.Fatal("expected specialist identity mismatch rejection")
	}
}
