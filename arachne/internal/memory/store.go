package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotFound reports a memory ID that is not present in the store.
	ErrNotFound = errors.New("memory record not found")
	// ErrDuplicateID reports an attempt to overwrite a memory record.
	ErrDuplicateID = errors.New("memory record ID already exists")
)

// Store exposes explicit append and query operations for episodes and semantics.
type Store interface {
	PutEpisode(context.Context, Episode) error
	GetEpisode(context.Context, string) (Episode, error)
	QueryEpisodes(context.Context, EpisodeQuery) ([]Episode, error)
	PutSemantic(context.Context, SemanticRecord) error
	GetSemantic(context.Context, string) (SemanticRecord, error)
	QuerySemantics(context.Context, SemanticQuery) ([]SemanticRecord, error)
	ApplyConsolidation(context.Context, ConsolidationRun, []ConsolidatedPattern) error
	GetConsolidation(context.Context, string) (ConsolidationRun, error)
	ConsolidatedForRun(context.Context, string) ([]ConsolidatedPattern, error)
	RevokeConsolidation(context.Context, string, string, string, time.Time) (ConsolidationRun, error)
}

type database struct {
	SchemaVersion string                         `json:"schema_version"`
	OrganismID    string                         `json:"organism_id"`
	Episodes      map[string]Episode             `json:"episodes"`
	Semantics     map[string]SemanticRecord      `json:"semantics"`
	Runs          map[string]ConsolidationRun    `json:"consolidation_runs"`
	Patterns      map[string]ConsolidatedPattern `json:"consolidated_patterns"`
}

// InMemory is a concurrency-safe store for one organism's attributed memories.
type InMemory struct {
	mu       sync.RWMutex
	database database
	persist  func(database) (bool, error)
}

// NewMemoryStore creates an in-memory memory store for one organism.
func NewMemoryStore(organismID string) (*InMemory, error) {
	if strings.TrimSpace(organismID) == "" {
		return nil, errors.New("organism ID must not be empty")
	}
	return newMemoryStore(database{
		SchemaVersion: "arachne.memory.v1",
		OrganismID:    organismID,
		Episodes:      make(map[string]Episode),
		Semantics:     make(map[string]SemanticRecord),
		Runs:          make(map[string]ConsolidationRun),
		Patterns:      make(map[string]ConsolidatedPattern),
	}), nil
}

func newMemoryStore(db database) *InMemory {
	return &InMemory{database: db}
}

// PutEpisode adds an immutable, source-attributed observed experience.
func (s *InMemory) PutEpisode(ctx context.Context, episode Episode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateEpisode(s.database.OrganismID, episode); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.database.Episodes[episode.ID]; exists {
		return ErrDuplicateID
	}
	next := cloneDatabase(s.database)
	next.Episodes[episode.ID] = cloneEpisode(episode)
	if s.persist != nil {
		renamed, err := s.persist(next)
		if err != nil {
			if renamed {
				s.database = next
			}
			return err
		}
	}
	s.database = next
	return nil
}

// GetEpisode returns one experience by stable ID.
func (s *InMemory) GetEpisode(ctx context.Context, id string) (Episode, error) {
	if err := ctx.Err(); err != nil {
		return Episode{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	episode, exists := s.database.Episodes[id]
	if !exists {
		return Episode{}, ErrNotFound
	}
	return cloneEpisode(episode), nil
}

// QueryEpisodes returns matching experiences ordered by occurrence time then ID.
func (s *InMemory) QueryEpisodes(ctx context.Context, query EpisodeQuery) ([]Episode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if query.Limit < 1 {
		return nil, errors.New("episode query limit must be positive")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Episode, 0)
	for _, episode := range s.database.Episodes {
		if !matchesEpisode(episode, query) {
			continue
		}
		result = append(result, cloneEpisode(episode))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].OccurredAt.Equal(result[j].OccurredAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].OccurredAt.Before(result[j].OccurredAt)
	})
	if len(result) > query.Limit {
		result = result[:query.Limit]
	}
	return result, nil
}

