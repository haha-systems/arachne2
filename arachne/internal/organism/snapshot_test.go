package organism

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/workspace"
)

type snapshotGate struct{}

func (snapshotGate) Evaluate(context.Context, Commitment) (EligibilityResult, error) {
	return EligibilityResult{Eligible: true, Reason: "fixture allowed"}, nil
}

type snapshotActuator struct{}

func (snapshotActuator) Execute(context.Context, agent.RequestedAction) (json.RawMessage, error) {
	return json.RawMessage(`{"applied":true}`), nil
}
func (snapshotActuator) ExecuteDetailed(context.Context, agent.RequestedAction) (ActuatorResult, error) {
	return ActuatorResult{
		Result:           json.RawMessage(`{"applied":true}`),
		ExpectedEvidence: json.RawMessage(`{"state":"ready"}`),
		ObservedEvidence: json.RawMessage(`{"state":"ready"}`),
		ConditionDeltas:  map[string]float64{"internal.energy": 0.2},
	}, nil
}

func newSnapshotRuntime(t *testing.T, organismID string) (*Runtime, *agent.LearnableSpecialist, *memory.Service) {
	t.Helper()
	ctx := context.Background()
	store, err := cognition.NewMemoryStore(4096)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine(organismID, store)
	if err != nil {
		t.Fatal(err)
	}
	memoryStore, err := memory.NewMemoryStore(organismID)
	if err != nil {
		t.Fatal(err)
	}
	memories, err := memory.NewService(organismID, memoryStore, events)
	if err != nil {
		t.Fatal(err)
	}
	regulator, err := regulation.New(regulation.DefaultPolicy(), events)
	if err != nil {
		t.Fatal(err)
	}
	model, err := agent.NewLearnableSpecialist(agent.LearnableSpecialistConfig{
		ID: "decision", Seed: 19, FeatureDimension: 3,
		Actions: []agent.ActionCapability{{Kind: "wait"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(Config{
		OrganismID: organismID, Events: events,
		Specialists: []RegisteredSpecialist{{ID: "decision", Implementation: model}},
		Workspace:   workspace.DefaultConfig(), Gate: snapshotGate{}, Actuator: snapshotActuator{},
		Memory: memories, Regulator: regulator, Learners: map[string]OutcomeLearner{"decision": model},
		Scorer: func(context.Context, StepResult) (map[string]float64, error) {
			return map[string]float64{"wait": 0.7}, nil
		},
		ReplaySelector: func(_ context.Context, episodes []memory.Episode) ([]string, error) {
			if len(episodes) == 0 {
				return nil, nil
			}
			return []string{episodes[0].ID}, nil
		},
		ComponentRestorability: map[string]Restorability{
			"eligibility-gate": RestorabilityStateless,
			"actuator":         RestorabilityStateless,
			"outcome-scorer":   RestorabilityStateless,
			"replay-selector":  RestorabilityStateless,
		},
		Deterministic: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = ctx
	return runtime, model, memories
}

func snapshotStep(t *testing.T, runtime *Runtime, interaction string, features []float64) StepResult {
	t.Helper()
	result, err := runtime.Step(context.Background(), Perception{
		InteractionID: interaction, SessionID: "snapshot-session",
		Input: json.RawMessage(`{"observation":"same"}`), Features: features,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome == nil {
		t.Fatal("synchronous fixture action did not report an outcome")
	}
	return result
}

func TestRuntimeSnapshotRestoresDeterministicContinuationAndIndependentForks(t *testing.T) {
	ctx := context.Background()
	source, sourceModel, sourceMemory := newSnapshotRuntime(t, "branching-organism")
	snapshotStep(t, source, "", []float64{0.2, 0.4, 0.6})
	snapshotStep(t, source, "", []float64{0.3, 0.5, 0.7})
	raw, err := source.SnapshotState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	baseline, err := source.Step(ctx, Perception{
		SessionID: "snapshot-session", Input: json.RawMessage(`{"observation":"same"}`),
		Features: []float64{0.4, 0.6, 0.8},
	})
	if err != nil {
		t.Fatal(err)
	}

	left, leftModel, leftMemory := newSnapshotRuntime(t, "branching-organism")
	if err := left.RestoreState(ctx, raw, "left"); err != nil {
		t.Fatal(err)
	}
	right, rightModel, rightMemory := newSnapshotRuntime(t, "branching-organism")
	if err := right.RestoreState(ctx, raw, "right"); err != nil {
		t.Fatal(err)
	}
	leftResult, err := left.Step(ctx, Perception{SessionID: "snapshot-session", Input: json.RawMessage(`{"observation":"same"}`), Features: []float64{0.4, 0.6, 0.8}})
	if err != nil {
		t.Fatal(err)
	}
	rightResult, err := right.Step(ctx, Perception{SessionID: "snapshot-session", Input: json.RawMessage(`{"observation":"same"}`), Features: []float64{0.4, 0.6, 0.8}})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []StepResult{baseline, leftResult, rightResult} {
		if len(result.Events.Replay) != 2 {
			t.Fatalf("completed replay state was not resumed: %v", result.Events.Replay)
		}
		if result.Commitment.Action.Kind != "wait" || result.Outcome == nil || result.Outcome.Status != "executed" {
			t.Fatalf("unexpected continuation: %#v", result.Commitment)
		}
	}
	if baseline.Proposals[0].ID != leftResult.Proposals[0].ID || leftResult.Proposals[0].ID != rightResult.Proposals[0].ID {
		t.Fatal("branch label changed branch-independent proposal identity")
	}
	if leftResult.Events.Outcome == rightResult.Events.Outcome {
		t.Fatal("branches reused an event identity")
	}
	leftModelState, _ := json.Marshal(leftModel.Snapshot())
	rightModelState, _ := json.Marshal(rightModel.Snapshot())
	sourceModelState, _ := json.Marshal(sourceModel.Snapshot())
	if !bytes.Equal(leftModelState, rightModelState) || !bytes.Equal(sourceModelState, leftModelState) {
		t.Fatal("deterministic model parameters diverged across identical continuations")
	}
	if left.Conditions()["internal.energy"] != right.Conditions()["internal.energy"] {
		t.Fatal("regulation condition state diverged across identical continuations")
	}
	leftEpisodes, _ := leftMemory.RetrieveEpisodes(ctx, memory.EpisodeQuery{SessionID: "snapshot-session", Limit: 10})
	rightEpisodes, _ := rightMemory.RetrieveEpisodes(ctx, memory.EpisodeQuery{SessionID: "snapshot-session", Limit: 10})
	if len(leftEpisodes) != 3 || len(rightEpisodes) != 3 {
		t.Fatalf("memory continuity lost: %d / %d", len(leftEpisodes), len(rightEpisodes))
	}

	beforeRight := rightModel.Snapshot()
	if _, err := leftModel.Update([]float64{0.4, 0.6, 0.8}, "wait", 0.1); err != nil {
		t.Fatal(err)
	}
	afterRight := rightModel.Snapshot()
	beforeJSON, _ := json.Marshal(beforeRight)
	afterJSON, _ := json.Marshal(afterRight)
	if !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("forks share learnable model memory")
	}

	if _, err := leftMemory.RecordEpisode(ctx, memory.Episode{
		ID: "branch-only", SessionID: "snapshot-session", Kind: "branch_intervention",
		Source: memory.SourceRef{Kind: "test", ID: "branch-left"}, OccurredAt: time.Now().UTC(),
		RecordedAt: time.Now().UTC(), Content: json.RawMessage(`{"branch":"left"}`),
	}); err != nil {
		t.Fatal(err)
	}
	rightEpisodes, _ = rightMemory.RetrieveEpisodes(ctx, memory.EpisodeQuery{SessionID: "snapshot-session", Limit: 10})
	if len(rightEpisodes) != 3 {
		t.Fatal("forks share mutable memory store")
	}
	if _, err := sourceMemory.RetrieveEpisodes(ctx, memory.EpisodeQuery{SessionID: "snapshot-session", Limit: 10}); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeSnapshotControlledObservationInterventionCausesAttributedDivergence(t *testing.T) {
	ctx := context.Background()
	source := newInterventionRuntime(t, "intervention-organism")
	finishInterventionStep(t, source, []float64{0.1})
	finishInterventionStep(t, source, []float64{0.2})
	raw, err := source.SnapshotState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	baseline := finishInterventionStep(t, source, []float64{0.3})
	left := newInterventionRuntime(t, "intervention-organism")
	if err := left.RestoreState(ctx, raw, "baseline"); err != nil {
		t.Fatal(err)
	}
	leftResult := finishInterventionStep(t, left, []float64{0.3})

	right := newInterventionRuntime(t, "intervention-organism")
	if err := right.RestoreState(ctx, raw, "intervened"); err != nil {
		t.Fatal(err)
	}
	// The intervention changes only one explicit feature in the branch perception.
	rightResult := finishInterventionStep(t, right, []float64{0.9})
	if baseline.Commitment.Action.Kind != "wait" || leftResult.Commitment.Action.Kind != "wait" {
		t.Fatalf("identical continuation did not reproduce baseline: %q / %q",
			baseline.Commitment.Action.Kind, leftResult.Commitment.Action.Kind)
	}
	if rightResult.Commitment.Action.Kind != "move" {
		t.Fatalf("controlled feature intervention did not alter the committed action: %q", rightResult.Commitment.Action.Kind)
	}
	if rightResult.Events.Outcome == leftResult.Events.Outcome {
		t.Fatal("intervened and baseline branches reused event identity")
	}
}

type interventionSpecialist struct{}

func (interventionSpecialist) Stateless() bool { return true }

func (interventionSpecialist) Propose(_ context.Context, activation agent.Activation) (agent.Proposal, error) {
	kind := "wait"
	payload := json.RawMessage(`{}`)
	if len(activation.Features) > 0 && activation.Features[0] >= 0.5 {
		kind = "move"
		payload = json.RawMessage(`{"dx":1,"dy":0}`)
	}
	return agent.Proposal{
		Summary: "deterministic feature response", Confidence: 1,
		RequestedActions: []agent.RequestedAction{{Kind: kind, Payload: payload}},
	}, nil
}

func newInterventionRuntime(t *testing.T, organismID string) *Runtime {
	t.Helper()
	eventStore, err := cognition.NewMemoryStore(128)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine(organismID, eventStore)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(Config{
		OrganismID:  organismID,
		Events:      events,
		Specialists: []RegisteredSpecialist{{ID: "fixture", Implementation: interventionSpecialist{}}},
		Workspace:   workspace.DefaultConfig(),
		Gate:        snapshotGate{},
		ComponentRestorability: map[string]Restorability{
			"eligibility-gate": RestorabilityStateless,
		},
		DeferredExecution: true,
		Deterministic:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func finishInterventionStep(t *testing.T, runtime *Runtime, features []float64) StepResult {
	t.Helper()
	result, err := runtime.Step(context.Background(), Perception{
		SessionID: "intervention-test", Input: json.RawMessage(`{"source":"controlled"}`),
		Features: features,
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := runtime.CompleteOutcome(context.Background(), result.InteractionID, Outcome{Status: "executed"})
	if err != nil {
		t.Fatal(err)
	}
	return completed
}

func TestRuntimeSnapshotRejectsCorruptionIdentityAndMissingState(t *testing.T) {
	ctx := context.Background()
	source, _, _ := newSnapshotRuntime(t, "snapshot-organism")
	snapshotStep(t, source, "", []float64{0.1, 0.2, 0.3})
	raw, err := source.SnapshotState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	target, _, _ := newSnapshotRuntime(t, "snapshot-organism")
	corrupt := append(json.RawMessage(nil), raw...)
	corrupt[len(corrupt)-2] ^= 1
	if err := target.RestoreState(ctx, corrupt, "bad"); err == nil {
		t.Fatal("expected checksum rejection")
	}

	other, _, _ := newSnapshotRuntime(t, "other-organism")
	if err := other.RestoreState(ctx, raw, "bad"); err == nil {
		t.Fatal("expected organism identity rejection")
	}

	var envelope runtimeSnapshotEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.SchemaVersion = "arachne.organism_runtime.v999"
	incompatible, _ := json.Marshal(envelope)
	if err := target.RestoreState(ctx, incompatible, "bad"); err == nil {
		t.Fatal("expected schema rejection")
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.State.Memory = nil
	stateBytes, _ := json.Marshal(envelope.State)
	checksum := sha256.Sum256(stateBytes)
	envelope.SHA256 = hex.EncodeToString(checksum[:])
	missing, _ := json.Marshal(envelope)
	target, _, _ = newSnapshotRuntime(t, "snapshot-organism")
	if err := target.RestoreState(ctx, missing, "bad"); err == nil {
		t.Fatal("expected missing required memory rejection")
	}
}

func TestRuntimeSnapshotReportsOpaqueComponentsAndMarksBestEffort(t *testing.T) {
	runtime, _, _ := newSnapshotRuntime(t, "opaque-report-test")
	runtime.componentRestorability = nil
	if _, err := runtime.SnapshotState(context.Background()); err == nil {
		t.Fatal("exact snapshot accepted a gate without an explicit state contract")
	}
	bestEffort, err := runtime.SnapshotBestEffortState(context.Background())
	if err != nil {
		t.Fatalf("best-effort snapshot should preserve available state: %v", err)
	}
	var envelope runtimeSnapshotEnvelope
	if err := json.Unmarshal(bestEffort, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.State.Mode != SnapshotBestEffort || envelope.State.Restorability.Exact {
		t.Fatalf("best-effort snapshot claimed exact restoration: mode=%q exact=%v", envelope.State.Mode, envelope.State.Restorability.Exact)
	}
	var found bool
	for _, component := range envelope.State.Restorability.Components {
		if component.ID == "eligibility-gate" && component.Restorability == RestorabilityOpaque && component.Reason != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("best-effort metadata did not identify the opaque eligibility gate")
	}
	fresh, _, _ := newSnapshotRuntime(t, "opaque-report-test")
	if err := fresh.RestoreState(context.Background(), bestEffort, "branch"); err == nil {
		t.Fatal("exact restore API accepted a best-effort snapshot")
	}
}

type opaqueStateSpecialist struct{}

func (opaqueStateSpecialist) Propose(context.Context, agent.Activation) (agent.Proposal, error) {
	return agent.Proposal{Summary: "opaque fixture"}, nil
}

func TestRuntimeRejectsOpaqueSpecialistForExactSnapshot(t *testing.T) {
	runtime, _, _ := newSnapshotRuntime(t, "silk-opaque-test")
	specialist, err := agent.NewSpecialistAgent("silk-backed", opaqueStateSpecialist{}, runtime.events)
	if err != nil {
		t.Fatal(err)
	}
	runtime.specialists = []*agent.SpecialistAgent{specialist}
	if _, err := runtime.SnapshotState(context.Background()); err == nil || !strings.Contains(err.Error(), "silk-backed") {
		t.Fatalf("exact snapshot did not name the opaque specialist: %v", err)
	}
	status := runtime.Determinism()
	if status.Deterministic || len(status.Reasons) == 0 {
		t.Fatal("runtime claimed deterministic continuation with opaque specialist state")
	}
	bestEffort, err := runtime.SnapshotBestEffortState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var envelope runtimeSnapshotEnvelope
	if err := json.Unmarshal(bestEffort, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.State.Restorability.Exact {
		t.Fatal("best-effort snapshot claimed exact restoration")
	}
}

func TestRuntimeSnapshotRejectsPendingDeferredOutcome(t *testing.T) {
	runtime, _, _ := newSnapshotRuntime(t, "pending-organism")
	runtime.mu.Lock()
	runtime.deferredExecution = true
	runtime.actuator = nil
	runtime.mu.Unlock()
	result, err := runtime.Step(context.Background(), Perception{
		Input: json.RawMessage(`{}`), Features: []float64{0.1, 0.2, 0.3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != nil {
		t.Fatal("deferred runtime reported an outcome before the environment")
	}
	if _, err := runtime.SnapshotState(context.Background()); err == nil {
		t.Fatal("expected pending outcome to block checkpoint")
	}
	completed, err := runtime.CompleteOutcome(context.Background(), result.InteractionID, Outcome{Status: "executed"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Outcome == nil {
		t.Fatal("deferred outcome was not recorded")
	}
	if _, err := runtime.SnapshotState(context.Background()); err != nil {
		t.Fatalf("completed runtime could not be checkpointed: %v", err)
	}
}
