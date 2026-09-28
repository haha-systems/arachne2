// Package main compares isolated signal changes in workspace selection.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/workspace"
)

type result struct {
	Specialists       []string
	Capacity          int
	EvidenceRequired  bool
	PredictionError   float64
	SelectionEventID  string
	RegulationEventID string
}

type scenario struct {
	Name              string
	Observed          string
	Salience          float64
	ActiveSpecialists int
	PendingActions    int
	Capacity          int
	RequireEvidence   bool
	Selected          []string
}

func main() {
	ctx := context.Background()
	scenarios := []scenario{
		{Name: "baseline", Observed: "same", Capacity: 2, Selected: []string{"unsupported", "supported"}},
		{Name: "high-load", Observed: "same", ActiveSpecialists: 2, Capacity: 1, Selected: []string{"unsupported"}},
		{Name: "high-action-pressure", Observed: "same", PendingActions: 2, Capacity: 1, Selected: []string{"unsupported"}},
		{Name: "high-surprise", Observed: "different", Capacity: 2, RequireEvidence: true, Selected: []string{"supported"}},
		{Name: "high-salience", Observed: "same", Salience: 0.9, Capacity: 2, RequireEvidence: true, Selected: []string{"supported"}},
	}
	results := make(map[string]result, len(scenarios))
	for replicate := 1; replicate <= 3; replicate++ {
		for _, test := range scenarios {
			observed, err := runScenario(ctx, replicate, test)
			fatalIf(err)
			fmt.Printf("replicate %d %s: capacity=%d evidence_required=%t error=%.1f selected=%v regulation_event=%s selection_event=%s\n",
				replicate, test.Name, observed.Capacity, observed.EvidenceRequired, observed.PredictionError,
				observed.Specialists, observed.RegulationEventID, observed.SelectionEventID)
			if observed.Capacity != test.Capacity || observed.EvidenceRequired != test.RequireEvidence || !slices.Equal(observed.Specialists, test.Selected) {
				fatalIf(fmt.Errorf("scenario %q changed from its expected selection", test.Name))
			}
			if replicate == 1 {
				results[test.Name] = observed
			} else if !sameBehavior(results[test.Name], observed) {
				fatalIf(fmt.Errorf("scenario %q changed behavior across repetitions", test.Name))
			}
		}
	}
	fmt.Printf("repeatable behavioral changes: yes (selection event %s; regulation event %s)\n",
		results["high-load"].SelectionEventID, results["high-load"].RegulationEventID)
}

func runScenario(ctx context.Context, replicate int, test scenario) (result, error) {
	interactionID := fmt.Sprintf("%s-%d", test.Name, replicate)
	store, err := cognition.NewMemoryStore(128)
	if err != nil {
		return result{}, err
	}
	events, err := cognition.NewSpine("regulation-experiment-"+interactionID, store)
	if err != nil {
		return result{}, err
	}
	stimulus, err := events.Emit(ctx, cognition.Draft{
		CorrelationID: interactionID, Kind: cognition.KindPerception,
		Payload: json.RawMessage(`{"task":"choose a candidate"}`),
	})
	if err != nil {
		return result{}, err
	}
	config := workspace.DefaultConfig()
	config.Capacity = 2
	proposalWorkspace, err := workspace.New(config, events)
	if err != nil {
		return result{}, err
	}
	runID := "workspace-" + interactionID
	if err := proposalWorkspace.Open(ctx, workspace.Request{
		ID: runID, InteractionID: interactionID, SourceEventIDs: []string{stimulus.EventID},
	}); err != nil {
		return result{}, err
	}
	proposals := []agent.Proposal{
		{ID: "unsupported-" + interactionID, InteractionID: interactionID, SpecialistID: "unsupported", Status: "candidate", Confidence: 0.95},
		{ID: "supported-" + interactionID, InteractionID: interactionID, SpecialistID: "supported", Status: "candidate", Confidence: 0.7, EvidenceEventIDs: []string{stimulus.EventID}},
	}
	for index := range proposals {
		event, emitErr := events.Emit(ctx, cognition.Draft{
			AgentID: proposals[index].SpecialistID, CorrelationID: interactionID,
			ParentEventIDs: []string{stimulus.EventID}, Kind: cognition.KindProposal,
			Payload: json.RawMessage(fmt.Sprintf(`{"proposal_id":%q}`, proposals[index].ID)),
		})
		if emitErr != nil {
			return result{}, emitErr
		}
		proposals[index].EventID = event.EventID
		if err := proposalWorkspace.Submit(ctx, runID, proposals[index]); err != nil {
			return result{}, err
		}
	}
	regulator, err := regulation.New(regulation.DefaultPolicy(), events)
	if err != nil {
		return result{}, err
	}
	input := regulation.Input{
		InteractionID: interactionID, SourceEventIDs: []string{stimulus.EventID},
		Expected:          json.RawMessage(`{"outcome":"same"}`),
		Observed:          json.RawMessage(fmt.Sprintf(`{"outcome":%q}`, test.Observed)),
		DeclaredSalience:  test.Salience,
		ActiveSpecialists: test.ActiveSpecialists, SpecialistCapacity: 2,
		PendingActions: test.PendingActions, ActionCapacity: 2, WorkspaceCapacity: 2,
	}
	snapshot, err := regulator.Evaluate(ctx, input)
	if err != nil {
		return result{}, err
	}
	report, err := proposalWorkspace.SelectWithPolicy(ctx, runID, snapshot.WorkspacePolicy)
	if err != nil {
		return result{}, err
	}
	selected := make([]string, 0, len(report.Selection.SelectedIDs))
	for _, entry := range report.Selection.Entries {
		if entry.Selected {
			selected = append(selected, entry.Specialist)
		}
	}
	return result{
		Specialists: selected, Capacity: report.Selection.Capacity,
		EvidenceRequired: report.Selection.EvidenceRequired,
		PredictionError:  snapshot.PredictionError,
		SelectionEventID: report.Selection.EventID, RegulationEventID: snapshot.RegulationEventID,
	}, nil
}

func sameBehavior(left, right result) bool {
	return left.Capacity == right.Capacity && left.EvidenceRequired == right.EvidenceRequired &&
		left.PredictionError == right.PredictionError && slices.Equal(left.Specialists, right.Specialists)
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
