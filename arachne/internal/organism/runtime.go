// Package organism composes persistent specialist, workspace, and action stages.
package organism

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/workspace"
)

const maxSpecialists = 64

// Perception is one explicit stimulus presented to the persistent organism runtime.
type Perception struct {
	InteractionID  string          `json:"interaction_id,omitempty"`
	SessionID      string          `json:"session_id,omitempty"`
	Input          json.RawMessage `json:"input"`
	Features       []float64       `json:"features,omitempty"`
	SourceEventIDs []string        `json:"source_event_ids,omitempty"`
}

// RegisteredSpecialist binds a stable identity to one proposal-only implementation.
type RegisteredSpecialist struct {
	ID             string
	Implementation agent.Specialist
}

// Commitment identifies one selected requested action or a deliberate no-op.
type Commitment struct {
	InteractionID    string                `json:"interaction_id"`
	SpecialistID     string                `json:"specialist_id,omitempty"`
	ProposalID       string                `json:"proposal_id,omitempty"`
	ProposalEventID  string                `json:"proposal_event_id,omitempty"`
	SelectionEventID string                `json:"selection_event_id"`
	Action           agent.RequestedAction `json:"action,omitempty"`
	NoOp             bool                  `json:"no_op"`
	Rationale        string                `json:"rationale"`
	EventID          string                `json:"event_id"`
}

// EligibilityResult records whether the committed action may be executed.
type EligibilityResult struct {
	Eligible             bool   `json:"eligible"`
	Reason               string `json:"reason"`
	GovernanceDecisionID string `json:"governance_decision_id,omitempty"`
	EventID              string `json:"event_id,omitempty"`
}

// Outcome records what happened in the environment after commitment.
type Outcome struct {
	Status              string             `json:"status"`
	Result              json.RawMessage    `json:"result,omitempty"`
	ExpectedEvidence    json.RawMessage    `json:"expected_evidence,omitempty"`
	ObservedEvidence    json.RawMessage    `json:"observed_evidence,omitempty"`
	PredictionAvailable bool               `json:"prediction_available"`
	ConditionDeltas     map[string]float64 `json:"condition_deltas,omitempty"`
	Error               string             `json:"error,omitempty"`
	EventID             string             `json:"event_id"`
}

// EventIDs names the event spine records emitted during a step.
type EventIDs struct {
	Perception      string   `json:"perception"`
	Activations     []string `json:"activations,omitempty"`
	Proposals       []string `json:"proposals,omitempty"`
	Selection       string   `json:"selection,omitempty"`
	Commitment      string   `json:"commitment,omitempty"`
	Eligibility     string   `json:"eligibility,omitempty"`
	Action          string   `json:"action,omitempty"`
	Outcome         string   `json:"outcome,omitempty"`
	Memory          string   `json:"memory,omitempty"`
	Regulation      string   `json:"regulation,omitempty"`
	PredictionError string   `json:"prediction_error,omitempty"`
	Learning        []string `json:"learning,omitempty"`
	Replay          []string `json:"replay,omitempty"`
}

// StepResult is the in-memory view of the same causal chain stored on the spine.
type StepResult struct {
	InteractionID string             `json:"interaction_id"`
	Events        EventIDs           `json:"events"`
	Proposals     []agent.Proposal   `json:"proposals"`
	Workspace     workspace.Report   `json:"workspace"`
	Commitment    Commitment         `json:"commitment"`
	Eligibility   *EligibilityResult `json:"eligibility,omitempty"`
	Outcome       *Outcome           `json:"outcome,omitempty"`
}

// Actuator executes an already committed action. It receives no policy authority.
type Actuator interface {
	Execute(context.Context, agent.RequestedAction) (json.RawMessage, error)
}

// ActuatorResult adds explicit evidence to the stable actuator result contract.
type ActuatorResult struct {
	Result           json.RawMessage
	ExpectedEvidence json.RawMessage
	ObservedEvidence json.RawMessage
	ConditionDeltas  map[string]float64
}