// PutSemantic adds a sourced candidate or records an explicit rejection/supersession.
func (s *InMemory) PutSemantic(ctx context.Context, record SemanticRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSemantic(s.database.OrganismID, record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.database.Semantics[record.ID]; exists {
		return ErrDuplicateID
	}
	next := cloneDatabase(s.database)
	next.Semantics[record.ID] = cloneSemantic(record)
	if s.persist != nil {
		renamed, err := s.persist(next)
		if err != nil {
			if renamed {
				s.database = next
			}
			return err
		}
	}
	s.database = next
	return nil
}

// GetSemantic returns one sourced interpretation by stable ID.
func (s *InMemory) GetSemantic(ctx context.Context, id string) (SemanticRecord, error) {
	if err := ctx.Err(); err != nil {
		return SemanticRecord{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, exists := s.database.Semantics[id]
	if !exists {
		return SemanticRecord{}, ErrNotFound
	}
	return cloneSemantic(record), nil
}

// QuerySemantics returns matching interpretations by creation time then ID.
func (s *InMemory) QuerySemantics(ctx context.Context, query SemanticQuery) ([]SemanticRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if query.Limit < 1 {
		return nil, errors.New("semantic query limit must be positive")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SemanticRecord, 0)
	for _, record := range s.database.Semantics {
		if !matchesSemantic(record, query) {
			continue
		}
		result = append(result, cloneSemantic(record))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	if len(result) > query.Limit {
		result = result[:query.Limit]
	}
	return result, nil
}

// ApplyConsolidation atomically stores one run and all of its derived patterns.
func (s *InMemory) ApplyConsolidation(ctx context.Context, run ConsolidationRun, patterns []ConsolidatedPattern) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateConsolidation(s.database.OrganismID, run, patterns); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.database.Runs[run.ID]; exists {
		return ErrDuplicateID
	}
	next := cloneDatabase(s.database)
	next.Runs[run.ID] = cloneRun(run)
	for _, pattern := range patterns {
		if _, exists := next.Patterns[pattern.ID]; exists {
			return ErrDuplicateID
		}
		next.Patterns[pattern.ID] = clonePattern(pattern)
	}
	if err := s.commit(next); err != nil {
		return err
	}
	return nil
}

// GetConsolidation returns the run record that explains one derived memory pass.
func (s *InMemory) GetConsolidation(ctx context.Context, id string) (ConsolidationRun, error) {
	if err := ctx.Err(); err != nil {
		return ConsolidationRun{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, exists := s.database.Runs[id]
	if !exists {
		return ConsolidationRun{}, ErrNotFound
	}
	return cloneRun(run), nil
}

// ConsolidatedForRun returns the derived patterns attached to one run.
func (s *InMemory) ConsolidatedForRun(ctx context.Context, runID string) ([]ConsolidatedPattern, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.database.Runs[runID]; !exists {
		return nil, ErrNotFound
	}
	result := make([]ConsolidatedPattern, 0)
	for _, pattern := range s.database.Patterns {
		if pattern.RunID == runID {
			result = append(result, clonePattern(pattern))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

// RevokeConsolidation hides a run's patterns while retaining its audit record.
func (s *InMemory) RevokeConsolidation(ctx context.Context, id, actor, reason string, at time.Time) (ConsolidationRun, error) {
	if err := ctx.Err(); err != nil {
		return ConsolidationRun{}, err
	}
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(reason) == "" || at.IsZero() {
		return ConsolidationRun{}, errors.New("consolidation revocation requires actor, reason, and time")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, exists := s.database.Runs[id]
	if !exists {
		return ConsolidationRun{}, ErrNotFound
	}
	if run.Status != ConsolidationActive {
		return ConsolidationRun{}, errors.New("consolidation run is already revoked")
	}
	next := cloneDatabase(s.database)
	run = next.Runs[id]
	run.Status = ConsolidationRevoked
	run.RevokedAt = &at
	run.RevokedBy = actor
	run.RevokeReason = reason
	next.Runs[id] = run
	if err := s.commit(next); err != nil {
		return ConsolidationRun{}, err
	}
	return cloneRun(run), nil
}

func (s *InMemory) commit(next database) error {
	if s.persist != nil {
		renamed, err := s.persist(next)
		if err != nil {
			if renamed {
				s.database = next
			}
			return err
		}
	}
	s.database = next
	return nil
}

func validateEpisode(organismID string, episode Episode) error {
	if strings.TrimSpace(episode.ID) == "" || episode.OrganismID != organismID {
		return errors.New("episode ID and matching organism ID are required")
	}
	if strings.TrimSpace(episode.Source.Kind) == "" || strings.TrimSpace(episode.Source.ID) == "" {
		return errors.New("episode source kind and ID are required")
	}
	if episode.OccurredAt.IsZero() || episode.RecordedAt.IsZero() {
		return errors.New("episode occurrence and record times are required")
	}
	if strings.TrimSpace(episode.Kind) == "" || len(episode.Content) == 0 || !json.Valid(episode.Content) {
		return errors.New("episode kind and valid JSON content are required")
	}
	return nil
}

func validateSemantic(organismID string, record SemanticRecord) error {
	if strings.TrimSpace(record.ID) == "" || record.OrganismID != organismID {
		return errors.New("semantic record ID and matching organism ID are required")
	}
	if strings.TrimSpace(record.Assertion) == "" || record.CreatedAt.IsZero() {
		return errors.New("semantic assertion and creation time are required")
	}
	if strings.TrimSpace(record.Source.Kind) == "" || strings.TrimSpace(record.Source.ID) == "" {
		return errors.New("semantic source kind and ID are required")
	}
	if math.IsNaN(record.Confidence) || math.IsInf(record.Confidence, 0) || record.Confidence < 0 || record.Confidence > 1 {
		return errors.New("semantic confidence must be between zero and one")
	}
	if record.Status != SemanticCandidate && record.Status != SemanticRejected && record.Status != SemanticSuperseded {
		return fmt.Errorf("unsupported semantic status %q", record.Status)
	}
	if len(record.Context) > 0 && !json.Valid(record.Context) {
		return errors.New("semantic context must be valid JSON")
	}
	return nil
}

func validateConsolidation(organismID string, run ConsolidationRun, patterns []ConsolidatedPattern) error {
	if strings.TrimSpace(run.ID) == "" || run.OrganismID != organismID || strings.TrimSpace(run.RuleID) == "" {
		return errors.New("consolidation ID, matching organism ID, and rule ID are required")
	}
	if run.MinimumSupport < 2 || run.CreatedAt.IsZero() || (run.Status != ConsolidationActive && run.Status != ConsolidationRevoked) {
		return errors.New("consolidation requires support of at least two, a creation time, and a valid status")
	}
	if run.Status == ConsolidationRevoked && (run.RevokedAt == nil || strings.TrimSpace(run.RevokedBy) == "" || strings.TrimSpace(run.RevokeReason) == "") {
		return errors.New("revoked consolidation requires actor, reason, and time")
	}
	patternIDs := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern.ID) == "" || pattern.RunID != run.ID || pattern.OrganismID != organismID || pattern.RuleID != run.RuleID {
			return errors.New("consolidated pattern attribution does not match its run")
		}
		if strings.TrimSpace(pattern.Fingerprint) == "" || strings.TrimSpace(pattern.Kind) == "" ||
			len(pattern.CanonicalContent) == 0 || !json.Valid(pattern.CanonicalContent) ||
			pattern.Support < run.MinimumSupport || pattern.Support != len(pattern.SourceEpisodeIDs) ||
			pattern.FirstObservedAt.IsZero() || pattern.LastObservedAt.Before(pattern.FirstObservedAt) || pattern.CreatedAt.IsZero() {
			return fmt.Errorf("consolidated pattern %q has invalid content or evidence support", pattern.ID)
		}
		patternIDs[pattern.ID] = struct{}{}
	}
	if len(patternIDs) != len(run.PatternIDs) {
		return errors.New("consolidation run pattern links do not match its outputs")
	}
	for _, patternID := range run.PatternIDs {
		if _, exists := patternIDs[patternID]; !exists {
			return fmt.Errorf("consolidation run references missing pattern %q", patternID)
		}
	}
	for _, episodeID := range run.SourceEpisodeIDs {
		if strings.TrimSpace(episodeID) == "" {
			return errors.New("consolidation source episode IDs must not be empty")
		}
	}
	return nil
}

func matchesEpisode(episode Episode, query EpisodeQuery) bool {
	if query.OrganismID != "" && episode.OrganismID != query.OrganismID ||
		query.AgentID != "" && episode.AgentID != query.AgentID ||
		query.SessionID != "" && episode.SessionID != query.SessionID ||
		query.CorrelationID != "" && episode.CorrelationID != query.CorrelationID ||
		query.SourceKind != "" && episode.Source.Kind != query.SourceKind ||
		query.SourceID != "" && episode.Source.ID != query.SourceID ||
		!query.From.IsZero() && episode.OccurredAt.Before(query.From) ||
		!query.To.IsZero() && episode.OccurredAt.After(query.To) {
		return false
	}
	for _, tag := range query.Tags {
		if !contains(episode.Tags, tag) {
			return false
		}
	}
	text := strings.ToLower(string(episode.Content))
	return containsTerms(text, query.Terms)
}

func matchesSemantic(record SemanticRecord, query SemanticQuery) bool {
	if query.OrganismID != "" && record.OrganismID != query.OrganismID ||
		query.AgentID != "" && record.AgentID != query.AgentID ||
		query.SessionID != "" && record.SessionID != query.SessionID ||
		query.Status != "" && record.Status != query.Status ||
		query.SourceID != "" && record.Source.ID != query.SourceID {
		return false
	}
	return containsTerms(strings.ToLower(record.Assertion), query.Terms)
}

func containsTerms(value string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(value, strings.ToLower(term)) {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func cloneDatabase(db database) database {
	clone := database{
		SchemaVersion: db.SchemaVersion,
		OrganismID:    db.OrganismID,
		Episodes:      make(map[string]Episode, len(db.Episodes)),
		Semantics:     make(map[string]SemanticRecord, len(db.Semantics)),
		Runs:          make(map[string]ConsolidationRun, len(db.Runs)),
		Patterns:      make(map[string]ConsolidatedPattern, len(db.Patterns)),
	}
	for id, episode := range db.Episodes {
		clone.Episodes[id] = cloneEpisode(episode)
	}
	for id, record := range db.Semantics {
		clone.Semantics[id] = cloneSemantic(record)
	}
	for id, run := range db.Runs {
		clone.Runs[id] = cloneRun(run)
	}
	for id, pattern := range db.Patterns {
		clone.Patterns[id] = clonePattern(pattern)
	}
	return clone
}

func cloneEpisode(episode Episode) Episode {
	episode.SourceEventIDs = append([]string(nil), episode.SourceEventIDs...)
	episode.Content = append(json.RawMessage(nil), episode.Content...)
	episode.Tags = append([]string(nil), episode.Tags...)
	return episode
}

func cloneSemantic(record SemanticRecord) SemanticRecord {
	record.SourceEpisodeIDs = append([]string(nil), record.SourceEpisodeIDs...)
	record.SourceEventIDs = append([]string(nil), record.SourceEventIDs...)
	record.Contradicts = append([]string(nil), record.Contradicts...)
	record.Context = append(json.RawMessage(nil), record.Context...)
	return record
}

func cloneRun(run ConsolidationRun) ConsolidationRun {
	run.SourceEpisodeIDs = append([]string(nil), run.SourceEpisodeIDs...)
	run.PatternIDs = append([]string(nil), run.PatternIDs...)
	if run.RevokedAt != nil {
		revokedAt := *run.RevokedAt
		run.RevokedAt = &revokedAt
	}
	return run
}

func clonePattern(pattern ConsolidatedPattern) ConsolidatedPattern {
	pattern.CanonicalContent = append(json.RawMessage(nil), pattern.CanonicalContent...)
	pattern.SourceEpisodeIDs = append([]string(nil), pattern.SourceEpisodeIDs...)
	pattern.SourceEventIDs = append([]string(nil), pattern.SourceEventIDs...)
	pattern.Tags = append([]string(nil), pattern.Tags...)
	return pattern
}
