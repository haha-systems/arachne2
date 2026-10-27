package organism

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/workspace"
)

const runtimeSnapshotSchema = "arachne.organism_runtime.v1"

// ConditionValue is one runtime condition in canonical order.
type ConditionValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// DeterminismStatus records whether the embedding's sources can reproduce continuation.
type DeterminismStatus struct {
	Deterministic bool     `json:"deterministic"`
	Reasons       []string `json:"reasons,omitempty"`
}

type runtimeSnapshotState struct {
	OrganismID        string                            `json:"organism_id"`
	Step              uint64                            `json:"step"`
	Workspace         workspace.Config                  `json:"workspace"`
	DeferredExecution bool                              `json:"deferred_execution"`
	Conditions        []ConditionValue                  `json:"conditions"`
	Specialists       []agent.RuntimeSpecialistSnapshot `json:"specialists"`
	Learners          []agent.RuntimeSpecialistSnapshot `json:"learners"`
	Restorability     RestorabilityReport               `json:"restorability"`
	Mode              SnapshotMode                      `json:"mode"`
	Events            cognition.EventSnapshot           `json:"events"`
	Memory            *memory.StateSnapshot             `json:"memory,omitempty"`
	Regulation        *regulation.StateSnapshot         `json:"regulation,omitempty"`
	Determinism       DeterminismStatus                 `json:"determinism"`
}

type runtimeSnapshotEnvelope struct {
	SchemaVersion string               `json:"schema_version"`
	State         runtimeSnapshotState `json:"state"`
	SHA256        string               `json:"sha256"`
}

type pendingStep struct {
	perception Perception
	result     StepResult
}

// SnapshotState returns an exact checksummed, versioned runtime snapshot.
func (r *Runtime) SnapshotState(ctx context.Context) (json.RawMessage, error) {
	return r.SnapshotStateWithMode(ctx, SnapshotExact)
}

// SnapshotBestEffortState records available state and explicit blockers without claiming exactness.
func (r *Runtime) SnapshotBestEffortState(ctx context.Context) (json.RawMessage, error) {
	return r.SnapshotStateWithMode(ctx, SnapshotBestEffort)
}

// SnapshotStateWithMode exports exact state or an explicitly marked best-effort inventory.
func (r *Runtime) SnapshotStateWithMode(ctx context.Context, mode SnapshotMode) (json.RawMessage, error) {
	if mode != SnapshotExact && mode != SnapshotBestEffort {
		return nil, fmt.Errorf("unknown organism snapshot mode %q", mode)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) != 0 {
		return nil, errors.New("cannot snapshot while a committed outcome is pending")
	}
	restorability := r.runtimeRestorability()
	if mode == SnapshotExact && !restorability.Exact {
		for _, component := range restorability.Components {
			if component.Restorability == RestorabilityOpaque {
				return nil, fmt.Errorf("exact snapshot refused: component %q (%s): %s", component.ID, component.Kind, component.Reason)
			}
		}
	}
	events, err := r.events.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot cognitive events: %w", err)
	}
	state := runtimeSnapshotState{
		OrganismID: r.organismID, Step: r.step, Workspace: r.workspace,
		DeferredExecution: r.deferredExecution, Events: events,
		Determinism: r.determinismStatus(), Restorability: restorability, Mode: mode,
	}
	for name, value := range r.conditions {
		state.Conditions = append(state.Conditions, ConditionValue{Name: name, Value: value})
	}
	sort.Slice(state.Conditions, func(i, j int) bool { return state.Conditions[i].Name < state.Conditions[j].Name })
	state.Specialists = make([]agent.RuntimeSpecialistSnapshot, 0, len(r.specialists))
	for _, specialist := range r.specialists {
		snapshot, err := specialist.SnapshotState()
		if err != nil {
			if mode == SnapshotBestEffort {
				state.Specialists = append(state.Specialists, agent.RuntimeSpecialistSnapshot{ID: specialist.ID(), StateKind: "opaque", Reason: err.Error()})
				continue
			}
			return nil, err
		}
		state.Specialists = append(state.Specialists, snapshot)
	}
	learnerIDs := make([]string, 0, len(r.learners))
	for id := range r.learners {
		learnerIDs = append(learnerIDs, id)
	}
	sort.Strings(learnerIDs)
	for _, id := range learnerIDs {
		participant, ok := r.learners[id].(agent.SnapshotParticipant)
		if !ok {
			if mode == SnapshotBestEffort {
				state.Learners = append(state.Learners, agent.RuntimeSpecialistSnapshot{ID: id, StateKind: "opaque", Reason: "learner has no snapshot contract"})
				continue
			}
			return nil, fmt.Errorf("exact snapshot refused: learner %q has no snapshot contract", id)
		}
		data, err := participant.SnapshotRuntimeState()
		if err != nil {
			if mode == SnapshotBestEffort {
				state.Learners = append(state.Learners, agent.RuntimeSpecialistSnapshot{ID: id, StateKind: "opaque", Reason: err.Error()})
				continue
			}
			return nil, fmt.Errorf("snapshot learner %q: %w", id, err)
		}
		state.Learners = append(state.Learners, agent.RuntimeSpecialistSnapshot{ID: id, StateKind: participant.RuntimeStateKind(), State: append(json.RawMessage(nil), data...), Deterministic: true})
	}
	if r.memory != nil {
		snapshot, err := r.memory.SnapshotState()
		if err != nil {
			return nil, fmt.Errorf("snapshot attributed memory: %w", err)
		}
		state.Memory = &snapshot
	}
	if r.regulator != nil {
		snapshot := r.regulator.SnapshotState()
		state.Regulation = &snapshot
	}
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("encode organism state: %w", err)
	}
	digest := sha256.Sum256(stateBytes)
	encoded, err := json.Marshal(runtimeSnapshotEnvelope{
		SchemaVersion: runtimeSnapshotSchema, State: state, SHA256: hex.EncodeToString(digest[:]),
	})
	if err != nil {
		return nil, fmt.Errorf("encode organism snapshot envelope: %w", err)
	}
	return encoded, nil
}

