package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

// Service binds memory operations to organism attribution and the event spine.
type Service struct {
	organismID string
	store      Store
	events     *cognition.Spine
	now        func() time.Time
}

// NewService creates an attributed memory interface over one store and event spine.
func NewService(organismID string, store Store, events *cognition.Spine) (*Service, error) {
	if strings.TrimSpace(organismID) == "" {
		return nil, errors.New("organism ID must not be empty")
	}
	if store == nil || events == nil {
		return nil, errors.New("memory store and cognitive event spine are required")
	}
	return &Service{organismID: organismID, store: store, events: events, now: time.Now}, nil
}

// RecordEpisode assigns missing identity and record time, stores the experience, and emits its source event.
func (s *Service) RecordEpisode(ctx context.Context, episode Episode) (Episode, error) {
	if episode.ID == "" {
		id, err := newID()
		if err != nil {
			return Episode{}, err
		}
		episode.ID = id
	}
	if episode.OrganismID == "" {
		episode.OrganismID = s.organismID
	}
	if episode.RecordedAt.IsZero() {
		episode.RecordedAt = s.now().UTC()
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "episode_recorded", "episode_id": episode.ID,
		"source": episode.Source, "source_event_ids": episode.SourceEventIDs,
		"kind": episode.Kind,
	})
	if err != nil {
		return Episode{}, fmt.Errorf("encode episode event: %w", err)
	}
	parentEventIDs := make([]string, 0, len(episode.SourceEventIDs))
	for _, id := range episode.SourceEventIDs {
		if id != "" {
			parentEventIDs = append(parentEventIDs, id)
		}
	}
	sort.Strings(parentEventIDs)
	parentEventIDs = uniqueStrings(parentEventIDs)
	event, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: episode.AgentID, SessionID: episode.SessionID, CorrelationID: episode.CorrelationID,
		ParentEventIDs: parentEventIDs, Kind: cognition.KindMemory, Payload: payload,
	})
	if err != nil {
		return cloneEpisode(episode), fmt.Errorf("record episode event for %q: %w", episode.ID, err)
	}
	episode.EventID = event.EventID
	if err := s.store.PutEpisode(ctx, episode); err != nil {
		return cloneEpisode(episode), fmt.Errorf("episode event recorded but episode could not be stored: %w", err)
	}
	return cloneEpisode(episode), nil
}

// RetrieveEpisodes returns attributable matches and emits the IDs made available.
func (s *Service) RetrieveEpisodes(ctx context.Context, query EpisodeQuery) ([]Episode, error) {
	query.OrganismID = s.organismID
	result, err := s.store.QueryEpisodes(ctx, query)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(result))
	for i, episode := range result {
		ids[i] = episode.ID
	}
	payload, err := json.Marshal(map[string]any{"operation": "episodes_retrieved", "episode_ids": ids})
	if err != nil {
		return nil, fmt.Errorf("encode episode retrieval event: %w", err)
	}
	if _, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: query.AgentID, SessionID: query.SessionID,
		Kind: cognition.KindMemory, Payload: payload,
	}); err != nil {
		return nil, fmt.Errorf("retrieval completed but event recording failed: %w", err)
	}
	return result, nil
}

// RecordSemanticCandidate adds a sourced interpretation without promoting it.
func (s *Service) RecordSemanticCandidate(ctx context.Context, record SemanticRecord) (SemanticRecord, error) {
	if record.ID == "" {
		id, err := newID()
		if err != nil {
			return SemanticRecord{}, err
		}
		record.ID = id
	}
	if record.OrganismID == "" {
		record.OrganismID = s.organismID
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = s.now().UTC()
	}
	if record.Status == "" {
		record.Status = SemanticCandidate
	}
	if record.Status != SemanticCandidate {
		return SemanticRecord{}, errors.New("service only records semantic candidates; review and governance own later status changes")
	}
	if err := s.store.PutSemantic(ctx, record); err != nil {
		return SemanticRecord{}, err
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "semantic_candidate_recorded", "semantic_id": record.ID,
		"source": record.Source, "source_episode_ids": record.SourceEpisodeIDs,
		"source_event_ids": record.SourceEventIDs,
	})
	if err != nil {
		return SemanticRecord{}, fmt.Errorf("encode semantic memory event: %w", err)
	}
	if _, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: record.AgentID, SessionID: record.SessionID,
		Kind: cognition.KindMemory, Payload: payload,
	}); err != nil {
		return cloneSemantic(record), fmt.Errorf("semantic record stored as %q but event recording failed: %w", record.ID, err)
	}
	return cloneSemantic(record), nil
}

// RetrieveSemantics returns attributable candidate records and emits their IDs.
func (s *Service) RetrieveSemantics(ctx context.Context, query SemanticQuery) ([]SemanticRecord, error) {
	query.OrganismID = s.organismID
	result, err := s.store.QuerySemantics(ctx, query)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(result))
	for i, record := range result {
		ids[i] = record.ID
	}
	payload, err := json.Marshal(map[string]any{"operation": "semantics_retrieved", "semantic_ids": ids})
	if err != nil {
		return nil, fmt.Errorf("encode semantic retrieval event: %w", err)
	}
	if _, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: query.AgentID, SessionID: query.SessionID,
		Kind: cognition.KindMemory, Payload: payload,
	}); err != nil {
		return nil, fmt.Errorf("semantic retrieval completed but event recording failed: %w", err)
	}
	return result, nil
}

func newID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate memory ID: %w", err)
	}
	return hex.EncodeToString(random[:]), nil
}
