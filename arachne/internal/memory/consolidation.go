package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

const exactRepeatRuleID = "episode.exact_repeat.v1"

// ErrConsolidationRevoked reports that ordinary retrieval hides a revoked run.
var ErrConsolidationRevoked = errors.New("consolidation run is revoked")

type repeatKey struct {
	kind    string
	content string
}

// Consolidate groups episodes with exactly repeated kind and normalized JSON content.
func (s *Service) Consolidate(ctx context.Context, policy ConsolidationPolicy) (ConsolidationRun, []ConsolidatedPattern, error) {
	if policy.MinimumSupport < 2 {
		return ConsolidationRun{}, nil, errors.New("consolidation minimum support must be at least two")
	}
	if policy.EpisodeQuery.Limit < 1 {
		return ConsolidationRun{}, nil, errors.New("consolidation episode query limit must be positive")
	}
	policy.EpisodeQuery.OrganismID = s.organismID
	episodes, err := s.store.QueryEpisodes(ctx, policy.EpisodeQuery)
	if err != nil {
		return ConsolidationRun{}, nil, err
	}
	buckets := make(map[repeatKey][]Episode)
	canonicalByKey := make(map[repeatKey]json.RawMessage)
	for _, episode := range episodes {
		canonical, canonicalErr := canonicalJSON(episode.Content)
		if canonicalErr != nil {
			return ConsolidationRun{}, nil, fmt.Errorf("canonicalize episode %q: %w", episode.ID, canonicalErr)
		}
		key := repeatKey{kind: episode.Kind, content: string(canonical)}
		buckets[key] = append(buckets[key], episode)
		canonicalByKey[key] = canonical
	}
	selectedIDs := make([]string, len(episodes))
	for i, episode := range episodes {
		selectedIDs[i] = episode.ID
	}
	sort.Strings(selectedIDs)
	runID := stableID(exactRepeatRuleID, fmt.Sprint(policy.MinimumSupport), strings.Join(selectedIDs, "\x00"))
	if existing, existingErr := s.store.GetConsolidation(ctx, runID); existingErr == nil {
		patterns, patternErr := s.store.ConsolidatedForRun(ctx, runID)
		return existing, patterns, patternErr
	} else if !errors.Is(existingErr, ErrNotFound) {
		return ConsolidationRun{}, nil, existingErr
	}
	createdAt := s.now().UTC()
	patterns := make([]ConsolidatedPattern, 0)
	for key, support := range buckets {
		if len(support) < policy.MinimumSupport {
			continue
		}
		pattern := buildPattern(s.organismID, runID, exactRepeatRuleID, key, canonicalByKey[key], support, createdAt)
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool { return patterns[i].ID < patterns[j].ID })
	patternIDs := make([]string, len(patterns))
	for i, pattern := range patterns {
		patternIDs[i] = pattern.ID
	}
	run := ConsolidationRun{
		ID: runID, OrganismID: s.organismID, RuleID: exactRepeatRuleID,
		MinimumSupport: policy.MinimumSupport, CreatedAt: createdAt,
		SourceEpisodeIDs: selectedIDs, PatternIDs: patternIDs,
		Status: ConsolidationActive,
	}
	if err := s.store.ApplyConsolidation(ctx, run, patterns); err != nil {
		return ConsolidationRun{}, nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "consolidation_applied", "run_id": run.ID,
		"rule_id": run.RuleID, "minimum_support": run.MinimumSupport,
		"source_episode_ids": run.SourceEpisodeIDs, "pattern_ids": run.PatternIDs,
	})
	if err != nil {
		return run, patterns, fmt.Errorf("encode consolidation event: %w", err)
	}
	if _, err := s.events.Emit(ctx, cognition.Draft{Kind: cognition.KindReplay, Payload: payload}); err != nil {
		return run, patterns, fmt.Errorf("consolidation %q persisted but event recording failed: %w", run.ID, err)
	}
	return run, patterns, nil
}

