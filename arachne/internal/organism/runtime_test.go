package organism

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
	"github.com/haha-systems/arachne2/internal/workspace"
)

type fakeActuator struct {
	calls  int
	action agent.RequestedAction
	result json.RawMessage
	err    error
}

type cancellingGate struct {
	cancel context.CancelFunc
}

func (g cancellingGate) Evaluate(context.Context, Commitment) (EligibilityResult, error) {
	g.cancel()
	return EligibilityResult{Eligible: true, Reason: "fixture"}, nil
}

func (a *fakeActuator) Execute(_ context.Context, action agent.RequestedAction) (json.RawMessage, error) {
	a.calls++
	a.action = action
	return append(json.RawMessage(nil), a.result...), a.err
}

func fixtureSpecialist(action agent.RequestedAction, confidence float64) agent.Specialist {
	return agent.SpecialistFunc(func(_ context.Context, _ agent.Activation) (agent.Proposal, error) {
		return agent.Proposal{
			Summary: "fixture proposal", Confidence: confidence,
			RequestedActions: []agent.RequestedAction{action},
		}, nil
	})
}

func newFixtureRuntime(t *testing.T, specialists []RegisteredSpecialist, gate EligibilityGate, actuator *fakeActuator) (*Runtime, *cognition.MemoryStore) {
	t.Helper()
	store, err := cognition.NewMemoryStore(512)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := cognition.NewSpine("fixture", store)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(Config{
		OrganismID: "fixture", Events: spine, Specialists: specialists,
		Workspace: workspace.DefaultConfig(), Gate: gate, Actuator: actuator,
	})
	if err != nil {
		t.Fatal(err)
	}
	return runtime, store
}

func fixturePerception() Perception {
	return Perception{Input: json.RawMessage(`{"stimulus":"hello"}`)}
}

func TestPersistentRuntimeStepsRecordReconstructableCausalChain(t *testing.T) {
	actuator := &fakeActuator{result: json.RawMessage(`{"observed":"done"}`)}
	runtime, store := newFixtureRuntime(t, []RegisteredSpecialist{
		{ID: "zeta", Implementation: fixtureSpecialist(agent.RequestedAction{Kind: "write", Payload: json.RawMessage(`{"scope":"fixture"}`)}, 0.9)},
		{ID: "alpha", Implementation: fixtureSpecialist(agent.RequestedAction{Kind: "write", Payload: json.RawMessage(`{"scope":"fixture"}`)}, 0.4)},
	}, FixtureGate{AllowedScopes: []string{"fixture"}}, actuator)

	first, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if first.InteractionID == second.InteractionID {
		t.Fatal("persistent steps reused an interaction ID")
	}
	if len(first.Proposals) != 2 || first.Proposals[0].SpecialistID != "zeta" || first.Proposals[1].SpecialistID != "alpha" {
		t.Fatalf("specialist attribution was not preserved: %#v", first.Proposals)
	}
	if first.Commitment.SpecialistID != "alpha" {
		t.Fatalf("deterministic commitment chose %q, want alpha", first.Commitment.SpecialistID)
	}
	if actuator.calls != 2 || string(first.Outcome.Result) != `{"observed":"done"}` {
		t.Fatalf("unexpected actuator results: calls=%d outcome=%s", actuator.calls, first.Outcome.Result)
	}

	events, err := store.Read(context.Background(), 0, 512)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]cognition.Event, len(events))
	for _, event := range events {
		byID[event.EventID] = event
	}
	assertParent := func(child, parent string) {
		t.Helper()
		for _, id := range byID[child].ParentEventIDs {
			if id == parent {
				return
			}
		}
		t.Fatalf("event %s does not link to parent %s", child, parent)
	}
	assertParent(first.Events.Activations[0], first.Events.Perception)
	assertParent(first.Events.Proposals[0], first.Events.Activations[0])
	assertParent(first.Events.Selection, first.Events.Proposals[0])
	assertParent(first.Events.Commitment, first.Events.Proposals[1])
	assertParent(first.Events.Action, first.Events.Commitment)
	assertParent(first.Events.Outcome, first.Events.Action)
	if byID[first.Events.Outcome].Kind != cognition.KindOutcome {
		t.Fatalf("outcome event kind = %q", byID[first.Events.Outcome].Kind)
	}
}

