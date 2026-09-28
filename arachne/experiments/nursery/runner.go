package nursery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// CognitiveTrace carries observations from existing Arachne subsystems without feeding
// instrumentation-only fields back into the next perception.
type CognitiveTrace struct {
	Events               []json.RawMessage `json:"events,omitempty"`
	ActionSourceEventIDs []string          `json:"action_source_event_ids,omitempty"`
	Workspace            json.RawMessage   `json:"workspace,omitempty"`
	MemoryReads          []string          `json:"memory_reads,omitempty"`
	MemoryWrites         []string          `json:"memory_writes,omitempty"`
	ReplayEventIDs       []string          `json:"replay_event_ids,omitempty"`
	Proposals            []json.RawMessage `json:"proposals,omitempty"`
	ModelInput           json.RawMessage   `json:"model_input,omitempty"`
	ModelOutput          json.RawMessage   `json:"model_output,omitempty"`
	ProviderStateRef     string            `json:"provider_state_ref,omitempty"`
}

// Organism is an experiment boundary for one continuous Arachne organism.
// An implementation must preserve its complete cognitive state in SnapshotState.
type Organism interface {
	Decide(context.Context, Perception) (Action, CognitiveTrace, error)
	SnapshotState(context.Context) (json.RawMessage, error)
	RestoreState(context.Context, json.RawMessage) error
}

// Teacher is an inactive-by-default interface for later curriculum experiments.
type Teacher interface {
	Intervene(context.Context, uint64, Perception) (json.RawMessage, error)
}

// TeacherFunc adapts a function to the optional future-experiment teacher interface.
type TeacherFunc func(context.Context, uint64, Perception) (json.RawMessage, error)

// Intervene implements Teacher.
func (f TeacherFunc) Intervene(ctx context.Context, tick uint64, perception Perception) (json.RawMessage, error) {
	return f(ctx, tick, perception)
}

// ProviderRecord describes inference reproducibility without exposing a provider as an action.
type ProviderRecord struct {
	Name                 string          `json:"name,omitempty"`
	Configuration        json.RawMessage `json:"configuration,omitempty"`
	State                json.RawMessage `json:"state,omitempty"`
	Deterministic        bool            `json:"deterministic"`
	NondeterminismReason string          `json:"nondeterminism_reason,omitempty"`
}

// ExperimentConfig contains declarative world and run settings only.
type ExperimentConfig struct {
	Name          string         `json:"name"`
	World         Config         `json:"world"`
	Provider      ProviderRecord `json:"provider"`
	TeacherActive bool           `json:"teacher_active"`
}

// TickRecord contains reconstructable evidence for one full environment cycle.
type TickRecord struct {
	Tick            uint64          `json:"tick"`
	WorldBefore     json.RawMessage `json:"world_before"`
	Perception      Perception      `json:"perception"`
	Cognition       CognitiveTrace  `json:"cognition"`
	Action          Action          `json:"action"`
	Changes         []Change        `json:"changes"`
	ViabilityBefore Viability       `json:"viability_before"`
	ViabilityAfter  Viability       `json:"viability_after"`
	WorldAfter      json.RawMessage `json:"world_after"`
	HistoryDigest   string          `json:"history_digest"`
}

// Session is one continuous world and organism life history.
type Session struct {
	Config             ExperimentConfig `json:"config"`
	World              *World           `json:"world"`
	Organism           Organism         `json:"-"`
	Teacher            Teacher          `json:"-"`
	History            []TickRecord     `json:"history"`
	BranchID           string           `json:"branch_id"`
	ParentCheckpointID string           `json:"parent_checkpoint_id,omitempty"`
	Interventions      []Intervention   `json:"interventions,omitempty"`
}

// NewSession creates one seeded world for one organism instance.
func NewSession(_ context.Context, config ExperimentConfig, organism Organism) (*Session, error) {
	if organism == nil {
		return nil, errors.New("one organism adapter is required")
	}
	if config.Name == "" {
		config.Name = "experiment-002-nursery"
	}
	config.Provider = cloneProvider(config.Provider)
	if !config.Provider.Deterministic && config.Provider.NondeterminismReason == "" {
		config.Provider.NondeterminismReason = "provider inference is not declared deterministic"
	}
	world, err := NewWorld(config.World)
	if err != nil {
		return nil, err
	}
	config.World = world.Config
	return &Session{Config: config, World: world, Organism: organism, BranchID: "original"}, nil
}

