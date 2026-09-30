package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
)

// SnapshotParticipant is a specialist with explicit, versioned plain-data state.
type SnapshotParticipant interface {
	RuntimeStateKind() string
	SnapshotRuntimeState() (json.RawMessage, error)
	RestoreRuntimeState(json.RawMessage) error
}

// DeterminismReporter declares whether a specialist's future proposals are reproducible.
type DeterminismReporter interface {
	Deterministic() (bool, string)
}

// RuntimeStateKind identifies the stable state schema used by runtime snapshots.
func (*LearnableSpecialist) RuntimeStateKind() string { return "arachne.learnable_specialist.v1" }

// SnapshotRuntimeState encodes the learnable model's complete state.
func (s *LearnableSpecialist) SnapshotRuntimeState() (json.RawMessage, error) {
	encoded, err := json.Marshal(s.Snapshot())
	if err != nil {
		return nil, fmt.Errorf("encode learnable specialist snapshot: %w", err)
	}
	return encoded, nil
}

// RestoreRuntimeState decodes and validates complete learnable model state.
func (s *LearnableSpecialist) RestoreRuntimeState(raw json.RawMessage) error {
	var snapshot LearnableSpecialistSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return fmt.Errorf("decode learnable specialist snapshot: %w", err)
	}
	return s.RestoreSnapshot(snapshot)
}

// Deterministic reports that the learnable model has no external inference dependency.
func (*LearnableSpecialist) Deterministic() (bool, string) { return true, "" }

// RestoreSnapshot restores model parameters and RNG state into the same configured specialist.
func (s *LearnableSpecialist) RestoreSnapshot(snapshot LearnableSpecialistSnapshot) error {
	config, err := normalizeLearnableConfig(snapshot.Config)
	if err != nil {
		return fmt.Errorf("validate learnable specialist configuration: %w", err)
	}
	if !reflect.DeepEqual(config, s.config) {
		return errors.New("learnable specialist configuration does not match snapshot")
	}
	if snapshot.RNGState == 0 || len(snapshot.Parameters) != len(s.actions) {
		return errors.New("learnable specialist snapshot is missing required model state")
	}
	byKind := make(map[string]LearnableActionParameters, len(snapshot.Parameters))
	for _, parameters := range snapshot.Parameters {
		if _, exists := byKind[parameters.Kind]; exists {
			return errors.New("learnable specialist snapshot has duplicate action parameters")
		}
		byKind[parameters.Kind] = parameters
	}
	restored := make([]actionHeads, len(s.actions))
	for i, current := range s.actions {
		parameters, exists := byKind[current.Kind]
		if !exists {
			return fmt.Errorf("learnable specialist snapshot is missing action %q", current.Kind)
		}
		score, err := restoreLinear(parameters.Score, s.config.FeatureDimension)
		if err != nil {
			return fmt.Errorf("restore score head for %q: %w", current.Kind, err)
		}
		restored[i] = actionHeads{Kind: current.Kind, Score: score, Parameters: make(map[string]linearHead, len(current.Parameters))}
		named := make(map[string]LinearParameters, len(parameters.Parameters))
		for _, value := range parameters.Parameters {
			if _, exists := named[value.Name]; exists {
				return fmt.Errorf("action %q has duplicate parameter head %q", current.Kind, value.Name)
			}
			named[value.Name] = value.Linear
		}
		if len(named) != len(current.Parameters) {
			return fmt.Errorf("action %q is missing required parameter heads", current.Kind)
		}
		for name := range current.Parameters {
			value, exists := named[name]
			if !exists {
				return fmt.Errorf("action %q is missing parameter head %q", current.Kind, name)
			}
			head, err := restoreLinear(value, s.config.FeatureDimension)
			if err != nil {
				return fmt.Errorf("restore parameter head %q for %q: %w", name, current.Kind, err)
			}
			restored[i].Parameters[name] = head
		}
	}
	if snapshot.LastUpdate != nil {
		last := snapshot.LastUpdate
		if last.Index == 0 || last.Index != snapshot.UpdateCount || last.ActionKind == "" ||
			!finiteScore(last.TargetScore) || !finiteScore(last.PredictedScore) ||
			len(last.Features) != s.config.FeatureDimension {
			return errors.New("learnable specialist snapshot has invalid last update")
		}
		for _, feature := range last.Features {
			if math.IsNaN(feature) || math.IsInf(feature, 0) {
				return errors.New("learnable specialist snapshot has invalid update feature")
			}
		}
	} else if snapshot.UpdateCount != 0 {
		return errors.New("learnable specialist snapshot is missing its last update")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions, s.rngState = restored, snapshot.RNGState
	s.updateCount = snapshot.UpdateCount
	if snapshot.LastUpdate == nil {
		s.lastUpdate = nil
	} else {
		last := cloneUpdate(*snapshot.LastUpdate)
		s.lastUpdate = &last
	}
	return nil
}

func restoreLinear(value LinearParameters, dimension int) (linearHead, error) {
	if !isFinite(value.Bias) || len(value.Weights) != dimension {
		return linearHead{}, errors.New("linear parameters have invalid shape or bias")
	}
	weights := append([]float64(nil), value.Weights...)
	for _, weight := range weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) {
			return linearHead{}, errors.New("linear parameter is not finite")
		}
	}
	return linearHead{Bias: value.Bias, Weights: weights}, nil
}

func finiteScore(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}
