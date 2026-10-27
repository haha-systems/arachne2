package agent

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func testLearnableConfig(id string, seed uint64) LearnableSpecialistConfig {
	return LearnableSpecialistConfig{
		ID: id, Seed: seed, FeatureDimension: 2, MaxActions: 2,
		Actions: []ActionCapability{
			{Kind: "move", NumericParameters: []NumericParameter{{Name: "distance", Min: 0, Max: 10}}},
			{Kind: "wait", NumericParameters: []NumericParameter{{Name: "seconds", Min: 1, Max: 5}}},
			{Kind: "observe"},
		},
	}
}

func TestLearnableSpecialistSeededInitializationAndProposalAreDeterministic(t *testing.T) {
	first, err := NewLearnableSpecialist(testLearnableConfig("learner-a", 41))
	if err != nil {
		t.Fatal(err)
	}
	equal, err := NewLearnableSpecialist(testLearnableConfig("learner-a", 41))
	if err != nil {
		t.Fatal(err)
	}
	distinct, err := NewLearnableSpecialist(testLearnableConfig("learner-a", 42))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Snapshot(), equal.Snapshot()) {
		t.Fatal("equal seeds should initialize equal specialist state")
	}
	if reflect.DeepEqual(first.Snapshot().Parameters, distinct.Snapshot().Parameters) {
		t.Fatal("different seeds should initialize distinct parameters")
	}

	activation := Activation{InteractionID: "interaction-1", Input: json.RawMessage(`{"legacy":"input"}`), Features: []float64{0.25, -0.75}}
	before := first.Snapshot()
	proposalA, err := first.Propose(context.Background(), activation)
	if err != nil {
		t.Fatal(err)
	}
	proposalB, err := first.Propose(context.Background(), activation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(proposalA, proposalB) {
		t.Fatal("proposal inference should repeat for equal state and input")
	}
	if !reflect.DeepEqual(before, first.Snapshot()) {
		t.Fatal("proposal inference must not mutate parameters or RNG state")
	}
	if len(proposalA.RequestedActions) != 2 {
		t.Fatalf("expected action cap of 2, got %d", len(proposalA.RequestedActions))
	}
}

func TestLearnableSpecialistBoundsConfidenceAndFiltersCapabilities(t *testing.T) {
	specialist, err := NewLearnableSpecialist(testLearnableConfig("bounded", 7))
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := specialist.Propose(context.Background(), Activation{Features: []float64{1, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Confidence < 0 || proposal.Confidence > 1 {
		t.Fatalf("confidence outside [0, 1]: %v", proposal.Confidence)
	}
	for _, action := range proposal.RequestedActions {
		switch action.Kind {
		case "move":
			assertNumericPayloadRange(t, action.Payload, "distance", 0, 10)
		case "wait":
			assertNumericPayloadRange(t, action.Payload, "seconds", 1, 5)
		case "observe":
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(action.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 0 {
				t.Fatalf("action with no numeric fields should have an empty payload, got %s", action.Payload)
			}
		default:
			t.Fatalf("proposed undeclared action kind %q", action.Kind)
		}
	}
}

func TestLearnableSpecialistExplicitUpdateAndIndependentState(t *testing.T) {
	first, err := NewLearnableSpecialist(testLearnableConfig("learner-a", 99))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewLearnableSpecialist(testLearnableConfig("learner-b", 99))
	if err != nil {
		t.Fatal(err)
	}
	beforeFirst := first.Snapshot()
	beforeSecond := second.Snapshot()

	update, err := first.Update([]float64{0.5, -0.25}, "move", 1)
	if err != nil {
		t.Fatal(err)
	}
	afterFirst := first.Snapshot()
	afterSecond := second.Snapshot()
	if update.Index != 1 || update.ActionKind != "move" || update.TargetScore != 1 {
		t.Fatalf("unexpected update metadata: %#v", update)
	}
	if reflect.DeepEqual(beforeFirst.Parameters, afterFirst.Parameters) {
		t.Fatal("target-score update should change the selected action's parameters")
	}
	if !reflect.DeepEqual(beforeSecond, afterSecond) {
		t.Fatal("updating one specialist must not mutate another specialist")
	}
	if afterFirst.UpdateCount != 1 || afterFirst.LastUpdate == nil {
		t.Fatalf("snapshot missing update metadata: %#v", afterFirst)
	}
	if _, err := first.Update([]float64{0.5, -0.25}, "undeclared", 0.5); err == nil {
		t.Fatal("update should reject undeclared action kinds")
	}
	if _, err := first.Update([]float64{0.5, -0.25}, "move", 1.1); err == nil {
		t.Fatal("update should reject targets outside [0, 1]")
	}
}

func TestLearnableSpecialistSnapshotAndActivationJSON(t *testing.T) {
	specialist, err := NewLearnableSpecialist(testLearnableConfig("serializable", 123))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := specialist.Update([]float64{0.2, 0.8}, "wait", 0.25); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(specialist.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var snapshot LearnableSpecialistSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, specialist.Snapshot()) {
		t.Fatal("snapshot should round-trip through JSON")
	}
	if snapshot.Config.Seed != 123 || snapshot.RNGState == 0 || snapshot.LastUpdate == nil {
		t.Fatalf("snapshot should expose seed, RNG state, and update metadata: %#v", snapshot)
	}

	activation := Activation{InteractionID: "i", Input: json.RawMessage(`{"kept":true}`), Features: []float64{1, 2}}
	activationJSON, err := json.Marshal(activation)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Activation
	if err := json.Unmarshal(activationJSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded.Input) != string(activation.Input) || !reflect.DeepEqual(decoded.Features, activation.Features) {
		t.Fatalf("activation JSON should preserve both legacy input and numeric features: %#v", decoded)
	}
}

func assertNumericPayloadRange(t *testing.T, payload json.RawMessage, field string, minimum, maximum float64) {
	t.Helper()
	var values map[string]float64
	if err := json.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 {
		t.Fatalf("expected only the declared numeric field %q, got %s", field, payload)
	}
	value, exists := values[field]
	if !exists {
		t.Fatalf("payload is missing declared numeric field %q: %s", field, payload)
	}
	if value < minimum || value > maximum {
		t.Fatalf("%s value %v outside [%v, %v]", field, value, minimum, maximum)
	}
}
