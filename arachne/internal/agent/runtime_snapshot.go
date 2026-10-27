package agent

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ID returns the stable identity assigned to this specialist.
func (a *SpecialistAgent) ID() string { return a.id }

// RuntimeSpecialistSnapshot stores one specialist's explicit snapshot contract.
type RuntimeSpecialistSnapshot struct {
	ID            string          `json:"id"`
	StateKind     string          `json:"state_kind"`
	State         json.RawMessage `json:"state"`
	Deterministic bool            `json:"deterministic"`
	Reason        string          `json:"reason,omitempty"`
}

// StatelessSpecialist explicitly promises that all future behavior is in its immutable configuration.
type StatelessSpecialist interface {
	Stateless() bool
}

// SnapshotState returns plain state and a reproducibility declaration for this specialist.
func (a *SpecialistAgent) SnapshotState() (RuntimeSpecialistSnapshot, error) {
	snapshot := RuntimeSpecialistSnapshot{ID: a.id}
	if participant, ok := a.implementation.(SnapshotParticipant); ok {
		if participant.RuntimeStateKind() == "" {
			return RuntimeSpecialistSnapshot{}, errors.New("snapshot participant has no stable state kind")
		}
		state, err := participant.SnapshotRuntimeState()
		if err != nil {
			return RuntimeSpecialistSnapshot{}, fmt.Errorf("snapshot specialist %q: %w", a.id, err)
		}
		if len(state) == 0 || !json.Valid(state) {
			return RuntimeSpecialistSnapshot{}, fmt.Errorf("specialist %q returned invalid snapshot state", a.id)
		}
		snapshot.StateKind, snapshot.State = participant.RuntimeStateKind(), append(json.RawMessage(nil), state...)
		if reporter, ok := a.implementation.(DeterminismReporter); ok {
			snapshot.Deterministic, snapshot.Reason = reporter.Deterministic()
		} else {
			snapshot.Reason = "specialist does not declare deterministic inference"
		}
		return snapshot, nil
	}
	if stateless, ok := a.implementation.(StatelessSpecialist); ok && stateless.Stateless() {
		snapshot.StateKind, snapshot.State, snapshot.Deterministic = "arachne.stateless.v1", json.RawMessage("{}"), true
		return snapshot, nil
	}
	return RuntimeSpecialistSnapshot{}, fmt.Errorf("specialist %q has mutable or opaque state without a snapshot contract", a.id)
}

// RestoreState validates the specialist identity and restores its explicit state.
func (a *SpecialistAgent) RestoreState(snapshot RuntimeSpecialistSnapshot) error {
	if snapshot.ID != a.id || snapshot.StateKind == "" || len(snapshot.State) == 0 || !json.Valid(snapshot.State) {
		return errors.New("specialist snapshot identity or state is invalid")
	}
	if snapshot.StateKind == "arachne.stateless.v1" {
		stateless, ok := a.implementation.(StatelessSpecialist)
		if !ok || !stateless.Stateless() || string(snapshot.State) != "{}" {
			return errors.New("stateless specialist configuration does not match snapshot")
		}
		return nil
	}
	participant, ok := a.implementation.(SnapshotParticipant)
	if !ok || participant.RuntimeStateKind() != snapshot.StateKind {
		return errors.New("specialist snapshot type does not match configured implementation")
	}
	return participant.RestoreRuntimeState(append(json.RawMessage(nil), snapshot.State...))
}