func TestActionOrderingIsDeterministicWithinSelectedProposal(t *testing.T) {
	multi := agent.SpecialistFunc(func(_ context.Context, _ agent.Activation) (agent.Proposal, error) {
		return agent.Proposal{Summary: "two actions", Confidence: 1, RequestedActions: []agent.RequestedAction{
			{Kind: "zeta", Payload: json.RawMessage(`{ "b": 2, "a": 1 }`)},
			{Kind: "alpha", Payload: json.RawMessage(`{"scope":"fixture"}`)},
		}}, nil
	})
	actuator := &fakeActuator{result: json.RawMessage(`null`)}
	runtime, _ := newFixtureRuntime(t, []RegisteredSpecialist{{ID: "only", Implementation: multi}}, FixtureGate{AllowedScopes: []string{"fixture"}}, actuator)
	result, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if result.Commitment.Action.Kind != "alpha" || actuator.action.Kind != "alpha" {
		t.Fatalf("commitment did not use deterministic action ordering: %#v", result.Commitment)
	}
}

func TestNoValidActionRecordsNoOpWithoutCallingActuator(t *testing.T) {
	actuator := &fakeActuator{}
	runtime, _ := newFixtureRuntime(t, []RegisteredSpecialist{{
		ID: "bad", Implementation: fixtureSpecialist(agent.RequestedAction{Kind: "write", Payload: json.RawMessage("not-json")}, 1),
	}}, FixtureGate{AllowedScopes: []string{"fixture"}}, actuator)
	result, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Commitment.NoOp || result.Outcome.Status != "no_op" || actuator.calls != 0 {
		t.Fatalf("invalid action was not turned into a no-op: %#v calls=%d", result, actuator.calls)
	}
	if len(result.Proposals) != 1 || result.Proposals[0].Status != "failed" {
		t.Fatalf("malformed action was not rejected by specialist validation: %#v", result.Proposals)
	}
}

func TestIneligibleActionIsRecordedAndNotExecuted(t *testing.T) {
	actuator := &fakeActuator{}
	runtime, _ := newFixtureRuntime(t, []RegisteredSpecialist{{
		ID: "writer", Implementation: fixtureSpecialist(agent.RequestedAction{Kind: "write", Payload: json.RawMessage(`{"scope":"outside"}`)}, 1),
	}}, FixtureGate{AllowedScopes: []string{"fixture"}}, actuator)
	result, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligibility.Eligible || result.Outcome.Status != "ineligible" || actuator.calls != 0 {
		t.Fatalf("ineligible action crossed the gate: %#v calls=%d", result, actuator.calls)
	}
}

func TestActuatorFailureRecordsOutcomeAndReturnsError(t *testing.T) {
	actuator := &fakeActuator{err: errors.New("fixture actuator failed")}
	runtime, store := newFixtureRuntime(t, []RegisteredSpecialist{{
		ID: "writer", Implementation: fixtureSpecialist(agent.RequestedAction{Kind: "write", Payload: json.RawMessage(`{"scope":"fixture"}`)}, 1),
	}}, FixtureGate{AllowedScopes: []string{"fixture"}}, actuator)
	result, err := runtime.Step(context.Background(), fixturePerception())
	if err == nil || result.Outcome == nil || result.Outcome.Status != "failed" || result.Events.Outcome == "" {
		t.Fatalf("actuator failure was not recorded: result=%#v err=%v", result, err)
	}
	events, readErr := store.Read(context.Background(), 0, 512)
	if readErr != nil {
		t.Fatal(readErr)
	}
	found := false
	for _, event := range events {
		if event.EventID == result.Events.Outcome && event.Kind == cognition.KindOutcome {
			found = true
		}
	}
	if !found {
		t.Fatal("failed actuator outcome is missing from the event spine")
	}
}