// DetailedActuator optionally reports explicit observations and internal condition changes.
type DetailedActuator interface {
	ExecuteDetailed(context.Context, agent.RequestedAction) (ActuatorResult, error)
}

// OutcomeScorer supplies bounded target scores after a reported outcome.
type OutcomeScorer func(context.Context, StepResult) (map[string]float64, error)

// OutcomeLearner receives one explicit action target score from the runtime.
type OutcomeLearner interface {
	UpdateOutcome(context.Context, []float64, string, float64) (json.RawMessage, error)
}

// ReplaySelector opts into replay by selecting episode IDs from recent recorded experience.
type ReplaySelector func(context.Context, []memory.Episode) ([]string, error)

// EligibilityGate determines whether a committed action may reach the actuator.
type EligibilityGate interface {
	Evaluate(context.Context, Commitment) (EligibilityResult, error)
}

// Config supplies the persistent organism identity and its fixed runtime dependencies.
type Config struct {
	OrganismID     string
	Events         *cognition.Spine
	Specialists    []RegisteredSpecialist
	Workspace      workspace.Config
	Gate           EligibilityGate
	Actuator       Actuator
	Memory         *memory.Service
	Regulator      *regulation.Regulator
	Scorer         OutcomeScorer
	Learners       map[string]OutcomeLearner
	ReplaySelector ReplaySelector
}

// Runtime serializes steps to preserve deterministic interaction IDs and event ordering.
type Runtime struct {
	mu             sync.Mutex
	organismID     string
	events         *cognition.Spine
	specialists    []*agent.SpecialistAgent
	workspace      workspace.Config
	gate           EligibilityGate
	actuator       Actuator
	memory         *memory.Service
	regulator      *regulation.Regulator
	scorer         OutcomeScorer
	learners       map[string]OutcomeLearner
	replaySelector ReplaySelector
	conditions     map[string]float64
	step           uint64
}

// New creates a persistent runtime with an immutable bounded specialist set.
func New(config Config) (*Runtime, error) {
	if strings.TrimSpace(config.OrganismID) == "" || config.Events == nil || config.Gate == nil || config.Actuator == nil {
		return nil, errors.New("organism ID, event spine, eligibility gate, and actuator are required")
	}
	if config.ReplaySelector != nil && config.Memory == nil {
		return nil, errors.New("memory service is required when replay selection is configured")
	}
	if len(config.Specialists) == 0 || len(config.Specialists) > maxSpecialists {
		return nil, fmt.Errorf("specialist count must be between one and %d", maxSpecialists)
	}
	if config.Workspace == (workspace.Config{}) {
		config.Workspace = workspace.DefaultConfig()
	}
	if config.Workspace.MaxProposals < len(config.Specialists) {
		return nil, errors.New("workspace proposal limit must include every registered specialist")
	}
	seen := make(map[string]struct{}, len(config.Specialists))
	specialists := make([]*agent.SpecialistAgent, 0, len(config.Specialists))
	for _, registered := range config.Specialists {
		if _, exists := seen[registered.ID]; exists {
			return nil, fmt.Errorf("duplicate specialist ID %q", registered.ID)
		}
		seen[registered.ID] = struct{}{}
		specialist, err := agent.NewSpecialistAgent(registered.ID, registered.Implementation, config.Events)
		if err != nil {
			return nil, err
		}
		specialists = append(specialists, specialist)
	}
	return &Runtime{
		organismID:     config.OrganismID,
		events:         config.Events,
		specialists:    specialists,
		workspace:      config.Workspace,
		gate:           config.Gate,
		actuator:       config.Actuator,
		memory:         config.Memory,
		regulator:      config.Regulator,
		scorer:         config.Scorer,
		learners:       maps.Clone(config.Learners),
		replaySelector: config.ReplaySelector,
		conditions:     make(map[string]float64),
	}, nil
}

