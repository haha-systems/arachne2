package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// StateSnapshot is a stable, deep plain-data copy of all mutable memory records.
type StateSnapshot struct {
	SchemaVersion string                `json:"schema_version"`
	OrganismID    string                `json:"organism_id"`
	Episodes      []Episode             `json:"episodes"`
	Semantics     []SemanticRecord      `json:"semantics"`
	Runs          []ConsolidationRun    `json:"consolidation_runs"`
	Patterns      []ConsolidatedPattern `json:"consolidated_patterns"`
}

// SnapshotState exports every record in deterministic ID order.
func (s *InMemory) SnapshotState() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := StateSnapshot{
		SchemaVersion: "arachne.memory_state.v1",
		OrganismID:    s.database.OrganismID,
		Episodes:      make([]Episode, 0, len(s.database.Episodes)),
		Semantics:     make([]SemanticRecord, 0, len(s.database.Semantics)),
		Runs:          make([]ConsolidationRun, 0, len(s.database.Runs)),
		Patterns:      make([]ConsolidatedPattern, 0, len(s.database.Patterns)),
	}
	for _, value := range s.database.Episodes {
		snapshot.Episodes = append(snapshot.Episodes, cloneEpisode(value))
	}
	for _, value := range s.database.Semantics {
		snapshot.Semantics = append(snapshot.Semantics, cloneSemantic(value))
	}
	for _, value := range s.database.Runs {
		snapshot.Runs = append(snapshot.Runs, cloneRun(value))
	}
	for _, value := range s.database.Patterns {
		snapshot.Patterns = append(snapshot.Patterns, clonePattern(value))
	}
	sort.Slice(snapshot.Episodes, func(i, j int) bool { return snapshot.Episodes[i].ID < snapshot.Episodes[j].ID })
	sort.Slice(snapshot.Semantics, func(i, j int) bool { return snapshot.Semantics[i].ID < snapshot.Semantics[j].ID })
	sort.Slice(snapshot.Runs, func(i, j int) bool { return snapshot.Runs[i].ID < snapshot.Runs[j].ID })
	sort.Slice(snapshot.Patterns, func(i, j int) bool { return snapshot.Patterns[i].ID < snapshot.Patterns[j].ID })
	return snapshot
}

// RestoreState imports a complete snapshot only into an empty store of the same organism.
func (s *InMemory) RestoreState(ctx context.Context, snapshot StateSnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if snapshot.SchemaVersion != "arachne.memory_state.v1" || snapshot.OrganismID != s.database.OrganismID {
		return errors.New("memory snapshot schema or organism identity is incompatible")
	}
	next := database{
		SchemaVersion: "arachne.memory.v1", OrganismID: snapshot.OrganismID,
		Episodes:  make(map[string]Episode, len(snapshot.Episodes)),
		Semantics: make(map[string]SemanticRecord, len(snapshot.Semantics)),
		Runs:      make(map[string]ConsolidationRun, len(snapshot.Runs)),
		Patterns:  make(map[string]ConsolidatedPattern, len(snapshot.Patterns)),
	}
	for _, value := range snapshot.Episodes {
		if err := validateEpisode(next.OrganismID, value); err != nil {
			return fmt.Errorf("validate memory episode %q: %w", value.ID, err)
		}
		if _, exists := next.Episodes[value.ID]; exists {
			return ErrDuplicateID
		}
		next.Episodes[value.ID] = cloneEpisode(value)
	}
	for _, value := range snapshot.Semantics {
		if err := validateSemantic(next.OrganismID, value); err != nil {
			return fmt.Errorf("validate semantic record %q: %w", value.ID, err)
		}
		if _, exists := next.Semantics[value.ID]; exists {
			return ErrDuplicateID
		}
		next.Semantics[value.ID] = cloneSemantic(value)
	}
	for _, value := range snapshot.Runs {
		if value.ID == "" || value.OrganismID != next.OrganismID {
			return fmt.Errorf("validate consolidation %q: incompatible identity", value.ID)
		}
		if _, exists := next.Runs[value.ID]; exists {
			return ErrDuplicateID
		}
		next.Runs[value.ID] = cloneRun(value)
	}
	for _, value := range snapshot.Patterns {
		if err := validatePatternSnapshot(next, value); err != nil {
			return err
		}
		if _, exists := next.Patterns[value.ID]; exists {
			return ErrDuplicateID
		}
		next.Patterns[value.ID] = clonePattern(value)
	}
	for _, run := range next.Runs {
		patterns := make([]ConsolidatedPattern, 0, len(run.PatternIDs))
		for _, id := range run.PatternIDs {
			pattern, exists := next.Patterns[id]
			if !exists {
				return fmt.Errorf("consolidation %q references missing pattern %q", run.ID, id)
			}
			patterns = append(patterns, pattern)
		}
		if err := validateConsolidation(next.OrganismID, run, patterns); err != nil {
			return fmt.Errorf("validate consolidation %q: %w", run.ID, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.database.Episodes)+len(s.database.Semantics)+len(s.database.Runs)+len(s.database.Patterns) != 0 {
		return errors.New("memory store must be empty before restore")
	}
	return s.commit(next)
}

func validatePatternSnapshot(db database, pattern ConsolidatedPattern) error {
	if pattern.ID == "" || pattern.OrganismID != db.OrganismID {
		return fmt.Errorf("consolidated pattern %q has incompatible organism identity", pattern.ID)
	}
	if _, exists := db.Runs[pattern.RunID]; !exists {
		return fmt.Errorf("consolidated pattern %q references missing run %q", pattern.ID, pattern.RunID)
	}
	return nil
}

// SnapshotState returns the store's complete state without exposing its implementation.
func (s *Service) SnapshotState() (StateSnapshot, error) {
	store, ok := s.store.(interface{ SnapshotState() StateSnapshot })
	if !ok {
		return StateSnapshot{}, errors.New("memory store does not support snapshots")
	}
	return store.SnapshotState(), nil
}

// RestoreState imports complete state into an empty memory store.
func (s *Service) RestoreState(ctx context.Context, snapshot StateSnapshot) error {
	store, ok := s.store.(interface {
		RestoreState(context.Context, StateSnapshot) error
	})
	if !ok {
		return errors.New("memory store does not support snapshot restore")
	}
	if snapshot.OrganismID != s.organismID {
		return errors.New("memory snapshot organism identity does not match service")
	}
	return store.RestoreState(ctx, snapshot)
}