// RetrieveConsolidation returns patterns that influenced later retrieval unless revoked.
func (s *Service) RetrieveConsolidation(ctx context.Context, runID string, includeRevoked bool, agentID, sessionID string) (ConsolidationRun, []ConsolidatedPattern, error) {
	run, err := s.store.GetConsolidation(ctx, runID)
	if err != nil {
		return ConsolidationRun{}, nil, err
	}
	if run.Status == ConsolidationRevoked && !includeRevoked {
		return run, nil, ErrConsolidationRevoked
	}
	patterns, err := s.store.ConsolidatedForRun(ctx, runID)
	if err != nil {
		return ConsolidationRun{}, nil, err
	}
	ids := make([]string, len(patterns))
	for i, pattern := range patterns {
		ids[i] = pattern.ID
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "consolidation_retrieved", "run_id": run.ID,
		"status": run.Status, "pattern_ids": ids, "include_revoked": includeRevoked,
	})
	if err != nil {
		return run, patterns, fmt.Errorf("encode consolidation retrieval event: %w", err)
	}
	if _, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: agentID, SessionID: sessionID,
		Kind: cognition.KindMemory, Payload: payload,
	}); err != nil {
		return run, patterns, fmt.Errorf("consolidated retrieval completed but event recording failed: %w", err)
	}
	return run, patterns, nil
}

// RevokeConsolidation removes a derived run from ordinary retrieval with an attributed reason.
func (s *Service) RevokeConsolidation(ctx context.Context, runID, actorID, reason string) (ConsolidationRun, error) {
	run, err := s.store.RevokeConsolidation(ctx, runID, actorID, reason, s.now().UTC())
	if err != nil {
		return ConsolidationRun{}, err
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "consolidation_revoked", "run_id": run.ID,
		"actor_id": run.RevokedBy, "reason": run.RevokeReason,
	})
	if err != nil {
		return run, fmt.Errorf("encode consolidation revocation event: %w", err)
	}
	if _, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: actorID, Kind: cognition.KindReplay, Payload: payload,
	}); err != nil {
		return run, fmt.Errorf("consolidation %q revoked but event recording failed: %w", run.ID, err)
	}
	return run, nil
}

func buildPattern(organismID, runID, ruleID string, key repeatKey, content json.RawMessage, support []Episode, createdAt time.Time) ConsolidatedPattern {
	sourceIDs := make([]string, 0, len(support))
	eventIDs := make([]string, 0)
	tags := make(map[string]struct{})
	firstObserved, lastObserved := support[0].OccurredAt, support[0].OccurredAt
	for _, episode := range support {
		sourceIDs = append(sourceIDs, episode.ID)
		eventIDs = append(eventIDs, episode.SourceEventIDs...)
		if episode.OccurredAt.Before(firstObserved) {
			firstObserved = episode.OccurredAt
		}
		if episode.OccurredAt.After(lastObserved) {
			lastObserved = episode.OccurredAt
		}
		for _, tag := range episode.Tags {
			tags[tag] = struct{}{}
		}
	}
	sort.Strings(sourceIDs)
	sort.Strings(eventIDs)
	eventIDs = uniqueStrings(eventIDs)
	tagList := make([]string, 0, len(tags))
	for tag := range tags {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	fingerprint := stableID(key.kind, key.content)
	return ConsolidatedPattern{
		ID: stableID(runID, fingerprint), RunID: runID, OrganismID: organismID,
		RuleID: ruleID, Fingerprint: fingerprint, Kind: key.kind,
		CanonicalContent: append(json.RawMessage(nil), content...),
		SourceEpisodeIDs: sourceIDs, SourceEventIDs: eventIDs, Tags: tagList,
		FirstObservedAt: firstObserved, LastObservedAt: lastObserved,
		Support: len(support), CreatedAt: createdAt,
	}
}

func canonicalJSON(content json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func stableID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	unique := values[:1]
	for _, value := range values[1:] {
		if value != unique[len(unique)-1] {
			unique = append(unique, value)
		}
	}
	return unique
}