// Step runs one complete perception-to-outcome cycle.
func (r *Runtime) Step(ctx context.Context, perception Perception) (StepResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return StepResult{}, err
	}
	if len(perception.Input) == 0 || len(perception.Input) > 1<<20 || !json.Valid(perception.Input) {
		return StepResult{}, errors.New("perception input must be valid JSON no larger than one MiB")
	}
	r.step++
	runID := fmt.Sprintf("%s:run:%d", r.organismID, r.step)
	interactionID := strings.TrimSpace(perception.InteractionID)
	if interactionID == "" {
		interactionID = fmt.Sprintf("%s:interaction:%d", r.organismID, r.step)
	}
	result := StepResult{InteractionID: interactionID}
	payload, err := json.Marshal(perception)
	if err != nil {
		return result, err
	}
	perceptionEvent, err := r.events.Emit(ctx, cognition.Draft{
		SessionID: perception.SessionID, CorrelationID: interactionID,
		ParentEventIDs: unique(perception.SourceEventIDs), Kind: cognition.KindPerception, Payload: payload,
	})
	if err != nil {
		return result, fmt.Errorf("record perception: %w", err)
	}
	result.Events.Perception = perceptionEvent.EventID
	report, proposals, activations, proposalIDs, err := r.collectProposals(ctx, perception, interactionID, runID, perceptionEvent.EventID)
	if err != nil {
		return result, err
	}
	result.Workspace = report
	result.Proposals = proposals
	result.Events.Activations = activations
	result.Events.Proposals = proposalIDs
	result.Events.Selection = report.Selection.EventID
	result, stepErr := r.commitAndExecute(ctx, perception, result, report)
	if result.Outcome != nil {
		if err := r.applyOutcomeFeedback(ctx, perception, &result); err != nil {
			return result, errors.Join(stepErr, err)
		}
	}
	return result, stepErr
}

func (r *Runtime) collectProposals(ctx context.Context, perception Perception, interactionID, runID, perceptionEventID string) (workspace.Report, []agent.Proposal, []string, []string, error) {
	service, err := workspace.New(r.workspace, r.events)
	if err != nil {
		return workspace.Report{}, nil, nil, nil, fmt.Errorf("create step workspace: %w", err)
	}
	sourceIDs := append(unique(perception.SourceEventIDs), perceptionEventID)
	recent := []memory.Episode(nil)
	if r.memory != nil {
		var err error
		recent, err = r.memory.RetrieveEpisodes(ctx, memory.EpisodeQuery{SessionID: perception.SessionID, Limit: 3})
		if err != nil {
			return workspace.Report{}, nil, nil, nil, fmt.Errorf("retrieve recent episodes: %w", err)
		}
		for _, episode := range recent {
			sourceIDs = append(sourceIDs, episode.SourceEventIDs...)
		}
		sourceIDs = unique(sourceIDs)
	}
	if err := service.Open(ctx, workspace.Request{
		ID: runID, InteractionID: interactionID, SessionID: perception.SessionID, SourceEventIDs: sourceIDs,
	}); err != nil {
		return workspace.Report{}, nil, nil, nil, fmt.Errorf("open workspace run: %w", err)
	}
	proposals := make([]agent.Proposal, 0, len(r.specialists))
	activations := make([]string, 0, len(r.specialists))
	proposalIDs := make([]string, 0, len(r.specialists))
	for _, specialist := range r.specialists {
		if err := ctx.Err(); err != nil {
			return workspace.Report{}, proposals, activations, proposalIDs, err
		}
		activation := agent.Activation{
			InteractionID: interactionID, SessionID: perception.SessionID,
			Input:    append(json.RawMessage(nil), perception.Input...),
			Features: append([]float64(nil), perception.Features...), SourceEventIDs: sourceIDs,
			RecentEpisodes: cloneEpisodes(recent), InternalConditions: maps.Clone(r.conditions),
		}
		proposal, activationEvent, err := specialist.Activate(ctx, activation, []string{perceptionEventID})
		if err != nil {
			return workspace.Report{}, proposals, activations, proposalIDs, err
		}
		proposals = append(proposals, proposal)
		activations = append(activations, activationEvent.EventID)
		proposalIDs = append(proposalIDs, proposal.EventID)
		if err := service.Submit(ctx, runID, proposal); err != nil {
			return workspace.Report{}, proposals, activations, proposalIDs, fmt.Errorf("submit proposal from %q: %w", proposal.SpecialistID, err)
		}
	}
	report, err := service.Select(ctx, runID)
	if err != nil {
		return workspace.Report{}, proposals, activations, proposalIDs, fmt.Errorf("select workspace proposals: %w", err)
	}
	return report, proposals, activations, proposalIDs, nil
}

