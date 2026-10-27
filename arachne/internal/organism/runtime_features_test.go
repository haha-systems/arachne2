package organism

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/haha-systems/arachne2/internal/agent"
)

func TestRuntimePassesNumericFeaturesToSpecialists(t *testing.T) {
	expected := []float64{0.25, -0.5, 1}
	var observed []float64
	specialist := agent.SpecialistFunc(func(_ context.Context, activation agent.Activation) (agent.Proposal, error) {
		observed = append([]float64(nil), activation.Features...)
		return agent.Proposal{Summary: "features received", Confidence: 0.5}, nil
	})
	runtime, _ := newFixtureRuntime(t, []RegisteredSpecialist{{
		ID: "feature-reader", Implementation: specialist,
	}}, FixtureGate{}, &fakeActuator{})

	_, err := runtime.Step(context.Background(), Perception{
		Input: json.RawMessage(`{"legacy":"input"}`), Features: expected,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(observed, expected) {
		t.Fatalf("specialist received features %v, want %v", observed, expected)
	}
}
