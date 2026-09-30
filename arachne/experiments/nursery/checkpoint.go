package nursery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const checkpointSchema = "arachne.nursery.checkpoint.v2"

// Checkpoint is a complete branch point for the world and one organism history.
type Checkpoint struct {
	Schema           string            `json:"schema"`
	ID               string            `json:"id"`
	BranchID         string            `json:"branch_id"`
	ParentID         string            `json:"parent_id,omitempty"`
	Tick             uint64            `json:"tick"`
	World            World             `json:"world"`
	OrganismState    json.RawMessage   `json:"organism_state"`
	Provider         ProviderRecord    `json:"provider"`
	Deterministic    *bool             `json:"deterministic,omitempty"`
	Experiment       ExperimentConfig  `json:"experiment"`
	History          []TickRecord      `json:"history"`
	EventHistoryRef  string            `json:"event_history_ref"`
	Nondeterminism   []string          `json:"nondeterminism,omitempty"`
	Mode             string            `json:"mode"`
	ExactRestoration bool              `json:"exact_restoration"`
	Restorability    RestorationReport `json:"restorability"`
	Interventions    []Intervention    `json:"interventions,omitempty"`
}

// Intervention records one explicit fork edit for later causal review.
type Intervention struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Description string `json:"description"`
}

// CheckpointEditor makes a controlled edit to a fork before its organism state is restored.
type CheckpointEditor func(context.Context, *Checkpoint) error

// CreateCheckpoint captures an exact branch point or fails when any component is opaque.
func (s *Session) CreateCheckpoint(ctx context.Context) (Checkpoint, error) {
	return s.createCheckpoint(ctx, "exact")
}

// CreateBestEffortCheckpoint records available state without promising exact continuation.
func (s *Session) CreateBestEffortCheckpoint(ctx context.Context) (Checkpoint, error) {
	return s.createCheckpoint(ctx, "best-effort")
}

func (s *Session) createCheckpoint(ctx context.Context, mode string) (Checkpoint, error) {
	if s == nil || s.World == nil || s.Organism == nil {
		return Checkpoint{}, errors.New("nursery session is incomplete")
	}
	restorability := RestorationReport{Exact: false, Components: []RestorationComponent{{ID: "organism", Kind: "runtime", Restorability: "opaque", Reason: "organism does not report its component inventory"}}}
	if reporter, ok := s.Organism.(RestorationReporter); ok {
		restorability = reporter.RestorationReport()
	}
	if mode == "exact" && !restorability.Exact {
		return Checkpoint{}, errors.New("exact checkpoint refused: organism reports opaque or unknown component state")
	}
	var organismState json.RawMessage
	var err error
	if mode == "best-effort" {
		if snapshotter, ok := s.Organism.(BestEffortSnapshotter); ok {
			organismState, err = snapshotter.SnapshotBestEffortState(ctx)
		} else {
			return Checkpoint{}, errors.New("best-effort checkpoint requires an explicit best-effort snapshot API")
		}
	} else {
		organismState, err = s.Organism.SnapshotState(ctx)
	}
	if err != nil {
		return Checkpoint{}, fmt.Errorf("snapshot organism state: %w", err)
	}
	if len(organismState) == 0 || !json.Valid(organismState) {
		return Checkpoint{}, errors.New("organism state snapshot must be valid JSON")
	}
	history := make([]TickRecord, len(s.History))
	for index := range s.History {
		history[index] = cloneTick(s.History[index])
	}
	historyBytes, err := json.Marshal(history)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("encode event history reference: %w", err)
	}
	historyDigest := sha256.Sum256(historyBytes)
	provider := checkpointProvider(s.Config.Provider)
	experiment := cloneExperiment(s.Config)
	experiment.Provider = provider
	checkpoint := Checkpoint{
		Schema: checkpointSchema, Tick: s.World.Tick, World: *cloneWorld(s.World),
		OrganismState: append(json.RawMessage(nil), organismState...),
		Provider:      provider, Experiment: experiment,
		History: history, EventHistoryRef: hex.EncodeToString(historyDigest[:]),
		BranchID: s.BranchID, ParentID: s.ParentCheckpointID,
		Mode: mode, ExactRestoration: mode == "exact" && restorability.Exact, Restorability: restorability,
		Interventions: append([]Intervention(nil), s.Interventions...),
	}
	reproducibility := Reproducibility{Deterministic: false, Reasons: []string{"organism does not declare deterministic continuation"}}
	if reporter, ok := s.Organism.(ReproducibilityReporter); ok {
		reproducibility = reporter.Reproducibility()
	}
	if !s.Config.Provider.Deterministic {
		reason := s.Config.Provider.NondeterminismReason
		if reason == "" {
			reason = "provider inference is not declared deterministic"
		}
		reproducibility.Deterministic = false
		reproducibility.Reasons = append(reproducibility.Reasons, reason)
	}
	if len(s.Config.Provider.Configuration) > 0 || len(s.Config.Provider.State) > 0 {
		reproducibility.Deterministic = false
		reproducibility.Reasons = append(reproducibility.Reasons, "opaque provider configuration/state was omitted from checkpoint")
	}
	deterministic := reproducibility.Deterministic && len(reproducibility.Reasons) == 0
	checkpoint.Deterministic = &deterministic
	for _, component := range restorability.Components {
		if component.Restorability == "opaque" {
			reproducibility.Reasons = append(reproducibility.Reasons, component.ID+": "+component.Reason)
		}
	}
	checkpoint.Nondeterminism = sortedUnique(reproducibility.Reasons)
	checkpoint.ID = checkpointID(checkpoint)
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return Checkpoint{}, fmt.Errorf("validate captured checkpoint: %w", err)
	}
	return checkpoint, nil
}