func (r *Runtime) commitAndExecute(ctx context.Context, perception Perception, result StepResult, report workspace.Report) (StepResult, error) {
	candidate := chooseAction(report)
	if candidate == nil {
		commitment := Commitment{
			InteractionID: result.InteractionID, SelectionEventID: report.Selection.EventID,
			NoOp: true, Rationale: "no valid requested action belongs to a selected proposal",
		}
		if err := r.recordCommitment(ctx, &result, &commitment, nil); err != nil {
			return result, err
		}
		outcome := Outcome{Status: "no_op"}
		if err := r.recordOutcome(ctx, &result, &outcome, commitment.EventID); err != nil {
			return result, err
		}
		result.Commitment, result.Outcome = commitment, &outcome
		return result, nil
	}
	commitment := Commitment{
		InteractionID: result.InteractionID, SpecialistID: candidate.proposal.SpecialistID,
		ProposalID: candidate.proposal.ID, ProposalEventID: candidate.proposal.EventID,
		SelectionEventID: report.Selection.EventID, Action: candidate.action,
		Rationale: "selected lexicographically by specialist ID, proposal ID, action kind, and canonical payload",
	}
	if err := r.recordCommitment(ctx, &result, &commitment, []string{candidate.proposal.EventID}); err != nil {
		return result, err
	}
	result.Commitment = commitment
	eligibility, gateEvent, err := r.recordEligibility(ctx, perception, result.InteractionID, commitment)
	if err != nil {
		return result, err
	}
	result.Eligibility = &eligibility
	result.Events.Eligibility = gateEvent.EventID
	if !eligibility.Eligible {
		outcome := Outcome{Status: "ineligible", Error: eligibility.Reason}
		if err := r.recordOutcome(ctx, &result, &outcome, gateEvent.EventID); err != nil {
			return result, err
		}
		result.Outcome = &outcome
		return result, nil
	}
	return r.executeCommitted(ctx, perception, result, commitment, gateEvent)
}

// Conditions returns an isolated snapshot of accumulated numeric internal conditions.
func (r *Runtime) Conditions() map[string]float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return maps.Clone(r.conditions)
}

func (r *Runtime) applyOutcomeFeedback(ctx context.Context, perception Perception, result *StepResult) error {
	if result.Outcome == nil {
		return nil
	}
	sourceIDs := outcomeSourceIDs(*result)
	var episode memory.Episode
	if r.memory != nil {
		var err error
		episode, err = r.recordOutcomeEpisode(ctx, perception, *result, sourceIDs)
		if err != nil {
			return err
		}
		result.Events.Memory = episode.EventID
	}
	for name, delta := range result.Outcome.ConditionDeltas {
		r.conditions[name] += delta
	}
	if err := r.applyRegulationFeedback(ctx, perception, result, sourceIDs); err != nil {
		return err
	}
	if err := r.applyLearningFeedback(ctx, perception, result); err != nil {
		return err
	}
	return r.runOutcomeReplay(ctx, result, episode, sourceIDs)
}

func outcomeSourceIDs(result StepResult) []string {
	ids := []string{
		result.Events.Perception, result.Events.Selection, result.Events.Commitment,
		result.Events.Eligibility, result.Events.Action, result.Events.Outcome,
	}
	ids = append(ids, result.Events.Proposals...)
	ids = append(ids, result.Commitment.ProposalEventID)
	return unique(ids)
}