// Step advances one tick, preserving the world and the same organism instance.
func (s *Session) Step(ctx context.Context) (TickRecord, error) {
	if s == nil || s.World == nil || s.Organism == nil {
		return TickRecord{}, errors.New("nursery session is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return TickRecord{}, err
	}
	before, err := json.Marshal(s.World)
	if err != nil {
		return TickRecord{}, fmt.Errorf("snapshot pre-tick world: %w", err)
	}
	perception := s.World.Perceive()
	if s.Config.TeacherActive {
		if s.Teacher == nil {
			return TickRecord{}, errors.New("teacher is active but no teacher interface is installed")
		}
		signal, err := s.Teacher.Intervene(ctx, s.World.Tick, perception)
		if err != nil {
			return TickRecord{}, fmt.Errorf("teacher intervention at tick %d: %w", s.World.Tick, err)
		}
		if len(signal) > 0 && !json.Valid(signal) {
			return TickRecord{}, errors.New("teacher signal must be valid JSON")
		}
		perception.TeacherSignal = append(json.RawMessage(nil), signal...)
	}
	viabilityBefore := s.World.Viability
	action, cognition, err := s.Organism.Decide(ctx, perception)
	if err != nil {
		return TickRecord{}, fmt.Errorf("organism decision at tick %d: %w", s.World.Tick, err)
	}
	if err := ctx.Err(); err != nil {
		return TickRecord{}, err
	}
	changes, err := s.World.Apply(action)
	if err != nil {
		return TickRecord{}, fmt.Errorf("apply environment action at tick %d: %w", s.World.Tick, err)
	}
	if len(changes) > 0 {
		for _, parent := range cognition.ActionSourceEventIDs {
			if parent == "" {
				continue
			}
			changes[0].ParentEventIDs = appendUnique(changes[0].ParentEventIDs, parent)
		}
		s.World.Recent = cloneChanges(changes)
	}
	after, err := json.Marshal(s.World)
	if err != nil {
		return TickRecord{}, fmt.Errorf("snapshot post-tick world: %w", err)
	}
	record := TickRecord{
		Tick: s.World.Tick, WorldBefore: before, Perception: perception, Cognition: cloneCognition(cognition),
		Action: action, Changes: changes, ViabilityBefore: viabilityBefore,
		ViabilityAfter: s.World.Viability, WorldAfter: after,
	}
	previousDigest := ""
	if len(s.History) > 0 {
		previousDigest = s.History[len(s.History)-1].HistoryDigest
	}
	record.HistoryDigest, err = historyDigest(previousDigest, record)
	if err != nil {
		return TickRecord{}, fmt.Errorf("encode tick history digest: %w", err)
	}
	s.History = append(s.History, cloneTick(record))
	return cloneTick(record), nil
}

// Run advances up to count ticks, stopping on cancellation or an organism error.
func (s *Session) Run(ctx context.Context, count int) ([]TickRecord, error) {
	if count < 0 {
		return nil, errors.New("tick count must not be negative")
	}
	result := make([]TickRecord, 0, count)
	for range count {
		record, err := s.Step(ctx)
		if err != nil {
			return result, err
		}
		result = append(result, record)
	}
	return result, nil
}

func cloneCognition(trace CognitiveTrace) CognitiveTrace {
	clone := trace
	clone.Events = cloneRawMessages(trace.Events)
	clone.Proposals = cloneRawMessages(trace.Proposals)
	clone.ModelInput = append(json.RawMessage(nil), trace.ModelInput...)
	clone.ModelOutput = append(json.RawMessage(nil), trace.ModelOutput...)
	clone.MemoryReads = append([]string(nil), trace.MemoryReads...)
	clone.MemoryWrites = append([]string(nil), trace.MemoryWrites...)
	clone.ReplayEventIDs = append([]string(nil), trace.ReplayEventIDs...)
	clone.ActionSourceEventIDs = append([]string(nil), trace.ActionSourceEventIDs...)
	return clone
}

func cloneRawMessages(values []json.RawMessage) []json.RawMessage {
	clone := make([]json.RawMessage, len(values))
	for index := range values {
		clone[index] = append(json.RawMessage(nil), values[index]...)
	}
	return clone
}

func cloneTick(record TickRecord) TickRecord {
	clone := record
	clone.WorldBefore = append(json.RawMessage(nil), record.WorldBefore...)
	clone.WorldAfter = append(json.RawMessage(nil), record.WorldAfter...)
	clone.Changes = cloneChanges(record.Changes)
	clone.Perception.Cells = append([]Cell(nil), record.Perception.Cells...)
	clone.Perception.Entities = append([]EntityView(nil), record.Perception.Entities...)
	clone.Perception.Inventory = append([]string(nil), record.Perception.Inventory...)
	clone.Perception.RecentChanges = cloneChanges(record.Perception.RecentChanges)
	clone.Perception.TeacherSignal = append(json.RawMessage(nil), record.Perception.TeacherSignal...)
	clone.Cognition = cloneCognition(record.Cognition)
	return clone
}

func historyDigest(previous string, record TickRecord) (string, error) {
	record.HistoryDigest = ""
	digestInput, err := json.Marshal(struct {
		Previous string     `json:"previous"`
		Record   TickRecord `json:"record"`
	}{Previous: previous, Record: record})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(digestInput)
	return hex.EncodeToString(digest[:]), nil
}
