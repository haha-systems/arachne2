package organism

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/workspace"
)

type detailedFeedbackActuator struct {
	result ActuatorResult
	err    error
	calls  int
}

func (a *detailedFeedbackActuator) Execute(context.Context, agent.RequestedAction) (json.RawMessage, error) {
	panic("detailed path should be used")
}

func (a *detailedFeedbackActuator) ExecuteDetailed(context.Context, agent.RequestedAction) (ActuatorResult, error) {
	a.calls++
	return a.result, a.err
}

type feedbackLearner struct {
	scores []float64
}

func (l *feedbackLearner) UpdateOutcome(_ context.Context, _ []float64, _ string, score float64) (json.RawMessage, error) {
	l.scores = append(l.scores, score)
	return json.Marshal(map[string]float64{"score": score})
}

type adaptiveFeedbackSpecialist struct {
	score float64
}

func (s *adaptiveFeedbackSpecialist) Propose(_ context.Context, _ agent.Activation) (agent.Proposal, error) {
	return agent.Proposal{
		Summary: "adaptive fixture", Confidence: s.score,
		RequestedActions: []agent.RequestedAction{{Kind: "adjust", Payload: json.RawMessage(`{"scope":"safe"}`)}},
	}, nil
}

func (s *adaptiveFeedbackSpecialist) UpdateOutcome(_ context.Context, _ []float64, _ string, score float64) (json.RawMessage, error) {
	s.score = score
	return json.Marshal(map[string]float64{"score": score})
}

func newFeedbackRuntime(t *testing.T, specialist agent.Specialist, actuator Actuator, score OutcomeScorer, learner OutcomeLearner, replay ReplaySelector) (*Runtime, *cognition.MemoryStore, *memory.Service) {
	t.Helper()
	eventStore, err := cognition.NewMemoryStore(1024)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := cognition.NewSpine("feedback-fixture", eventStore)
	if err != nil {
		t.Fatal(err)
	}
	memoryStore, err := memory.NewMemoryStore("feedback-fixture")
	if err != nil {
		t.Fatal(err)
	}
	memoryService, err := memory.NewService("feedback-fixture", memoryStore, spine)
	if err != nil {
		t.Fatal(err)
	}
	regulator, err := regulation.New(regulation.DefaultPolicy(), spine)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(Config{
		OrganismID: "feedback-fixture", Events: spine,
		Specialists: []RegisteredSpecialist{{ID: "adaptive", Implementation: specialist}},
		Workspace:   workspace.DefaultConfig(), Gate: FixtureGate{AllowedScopes: []string{"safe"}},
		Actuator: actuator, Memory: memoryService, Regulator: regulator, Scorer: score,
		Learners: map[string]OutcomeLearner{"adaptive": learner}, ReplaySelector: replay,
	})
	if err != nil {
		t.Fatal(err)
	}
	return runtime, eventStore, memoryService
}