func (r *Runtime) recordOutcomeEpisode(ctx context.Context, perception Perception, result StepResult, sourceIDs []string) (memory.Episode, error) {
	content, err := json.Marshal(map[string]any{
		"perception": perception, "commitment": result.Commitment, "outcome": result.Outcome,
	})
	if err != nil {
		return memory.Episode{}, fmt.Errorf("encode attributed outcome episode: %w", err)
	}
	episode, err := r.memory.RecordEpisode(ctx, memory.Episode{
		OrganismID: r.organismID, Source: memory.SourceRef{Kind: "organism_interaction", ID: result.InteractionID},
		SourceEventIDs: sourceIDs, OccurredAt: time.Now().UTC(), AgentID: result.Commitment.SpecialistID,
		SessionID: perception.SessionID, CorrelationID: result.InteractionID,
		Kind: "action_outcome", Content: content,
	})
	if err != nil {
		return memory.Episode{}, fmt.Errorf("record outcome episode: %w", err)
	}
	return episode, nil
}

func (r *Runtime) applyRegulationFeedback(ctx context.Context, perception Perception, result *StepResult, sourceIDs []string) error {
	if r.regulator == nil {
		return nil
	}
	outcome := result.Outcome
	input := regulation.Input{
		InteractionID: result.InteractionID, SessionID: perception.SessionID,
		SourceEventIDs:     unique(append(sourceIDs, result.Events.Memory)),
		Expected:           append(json.RawMessage(nil), outcome.ExpectedEvidence...),
		Observed:           append(json.RawMessage(nil), outcome.ObservedEvidence...),
		ConditionDeltas:    maps.Clone(outcome.ConditionDeltas),
		SpecialistCapacity: max(1, len(r.specialists)), ActionCapacity: 1, WorkspaceCapacity: r.workspace.Capacity,
	}
	snapshot, err := r.regulator.Evaluate(ctx, input)
	if err != nil {
		return fmt.Errorf("regulate reported outcome: %w", err)
	}
	result.Events.Regulation = snapshot.RegulationEventID
	result.Events.PredictionError = snapshot.Prediction.EventID
	return nil
}

func (r *Runtime) applyLearningFeedback(ctx context.Context, perception Perception, result *StepResult) error {
	if r.scorer == nil || result.Commitment.NoOp {
		return nil
	}
	targets, err := r.scorer(ctx, *result)
	if err != nil {
		return fmt.Errorf("score reported outcome: %w", err)
	}
	if err := validateTargetScores(targets); err != nil {
		return err
	}
	learner := r.learners[result.Commitment.SpecialistID]
	score, exists := targets[result.Commitment.Action.Kind]
	if learner == nil || !exists {
		return nil
	}
	metadata, err := learner.UpdateOutcome(ctx, append([]float64(nil), perception.Features...), result.Commitment.Action.Kind, score)
	if err != nil {
		return fmt.Errorf("update learner %q: %w", result.Commitment.SpecialistID, err)
	}
	event, err := r.events.EmitJSON(ctx, cognition.Draft{
		AgentID: result.Commitment.SpecialistID, SessionID: perception.SessionID,
		CorrelationID:  result.InteractionID,
		ParentEventIDs: unique([]string{result.Events.Outcome, result.Events.Memory}),
		Kind:           cognition.KindLearning,
	}, map[string]any{"action_kind": result.Commitment.Action.Kind, "target_score": score, "metadata": metadata})
	if err != nil {
		return fmt.Errorf("record learner update: %w", err)
	}
	result.Events.Learning = append(result.Events.Learning, event.EventID)
	return nil
}

func validateTargetScores(targets map[string]float64) error {
	for kind, score := range targets {
		if strings.TrimSpace(kind) == "" || math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
			return fmt.Errorf("outcome scorer returned invalid score for action %q", kind)
		}
	}
	return nil
}