// RestoreState restores into a newly constructed runtime with fresh stores and embedding capabilities.
func (r *Runtime) RestoreState(ctx context.Context, raw json.RawMessage, branchID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) != 0 || r.step != 0 || len(r.conditions) != 0 {
		return errors.New("runtime must be fresh before snapshot restoration")
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return errors.New("organism snapshot is malformed")
	}
	var envelope runtimeSnapshotEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode organism snapshot: %w", err)
	}
	if envelope.SchemaVersion != runtimeSnapshotSchema {
		return fmt.Errorf("incompatible organism snapshot schema %q", envelope.SchemaVersion)
	}
	stateBytes, err := json.Marshal(envelope.State)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(stateBytes)
	if envelope.SHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("organism snapshot checksum does not match its state")
	}
	state := envelope.State
	if state.OrganismID != r.organismID {
		return errors.New("organism snapshot identity does not match runtime")
	}
	if state.Workspace != r.workspace || state.DeferredExecution != r.deferredExecution {
		return errors.New("organism snapshot configuration does not match runtime")
	}
	if len(state.Specialists) != len(r.specialists) {
		return errors.New("organism snapshot is missing required specialist state")
	}
	if state.Mode != SnapshotExact || !state.Restorability.Exact {
		return errors.New("organism snapshot is not an exact checkpoint")
	}
	currentRestorability, err := json.Marshal(r.runtimeRestorability())
	if err != nil {
		return fmt.Errorf("encode runtime restorability report: %w", err)
	}
	snapshotRestorability, err := json.Marshal(state.Restorability)
	if err != nil || !bytes.Equal(currentRestorability, snapshotRestorability) {
		return errors.New("organism snapshot component inventory does not match runtime")
	}
	if len(state.Learners) != len(r.learners) {
		return errors.New("organism snapshot is missing required learner state")
	}
	conditions := make(map[string]float64, len(state.Conditions))
	for _, condition := range state.Conditions {
		if strings.TrimSpace(condition.Name) == "" || math.IsNaN(condition.Value) || math.IsInf(condition.Value, 0) {
			return errors.New("organism snapshot contains an invalid condition")
		}
		if _, exists := conditions[condition.Name]; exists {
			return errors.New("organism snapshot contains duplicate conditions")
		}
		conditions[condition.Name] = condition.Value
	}
	if (r.memory == nil) != (state.Memory == nil) || (r.regulator == nil) != (state.Regulation == nil) {
		return errors.New("organism snapshot is missing configured memory or regulation state")
	}
	if state.Events.OrganismID != r.organismID || state.Events.SchemaVersion != "arachne.event_snapshot.v1" {
		return errors.New("organism snapshot is missing compatible event history")
	}
	for i, specialist := range r.specialists {
		if state.Specialists[i].ID != specialist.ID() {
			return fmt.Errorf("organism snapshot specialist %d does not match runtime", i)
		}
	}
	for _, snapshot := range state.Learners {
		learner, ok := r.learners[snapshot.ID]
		if !ok {
			return fmt.Errorf("organism snapshot names unknown learner %q", snapshot.ID)
		}
		participant, ok := learner.(agent.SnapshotParticipant)
		if !ok || participant.RuntimeStateKind() != snapshot.StateKind {
			return fmt.Errorf("learner %q does not match its snapshot contract", snapshot.ID)
		}
		if err := participant.RestoreRuntimeState(append(json.RawMessage(nil), snapshot.State...)); err != nil {
			return fmt.Errorf("restore learner %q: %w", snapshot.ID, err)
		}
	}
	if r.memory != nil {
		if err := r.memory.RestoreState(ctx, *state.Memory); err != nil {
			return fmt.Errorf("restore attributed memory: %w", err)
		}
	}
	if r.regulator != nil {
		if err := r.regulator.RestoreState(*state.Regulation); err != nil {
			return fmt.Errorf("restore regulation state: %w", err)
		}
	}
	for i, specialist := range r.specialists {
		if err := specialist.RestoreState(state.Specialists[i]); err != nil {
			return fmt.Errorf("restore specialist %q: %w", specialist.ID(), err)
		}
	}
	if err := r.events.Restore(ctx, state.Events, branchID); err != nil {
		return fmt.Errorf("restore event history: %w", err)
	}
	r.conditions, r.step = conditions, state.Step
	return nil
}