func TestOutcomeFeedbackRecordsAttributedEpisodesAndChangesLaterActivation(t *testing.T) {
	var activations []agent.Activation
	specialist := agent.SpecialistFunc(func(_ context.Context, activation agent.Activation) (agent.Proposal, error) {
		activations = append(activations, activation)
		return agent.Proposal{Summary: "fixture", Confidence: 0.8,
			RequestedActions: []agent.RequestedAction{{Kind: "adjust", Payload: json.RawMessage(`{"scope":"safe"}`)}}}, nil
	})
	actuator := &detailedFeedbackActuator{result: ActuatorResult{
		Result: json.RawMessage(`{"done":true}`), ConditionDeltas: map[string]float64{"energy": -0.25},
	}}
	learner := &feedbackLearner{}
	runtime, eventStore, memories := newFeedbackRuntime(t, specialist, actuator,
		func(context.Context, StepResult) (map[string]float64, error) {
			return map[string]float64{"adjust": 0.75}, nil
		},
		learner,
		func(_ context.Context, episodes []memory.Episode) ([]string, error) {
			if len(episodes) == 0 {
				return nil, nil
			}
			return []string{episodes[0].ID}, nil
		})

	first, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if first.Events.Memory == "" || first.Events.Regulation == "" || first.Events.PredictionError != "" {
		t.Fatalf("unexpected feedback event IDs: %+v", first.Events)
	}
	if len(first.Events.Learning) != 1 || len(first.Events.Replay) != 2 {
		t.Fatalf("learning or replay events missing: %+v", first.Events)
	}
	if got := runtime.Conditions()["energy"]; got != -0.25 {
		t.Fatalf("condition delta not retained: %v", got)
	}
	if got := runtime.regulator.Conditions()["energy"]; got != -0.25 {
		t.Fatalf("regulator did not receive condition delta: %v", got)
	}
	if len(learner.scores) != 1 || learner.scores[0] != 0.75 {
		t.Fatalf("learner scores: %v", learner.scores)
	}
	if actuator.calls != 1 {
		t.Fatalf("replay executed actuator: calls=%d", actuator.calls)
	}

	second, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if len(activations) != 2 || len(activations[1].RecentEpisodes) != 1 {
		t.Fatalf("later activation did not receive recent episode: %+v", activations)
	}
	if got := activations[1].InternalConditions["energy"]; got != -0.25 {
		t.Fatalf("later activation did not receive copied condition state: %v", got)
	}
	episodes, err := memories.RetrieveEpisodes(context.Background(), memory.EpisodeQuery{Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 2 {
		t.Fatalf("recorded episodes: %d", len(episodes))
	}
	if episodes[0].EventID == "" {
		t.Fatal("retrieved episode lost its memory event ID")
	}
	for _, id := range []string{first.Events.Perception, first.Events.Proposals[0], first.Events.Commitment, first.Events.Action, first.Events.Outcome} {
		found := false
		for _, parent := range episodes[0].SourceEventIDs {
			if id == parent {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("episode is missing source event %q: %+v", id, episodes[0].SourceEventIDs)
		}
	}
	events, err := eventStore.Read(context.Background(), 0, 1024)
	if err != nil {
		t.Fatal(err)
	}
	predictionErrors := 0
	for _, event := range events {
		if event.Kind == cognition.KindPredictionError {
			predictionErrors++
		}
	}
	if predictionErrors != 0 {
		t.Fatalf("missing prior predictions emitted %d prediction errors", predictionErrors)
	}
	for _, event := range events {
		if event.EventID == first.Events.Memory {
			for _, required := range []string{first.Events.Perception, first.Events.Proposals[0], first.Events.Commitment, first.Events.Action, first.Events.Outcome} {
				found := false
				for _, parent := range event.ParentEventIDs {
					if parent == required {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("memory event is missing parent %q: %+v", required, event.ParentEventIDs)
				}
			}
		}
	}
	if second.Events.Memory == "" {
		t.Fatal("second episode was not recorded")
	}
}

func TestOutcomeScoresCanChangeLaterCognition(t *testing.T) {
	leftSpecialist, rightSpecialist := &adaptiveFeedbackSpecialist{}, &adaptiveFeedbackSpecialist{}
	leftActuator := &detailedFeedbackActuator{result: ActuatorResult{Result: json.RawMessage(`{"outcome":"left"}`)}}
	rightActuator := &detailedFeedbackActuator{result: ActuatorResult{Result: json.RawMessage(`{"outcome":"right"}`)}}
	left, _, _ := newFeedbackRuntime(t, leftSpecialist, leftActuator,
		func(context.Context, StepResult) (map[string]float64, error) {
			return map[string]float64{"adjust": 1}, nil
		}, leftSpecialist, nil)
	right, _, _ := newFeedbackRuntime(t, rightSpecialist, rightActuator,
		func(context.Context, StepResult) (map[string]float64, error) {
			return map[string]float64{"adjust": 0}, nil
		}, rightSpecialist, nil)
	if _, err := left.Step(context.Background(), fixturePerception()); err != nil {
		t.Fatal(err)
	}
	if _, err := right.Step(context.Background(), fixturePerception()); err != nil {
		t.Fatal(err)
	}
	leftNext, err := left.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	rightNext, err := right.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if leftNext.Proposals[0].Confidence == rightNext.Proposals[0].Confidence {
		t.Fatalf("different explicit outcome scores did not change cognition: left=%v right=%v", leftNext.Proposals[0].Confidence, rightNext.Proposals[0].Confidence)
	}
}

func TestLearnerRequiresExplicitScore(t *testing.T) {
	learner := &feedbackLearner{}
	actuator := &detailedFeedbackActuator{result: ActuatorResult{Result: json.RawMessage(`{"done":true}`)}}
	runtime, _, _ := newFeedbackRuntime(t, fixtureSpecialist(agent.RequestedAction{
		Kind: "adjust", Payload: json.RawMessage(`{"scope":"safe"}`),
	}, 0.8), actuator,
		func(context.Context, StepResult) (map[string]float64, error) { return map[string]float64{}, nil },
		learner, nil)
	result, err := runtime.Step(context.Background(), fixturePerception())
	if err != nil {
		t.Fatal(err)
	}
	if len(learner.scores) != 0 || len(result.Events.Learning) != 0 {
		t.Fatalf("learner ran without an explicit target score: scores=%v events=%v", learner.scores, result.Events.Learning)
	}
}

func TestFailedDetailedExecutionCompletesFeedbackBeforeReturningError(t *testing.T) {
	executionFailure := errors.New("actuator unavailable")
	actuator := &detailedFeedbackActuator{result: ActuatorResult{
		Result: json.RawMessage(`{"attempted":true}`), ConditionDeltas: map[string]float64{"load": 1},
	}, err: executionFailure}
	runtime, _, _ := newFeedbackRuntime(t, fixtureSpecialist(agent.RequestedAction{
		Kind: "adjust", Payload: json.RawMessage(`{"scope":"safe"}`),
	}, 0.8), actuator, nil, nil, nil)
	result, err := runtime.Step(context.Background(), fixturePerception())
	if !errors.Is(err, executionFailure) {
		t.Fatalf("execution error = %v", err)
	}
	if result.Events.Outcome == "" || result.Events.Memory == "" || result.Events.Regulation == "" {
		t.Fatalf("failure feedback incomplete: %+v", result.Events)
	}
	if runtime.Conditions()["load"] != 1 {
		t.Fatalf("failed execution condition delta not applied: %v", runtime.Conditions())
	}
}