func (r *Runtime) runOutcomeReplay(ctx context.Context, result *StepResult, episode memory.Episode, sourceIDs []string) error {
	if r.replaySelector == nil || r.memory == nil {
		return nil
	}
	selected, err := r.replaySelector(ctx, cloneEpisodes([]memory.Episode{episode}))
	if err != nil {
		return fmt.Errorf("select replay episodes: %w", err)
	}
	if len(selected) == 0 {
		return nil
	}
	plan, err := r.memory.ScheduleReplay(ctx, memory.ReplayPlan{
		InteractionID: result.InteractionID, RequestedBy: r.organismID, ScheduledAt: time.Now().UTC(),
		EpisodeIDs: unique(selected), SourceEventIDs: unique(append(sourceIDs, result.Events.Memory)),
	})
	if err != nil {
		return fmt.Errorf("schedule outcome replay: %w", err)
	}
	run, err := r.memory.RunReplay(ctx, plan)
	if err != nil {
		return fmt.Errorf("run outcome replay: %w", err)
	}
	result.Events.Replay = append(result.Events.Replay, plan.ScheduleEventID, run.EventID)
	return nil
}

func validDeltas(values map[string]float64) bool {
	for key, value := range values {
		if strings.TrimSpace(key) == "" || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func cloneEpisodes(values []memory.Episode) []memory.Episode {
	result := make([]memory.Episode, len(values))
	for i, value := range values {
		result[i] = value
		result[i].SourceEventIDs = append([]string(nil), value.SourceEventIDs...)
		result[i].Tags = append([]string(nil), value.Tags...)
		result[i].Content = append(json.RawMessage(nil), value.Content...)
	}
	return result
}

func (r *Runtime) recordEligibility(ctx context.Context, perception Perception, interactionID string, commitment Commitment) (EligibilityResult, cognition.Event, error) {
	eligibility, err := r.gate.Evaluate(ctx, commitment)
	if err != nil {
		return EligibilityResult{}, cognition.Event{}, fmt.Errorf("evaluate committed action eligibility: %w", err)
	}
	parents := []string{commitment.EventID}
	if eligibility.GovernanceDecisionID != "" {
		parents = append(parents, eligibility.GovernanceDecisionID)
	}
	event, err := r.events.EmitJSON(ctx, cognition.Draft{
		SessionID: perception.SessionID, CorrelationID: interactionID,
		ParentEventIDs: parents, Kind: cognition.KindDecision,
	}, eligibility)
	if err != nil {
		return EligibilityResult{}, cognition.Event{}, fmt.Errorf("record eligibility result: %w", err)
	}
	eligibility.EventID = event.EventID
	return eligibility, event, nil
}

func (r *Runtime) executeCommitted(ctx context.Context, perception Perception, result StepResult, commitment Commitment, gateEvent cognition.Event) (StepResult, error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "action_committed", "interaction_id": result.InteractionID,
		"specialist_id": commitment.SpecialistID, "proposal_id": commitment.ProposalID,
		"proposal_event_id": commitment.ProposalEventID, "commitment_event_id": commitment.EventID,
		"kind": commitment.Action.Kind, "payload": commitment.Action.Payload,
	})
	if err != nil {
		return result, err
	}
	actionEvent, err := r.events.Emit(ctx, cognition.Draft{
		AgentID: commitment.SpecialistID, SessionID: perception.SessionID, CorrelationID: result.InteractionID,
		ParentEventIDs: []string{commitment.EventID, gateEvent.EventID, commitment.ProposalEventID},
		Kind:           cognition.KindAction, Payload: payload,
	})
	if err != nil {
		return result, fmt.Errorf("record action dispatch: %w", err)
	}
	result.Events.Action = actionEvent.EventID
	var detailed ActuatorResult
	var executeErr error
	if actuator, ok := r.actuator.(DetailedActuator); ok {
		detailed, executeErr = actuator.ExecuteDetailed(ctx, commitment.Action)
	} else {
		detailed.Result, executeErr = r.actuator.Execute(ctx, commitment.Action)
	}
	outcome := Outcome{
		Status: "executed", Result: append(json.RawMessage(nil), detailed.Result...),
		PredictionAvailable: len(detailed.ExpectedEvidence) > 0 && len(detailed.ObservedEvidence) > 0,
		ExpectedEvidence:    append(json.RawMessage(nil), detailed.ExpectedEvidence...),
		ObservedEvidence:    append(json.RawMessage(nil), detailed.ObservedEvidence...),
		ConditionDeltas:     maps.Clone(detailed.ConditionDeltas),
	}
	if executeErr != nil {
		outcome.Status, outcome.Error = "failed", executeErr.Error()
	}
	if len(outcome.Result) > 0 && !json.Valid(outcome.Result) {
		outcome.Status, outcome.Error, outcome.Result = "failed", "actuator returned invalid JSON outcome", nil
	}
	if len(outcome.ExpectedEvidence) > 0 && !json.Valid(outcome.ExpectedEvidence) ||
		len(outcome.ObservedEvidence) > 0 && !json.Valid(outcome.ObservedEvidence) || !validDeltas(outcome.ConditionDeltas) || (len(outcome.ExpectedEvidence) == 0) != (len(outcome.ObservedEvidence) == 0) {
		outcome.Status, outcome.Error = "failed", "actuator returned invalid detailed outcome evidence"
		outcome.ExpectedEvidence, outcome.ObservedEvidence, outcome.ConditionDeltas = nil, nil, nil
		outcome.PredictionAvailable = false
	}
	if err := r.recordOutcome(ctx, &result, &outcome, actionEvent.EventID); err != nil {
		return result, err
	}
	result.Outcome = &outcome
	if executeErr != nil {
		return result, fmt.Errorf("execute committed action: %w", executeErr)
	}
	if outcome.Status == "failed" {
		return result, errors.New(outcome.Error)
	}
	return result, nil
}