// Determinism reports the runtime's current replay classification.
func (r *Runtime) Determinism() DeterminismStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.determinismStatus()
}

// DeferredExecution reports whether the embedding must report the environment consequence.
func (r *Runtime) DeferredExecution() bool { return r.deferredExecution }

func (r *Runtime) determinismStatus() DeterminismStatus {
	reasons := append([]string(nil), r.nondeterminismReasons...)
	if !r.deterministic {
		reasons = append(reasons, "embedding dependencies are not declared deterministic")
	}
	for _, specialist := range r.specialists {
		snapshot, err := specialist.SnapshotState()
		if err != nil {
			reasons = append(reasons, err.Error())
			continue
		}
		if !snapshot.Deterministic {
			reason := snapshot.Reason
			if reason == "" {
				reason = "specialist is not declared deterministic"
			}
			reasons = append(reasons, specialist.ID()+": "+reason)
		}
	}
	for _, component := range r.runtimeRestorability().Components {
		if component.Restorability == RestorabilityOpaque {
			reasons = append(reasons, component.ID+": "+component.Reason)
		}
	}
	reasons = unique(reasons)
	sort.Strings(reasons)
	return DeterminismStatus{Deterministic: len(reasons) == 0, Reasons: reasons}
}

// CompleteOutcome records a deferred environment consequence and then runs PR3 feedback.
func (r *Runtime) CompleteOutcome(ctx context.Context, interactionID string, outcome Outcome) (StepResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending, exists := r.pending[interactionID]
	if !exists {
		return StepResult{}, fmt.Errorf("interaction %q has no pending committed action", interactionID)
	}
	if outcome.Status == "" || outcome.Status == "pending" {
		return pending.result, errors.New("reported outcome status is required")
	}
	result := cloneStepResult(pending.result)
	outcome.EventID = ""
	if err := r.recordOutcome(ctx, &result, &outcome, result.Events.Action); err != nil {
		return result, err
	}
	result.Outcome = &outcome
	delete(r.pending, interactionID)
	if err := r.applyOutcomeFeedback(ctx, pending.perception, &result); err != nil {
		return result, err
	}
	return result, nil
}

func stableProposalID(interactionID, specialistID string) string {
	return deterministicStateID("proposal", interactionID, specialistID)
}

func deterministicStateID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, ":")))
	return hex.EncodeToString(digest[:16])
}

func clonePerception(value Perception) Perception {
	value.Input = append(json.RawMessage(nil), value.Input...)
	value.Features = append([]float64(nil), value.Features...)
	value.SourceEventIDs = append([]string(nil), value.SourceEventIDs...)
	return value
}

func cloneStepResult(value StepResult) StepResult {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var clone StepResult
	if json.Unmarshal(encoded, &clone) != nil {
		return value
	}
	return clone
}