// Fork restores a checkpoint into an independent branch with the same world and cognitive state.
func Fork(ctx context.Context, checkpoint Checkpoint, organism Organism, branchID string) (*Session, error) {
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return nil, err
	}
	if checkpoint.Mode != "exact" || !checkpoint.ExactRestoration {
		return nil, errors.New("exact fork refused: checkpoint is best-effort or lacks exact-restoration proof")
	}
	if organism == nil || branchID == "" {
		return nil, errors.New("organism adapter and branch ID are required")
	}
	if err := organism.RestoreState(ctx, append(json.RawMessage(nil), checkpoint.OrganismState...)); err != nil {
		return nil, fmt.Errorf("restore organism state: %w", err)
	}
	config := checkpoint.Experiment
	config.Provider = cloneProvider(checkpoint.Provider)
	history := make([]TickRecord, len(checkpoint.History))
	for index := range checkpoint.History {
		history[index] = cloneTick(checkpoint.History[index])
	}
	return &Session{
		Config: config, World: cloneWorld(&checkpoint.World), Organism: organism,
		History: history, BranchID: branchID, ParentCheckpointID: checkpoint.ID,
		Interventions: append([]Intervention(nil), checkpoint.Interventions...),
	}, nil
}

// ForkWithIntervention edits a validated checkpoint, records the intervention, then restores a branch.
func ForkWithIntervention(ctx context.Context, checkpoint Checkpoint, organism Organism, branchID string, intervention Intervention, edit CheckpointEditor) (*Session, error) {
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return nil, err
	}
	if intervention.ID == "" || intervention.Kind == "" || intervention.Target == "" || edit == nil {
		return nil, errors.New("intervention ID, kind, target, and checkpoint editor are required")
	}
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, fmt.Errorf("copy checkpoint for intervention: %w", err)
	}
	var forked Checkpoint
	if err := json.Unmarshal(data, &forked); err != nil {
		return nil, fmt.Errorf("decode copied checkpoint: %w", err)
	}
	forked.ParentID = checkpoint.ID
	forked.BranchID = branchID
	forked.Interventions = append(forked.Interventions, intervention)
	if err := edit(ctx, &forked); err != nil {
		return nil, fmt.Errorf("apply checkpoint intervention %q: %w", intervention.ID, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	historyBytes, err := json.Marshal(forked.History)
	if err != nil {
		return nil, fmt.Errorf("encode fork event history: %w", err)
	}
	historyDigest := sha256.Sum256(historyBytes)
	forked.EventHistoryRef = hex.EncodeToString(historyDigest[:])
	forked.ID = checkpointID(forked)
	if err := ValidateCheckpoint(forked); err != nil {
		return nil, fmt.Errorf("intervened checkpoint is invalid: %w", err)
	}
	branch, err := Fork(ctx, forked, organism, branchID)
	if err != nil {
		return nil, err
	}
	branch.ParentCheckpointID = checkpoint.ID
	branch.Interventions = append([]Intervention(nil), forked.Interventions...)
	return branch, nil
}

func sortedUnique(values []string) []string {
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
	sort.Strings(result)
	return result
}

func validateCheckpointMode(checkpoint Checkpoint) error {
	if checkpoint.Mode != "exact" && checkpoint.Mode != "best-effort" {
		return errors.New("checkpoint mode must be exact or best-effort")
	}
	if checkpoint.ExactRestoration && (checkpoint.Mode != "exact" || !checkpoint.Restorability.Exact) {
		return errors.New("checkpoint exact-restoration metadata is inconsistent")
	}
	if checkpoint.Mode == "exact" && !checkpoint.ExactRestoration {
		return errors.New("exact checkpoint does not establish exact restoration")
	}
	return nil
}

// ValidateCheckpoint verifies schema, world state, and its event-history reference.
func ValidateCheckpoint(checkpoint Checkpoint) error {
	if err := validateCheckpointMode(checkpoint); err != nil {
		return err
	}
	if checkpoint.Schema != checkpointSchema || checkpoint.ID == "" || checkpoint.BranchID == "" {
		return errors.New("checkpoint schema, ID, and branch ID are required")
	}
	if checkpoint.Tick != checkpoint.World.Tick || checkpoint.Experiment.Name == "" ||
		checkpoint.Experiment.World != checkpoint.World.Config || checkpoint.Tick != uint64(len(checkpoint.History)) {
		return errors.New("checkpoint tick or experiment configuration is inconsistent")
	}
	if len(checkpoint.OrganismState) == 0 || !json.Valid(checkpoint.OrganismState) {
		return errors.New("checkpoint organism state is invalid")
	}
	if len(checkpoint.Provider.Configuration) > 0 || len(checkpoint.Provider.State) > 0 ||
		len(checkpoint.Experiment.Provider.Configuration) > 0 || len(checkpoint.Experiment.Provider.State) > 0 {
		return errors.New("checkpoint cannot contain opaque provider configuration or state")
	}
	if checkpoint.World.Config.Width < 6 || checkpoint.World.Config.Height < 6 ||
		len(checkpoint.World.Cells) != checkpoint.World.Config.Width*checkpoint.World.Config.Height ||
		!inside(checkpoint.World.Config, checkpoint.World.Position) {
		return errors.New("checkpoint world state is inconsistent")
	}
	if err := validateCheckpointHistory(checkpoint); err != nil {
		return err
	}
	if checkpoint.ID != checkpointID(checkpoint) {
		return errors.New("checkpoint content digest does not match its ID")
	}
	return nil
}

// SaveCheckpoint writes a checkpoint by atomic rename so partial files are not accepted.
func SaveCheckpoint(path string, checkpoint Checkpoint) error {
	if path == "" {
		return errors.New("checkpoint path is required")
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return err
	}
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, ".nursery-checkpoint-*.tmp")
	if err != nil {
		return fmt.Errorf("create checkpoint temporary file: %w", err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write checkpoint: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync checkpoint: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close checkpoint: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("install checkpoint: %w", err)
	}
	return nil
}

// LoadCheckpoint decodes and validates an exact checkpoint before returning it.
func LoadCheckpoint(path string) (Checkpoint, error) {
	// #nosec G304 -- the caller supplies a checkpoint path; organism-controlled paths are never used.
	data, err := os.ReadFile(path)
	if err != nil {
		return Checkpoint{}, err
	}
	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return Checkpoint{}, fmt.Errorf("decode checkpoint: %w", err)
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

func checkpointID(checkpoint Checkpoint) string {
	checkpoint.ID = ""
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func checkpointProvider(provider ProviderRecord) ProviderRecord {
	provider.Configuration = nil
	provider.State = nil
	return provider
}

func cloneProvider(provider ProviderRecord) ProviderRecord {
	provider.Configuration = append(json.RawMessage(nil), provider.Configuration...)
	provider.State = append(json.RawMessage(nil), provider.State...)
	return provider
}

func cloneExperiment(config ExperimentConfig) ExperimentConfig {
	config.Provider = cloneProvider(config.Provider)
	return config
}

func cloneWorld(world *World) *World {
	clone := *world
	clone.Cells = append([]Cell(nil), world.Cells...)
	clone.Entities = append([]entity(nil), world.Entities...)
	clone.Inventory = append([]string(nil), world.Inventory...)
	clone.Recent = cloneChanges(world.Recent)
	return &clone
}