type actionCandidate struct {
	proposal agent.Proposal
	action   agent.RequestedAction
	payload  string
}

func chooseAction(report workspace.Report) *actionCandidate {
	candidates := actionCandidates(report)
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return actionCandidateLess(candidates[i], candidates[j])
	})
	return &candidates[0]
}

func actionCandidates(report workspace.Report) []actionCandidate {
	if report.Selection == nil {
		return nil
	}
	selected := make(map[string]struct{}, len(report.Selection.SelectedIDs))
	for _, id := range report.Selection.SelectedIDs {
		selected[id] = struct{}{}
	}
	candidates := make([]actionCandidate, 0)
	for _, proposal := range report.Proposals {
		if proposal.Status != "candidate" {
			continue
		}
		if _, ok := selected[proposal.ID]; !ok {
			continue
		}
		for _, requested := range proposal.RequestedActions {
			if agent.ValidateRequestedAction(requested) != nil {
				continue
			}
			canonical, err := canonicalJSON(requested.Payload)
			if err != nil {
				continue
			}
			action := requested
			action.Payload = json.RawMessage(canonical)
			candidates = append(candidates, actionCandidate{
				proposal: proposal, action: action, payload: string(canonical),
			})
		}
	}
	return candidates
}

func actionCandidateLess(left, right actionCandidate) bool {
	if left.proposal.SpecialistID != right.proposal.SpecialistID {
		return left.proposal.SpecialistID < right.proposal.SpecialistID
	}
	if left.proposal.ID != right.proposal.ID {
		return left.proposal.ID < right.proposal.ID
	}
	if left.action.Kind != right.action.Kind {
		return left.action.Kind < right.action.Kind
	}
	return left.payload < right.payload
}