func TestGovernanceGateKeepsPendingActionIneligible(t *testing.T) {
	store, err := cognition.NewMemoryStore(512)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := cognition.NewSpine("fixture", store)
	if err != nil {
		t.Fatal(err)
	}
	service, err := governance.New(governance.Policy{
		ID: "fixture-policy", Version: "1", Approvers: []string{"reviewer"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionExternalEffect: {
				MinimumApprovals: 1, AllowedTargets: []string{"fixture"},
			},
		},
	}, spine)
	if err != nil {
		t.Fatal(err)
	}
	actuator := &fakeActuator{}
	runtime, err := New(Config{
		OrganismID: "fixture", Events: spine, Workspace: workspace.DefaultConfig(),
		Specialists: []RegisteredSpecialist{{
			ID: "writer", Implementation: fixtureSpecialist(agent.RequestedAction{
				Kind: "write", Payload: json.RawMessage(`{"target":"fixture"}`),
			}, 1),
		}},
		Gate: GovernanceGate{
			Service:       service,
			ClassByAction: map[string]governance.ActionClass{"write": governance.ActionExternalEffect},
		},
		Actuator: actuator,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligibility.Eligible || result.Eligibility.GovernanceDecisionID == "" || actuator.calls != 0 {
		t.Fatalf("pending governance decision reached actuator: eligibility=%#v calls=%d", result.Eligibility, actuator.calls)
	}
	events, err := store.Read(context.Background(), 0, 512)
	if err != nil {
		t.Fatal(err)
	}
	var eligibilityEvent cognition.Event
	for _, event := range events {
		if event.EventID == result.Events.Eligibility {
			eligibilityEvent = event
			break
		}
	}
	linkedDecision := false
	for _, parentID := range eligibilityEvent.ParentEventIDs {
		if parentID == result.Eligibility.GovernanceDecisionID {
			linkedDecision = true
		}
	}
	if !linkedDecision {
		t.Fatal("eligibility event does not link to its governance decision")
	}
}

func TestCancellationAfterCommitmentPreservesPartialAuditTrail(t *testing.T) {
	actuator := &fakeActuator{}
	ctx, cancel := context.WithCancel(context.Background())
	runtime, store := newFixtureRuntime(t, []RegisteredSpecialist{{
		ID: "writer", Implementation: fixtureSpecialist(agent.RequestedAction{
			Kind: "write", Payload: json.RawMessage(`{"scope":"fixture"}`),
		}, 1),
	}}, cancellingGate{cancel: cancel}, actuator)
	result, err := runtime.Step(ctx, fixturePerception())
	if !errors.Is(err, context.Canceled) || result.Events.Commitment == "" || result.Events.Action != "" || actuator.calls != 0 {
		t.Fatalf("cancellation crossed the eligibility boundary: result=%#v err=%v calls=%d", result, err, actuator.calls)
	}
	events, err := store.Read(context.Background(), 0, 512)
	if err != nil {
		t.Fatal(err)
	}
	foundCommitment := false
	for _, event := range events {
		if event.EventID == result.Events.Commitment && event.Kind == cognition.KindDecision {
			foundCommitment = true
		}
	}
	if !foundCommitment {
		t.Fatal("commitment event was lost after a later stage was cancelled")
	}
}

func TestCancelledStepReturnsWithoutDispatch(t *testing.T) {
	actuator := &fakeActuator{}
	runtime, _ := newFixtureRuntime(t, []RegisteredSpecialist{{
		ID: "writer", Implementation: fixtureSpecialist(agent.RequestedAction{Kind: "write", Payload: json.RawMessage(`{"scope":"fixture"}`)}, 1),
	}}, FixtureGate{AllowedScopes: []string{"fixture"}}, actuator)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runtime.Step(ctx, fixturePerception()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Step error = %v, want context canceled", err)
	}
	if actuator.calls != 0 {
		t.Fatalf("cancelled step dispatched %d actions", actuator.calls)
	}
}