func canonicalJSON(payload json.RawMessage) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func (r *Runtime) recordCommitment(ctx context.Context, result *StepResult, commitment *Commitment, extraParents []string) error {
	parents := append([]string{commitment.SelectionEventID}, extraParents...)
	event, err := r.events.EmitJSON(ctx, cognition.Draft{
		SessionID: result.Workspace.Request.SessionID, CorrelationID: commitment.InteractionID,
		ParentEventIDs: unique(parents), Kind: cognition.KindDecision,
	}, commitment)
	if err != nil {
		return fmt.Errorf("record action commitment: %w", err)
	}
	commitment.EventID = event.EventID
	result.Events.Commitment = event.EventID
	return nil
}

func (r *Runtime) recordOutcome(ctx context.Context, result *StepResult, outcome *Outcome, parentID string) error {
	event, err := r.events.EmitJSON(ctx, cognition.Draft{
		SessionID: result.Workspace.Request.SessionID, CorrelationID: result.InteractionID,
		ParentEventIDs: []string{parentID}, Kind: cognition.KindOutcome,
	}, outcome)
	if err != nil {
		return fmt.Errorf("record action outcome: %w", err)
	}
	outcome.EventID = event.EventID
	result.Events.Outcome = event.EventID
	return nil
}

// FixtureGate allows only action payloads whose scope is explicitly configured.
type FixtureGate struct {
	AllowedScopes []string
}

// Evaluate implements EligibilityGate with exact fixture scope matching.
func (g FixtureGate) Evaluate(_ context.Context, commitment Commitment) (EligibilityResult, error) {
	var payload struct {
		Scope string `json:"scope"`
	}
	if err := json.Unmarshal(commitment.Action.Payload, &payload); err != nil {
		return EligibilityResult{}, fmt.Errorf("decode fixture action scope: %w", err)
	}
	for _, scope := range g.AllowedScopes {
		if payload.Scope == scope && scope != "" {
			return EligibilityResult{Eligible: true, Reason: "action scope is explicitly allowed by fixture gate"}, nil
		}
	}
	return EligibilityResult{Reason: "action scope is not allowed by fixture gate"}, nil
}

// GovernanceGate adapts committed actions to Arachne's consequential action policy.
type GovernanceGate struct {
	Service       *governance.Service
	ClassByAction map[string]governance.ActionClass
	Approvals     func(context.Context, Commitment) ([]governance.Approval, error)
}

// Evaluate records a governance decision and grants eligibility only for approval.
func (g GovernanceGate) Evaluate(ctx context.Context, commitment Commitment) (EligibilityResult, error) {
	if g.Service == nil {
		return EligibilityResult{}, errors.New("governance service is required")
	}
	class, exists := g.ClassByAction[commitment.Action.Kind]
	if !exists {
		return EligibilityResult{Reason: "action kind has no governance class"}, nil
	}
	var target struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(commitment.Action.Payload, &target); err != nil {
		return EligibilityResult{}, fmt.Errorf("decode governance action target: %w", err)
	}
	approvals := []governance.Approval(nil)
	if g.Approvals != nil {
		var err error
		approvals, err = g.Approvals(ctx, commitment)
		if err != nil {
			return EligibilityResult{}, err
		}
	}
	proposal := governance.Proposal{
		ID:            commitment.ProposalID + ":" + commitment.Action.Kind,
		InteractionID: commitment.InteractionID, ProposerID: commitment.SpecialistID,
		Class: class, Target: target.Target, Action: commitment.Action.Kind,
		Payload:        append(json.RawMessage(nil), commitment.Action.Payload...),
		CreatedAt:      time.Now().UTC(),
		SourceEventIDs: unique([]string{commitment.ProposalEventID, commitment.EventID}),
	}
	decision, err := g.Service.Evaluate(ctx, proposal, approvals)
	if err != nil {
		return EligibilityResult{}, err
	}
	return EligibilityResult{
		Eligible:             decision.Outcome == governance.OutcomeApproved,
		Reason:               strings.Join(decision.Reasons, "; "),
		GovernanceDecisionID: decision.EventID,
	}, nil
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
