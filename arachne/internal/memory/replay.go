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

// ReplayPlan schedules immutable episodes to be reconsidered as attention cues.
type ReplayPlan struct {
	ID              string    `json:"id"`
	InteractionID   string    `json:"interaction_id"`
	RequestedBy     string    `json:"requested_by"`
	ScheduledAt     time.Time `json:"scheduled_at"`
	EpisodeIDs      []string  `json:"episode_ids"`
	SourceEventIDs  []string  `json:"source_event_ids,omitempty"`
	PlanDigest      string    `json:"plan_digest"`
	ScheduleEventID string    `json:"schedule_event_id"`
}

// ReplayCue is a compact, attributable candidate for renewed attention.
type ReplayCue struct {
	EpisodeID      string    `json:"episode_id"`
	Kind           string    `json:"kind"`
	OccurredAt     time.Time `json:"occurred_at"`
	ContentDigest  string    `json:"content_digest"`
	SourceEventIDs []string  `json:"source_event_ids,omitempty"`
}

// ReplayRun records the deterministic outputs of one scheduled replay.
type ReplayRun struct {
	ID              string      `json:"id"`
	PlanID          string      `json:"plan_id"`
	PlanDigest      string      `json:"plan_digest"`
	ScheduleEventID string      `json:"schedule_event_id"`
	CompletedAt     time.Time   `json:"completed_at"`
	InputEpisodeIDs []string    `json:"input_episode_ids"`
	Cues            []ReplayCue `json:"cues"`
	EventID         string      `json:"event_id"`
}

// ScheduleReplay records an explicit replay request and its fixed evidence set.
func (s *Service) ScheduleReplay(ctx context.Context, plan ReplayPlan) (ReplayPlan, error) {
	plan, err := prepareReplayPlan(ctx, s.store, plan)
	if err != nil {
		return ReplayPlan{}, err
	}
	plan.ID = replayPlanID(plan)
	plan.PlanDigest = replayPlanDigest(plan)
	payload, err := json.Marshal(map[string]any{
		"operation": "replay_scheduled", "plan_id": plan.ID,
		"plan_digest": plan.PlanDigest, "interaction_id": plan.InteractionID,
		"requested_by": plan.RequestedBy, "scheduled_at": plan.ScheduledAt,
		"episode_ids": plan.EpisodeIDs, "source_event_ids": plan.SourceEventIDs,
	})
	if err != nil {
		return ReplayPlan{}, fmt.Errorf("encode replay schedule: %w", err)
	}
	event, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: plan.RequestedBy, CorrelationID: plan.InteractionID,
		ParentEventIDs: plan.SourceEventIDs, Kind: cognition.KindReplay, Payload: payload,
	})
	if err != nil {
		return ReplayPlan{}, fmt.Errorf("record replay schedule: %w", err)
	}
	plan.ScheduleEventID = event.EventID
	return plan, nil
}

func prepareReplayPlan(ctx context.Context, store Store, plan ReplayPlan) (ReplayPlan, error) {
	if strings.TrimSpace(plan.InteractionID) == "" || strings.TrimSpace(plan.RequestedBy) == "" || plan.ScheduledAt.IsZero() {
		return ReplayPlan{}, errors.New("replay requires interaction, requester, and scheduled time")
	}
	if len(plan.EpisodeIDs) == 0 {
		return ReplayPlan{}, errors.New("replay requires at least one source episode")
	}
	plan.EpisodeIDs = append([]string(nil), plan.EpisodeIDs...)
	sort.Strings(plan.EpisodeIDs)
	for i, id := range plan.EpisodeIDs {
		if strings.TrimSpace(id) == "" {
			return ReplayPlan{}, errors.New("replay episode IDs must not be empty")
		}
		if i > 0 && plan.EpisodeIDs[i-1] == id {
			return ReplayPlan{}, fmt.Errorf("replay episode %q is listed more than once", id)
		}
		if _, err := store.GetEpisode(ctx, id); err != nil {
			return ReplayPlan{}, fmt.Errorf("load replay episode %q: %w", id, err)
		}
	}
	plan.SourceEventIDs = sortedUniqueReplayIDs(plan.SourceEventIDs)
	for _, eventID := range plan.SourceEventIDs {
		if strings.TrimSpace(eventID) == "" {
			return ReplayPlan{}, errors.New("replay source event IDs must not be empty")
		}
	}
	plan.ScheduledAt = plan.ScheduledAt.UTC()
	return plan, nil
}

// RunReplay emits ordered attention cues without changing memory or granting authority.
func (s *Service) RunReplay(ctx context.Context, plan ReplayPlan) (ReplayRun, error) {
	if plan.ID == "" || plan.PlanDigest == "" || plan.ScheduleEventID == "" || len(plan.EpisodeIDs) == 0 {
		return ReplayRun{}, errors.New("replay must be scheduled before it can run")
	}
	if replayPlanID(plan) != plan.ID || replayPlanDigest(plan) != plan.PlanDigest {
		return ReplayRun{}, errors.New("replay plan changed after scheduling")
	}
	if plan.ScheduledAt.After(s.now()) {
		return ReplayRun{}, errors.New("replay is not due yet")
	}
	cues, inputIDs, parents, err := buildReplayCues(ctx, s.store, plan)
	if err != nil {
		return ReplayRun{}, err
	}
	completedAt := s.now().UTC()
	run := ReplayRun{
		ID: replayRunID(plan.ID), PlanID: plan.ID, PlanDigest: plan.PlanDigest,
		ScheduleEventID: plan.ScheduleEventID, CompletedAt: completedAt,
		InputEpisodeIDs: inputIDs, Cues: cues,
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "replay_completed", "run_id": run.ID,
		"plan_id": run.PlanID, "plan_digest": run.PlanDigest,
		"schedule_event_id": run.ScheduleEventID, "completed_at": run.CompletedAt,
		"input_episode_ids": run.InputEpisodeIDs, "cues": run.Cues,
	})
	if err != nil {
		return ReplayRun{}, fmt.Errorf("encode replay result: %w", err)
	}
	event, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: plan.RequestedBy, CorrelationID: plan.InteractionID,
		ParentEventIDs: parents, Kind: cognition.KindReplay, Payload: payload,
	})
	if err != nil {
		return ReplayRun{}, fmt.Errorf("record replay result: %w", err)
	}
	run.EventID = event.EventID
	return run, nil
}

func buildReplayCues(ctx context.Context, store Store, plan ReplayPlan) ([]ReplayCue, []string, []string, error) {
	episodes := make([]Episode, 0, len(plan.EpisodeIDs))
	for _, id := range plan.EpisodeIDs {
		episode, err := store.GetEpisode(ctx, id)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("load replay episode %q: %w", id, err)
		}
		episodes = append(episodes, episode)
	}
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].OccurredAt.Equal(episodes[j].OccurredAt) {
			return episodes[i].ID < episodes[j].ID
		}
		return episodes[i].OccurredAt.Before(episodes[j].OccurredAt)
	})
	cues := make([]ReplayCue, 0, len(episodes))
	parents := append([]string{plan.ScheduleEventID}, plan.SourceEventIDs...)
	inputIDs := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		canonical, err := canonicalJSON(episode.Content)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("canonicalize replay episode %q: %w", episode.ID, err)
		}
		contentHash := sha256.Sum256(canonical)
		cues = append(cues, ReplayCue{
			EpisodeID: episode.ID, Kind: episode.Kind, OccurredAt: episode.OccurredAt,
			ContentDigest:  hex.EncodeToString(contentHash[:]),
			SourceEventIDs: sortedUniqueReplayIDs(episode.SourceEventIDs),
		})
		inputIDs = append(inputIDs, episode.ID)
		parents = append(parents, episode.SourceEventIDs...)
	}
	return cues, inputIDs, sortedUniqueReplayIDs(parents), nil
}

func replayPlanID(plan ReplayPlan) string {
	return stableID(plan.InteractionID, plan.RequestedBy, plan.ScheduledAt.UTC().Format(time.RFC3339Nano), strings.Join(plan.EpisodeIDs, "\x00"), strings.Join(plan.SourceEventIDs, "\x00"))
}

func replayPlanDigest(plan ReplayPlan) string {
	encoded, _ := json.Marshal(struct {
		ID             string    `json:"id"`
		InteractionID  string    `json:"interaction_id"`
		RequestedBy    string    `json:"requested_by"`
		ScheduledAt    time.Time `json:"scheduled_at"`
		EpisodeIDs     []string  `json:"episode_ids"`
		SourceEventIDs []string  `json:"source_event_ids"`
	}{plan.ID, plan.InteractionID, plan.RequestedBy, plan.ScheduledAt.UTC(), plan.EpisodeIDs, plan.SourceEventIDs})
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func replayRunID(planID string) string { return stableID("replay.run.v1", planID) }

func sortedUniqueReplayIDs(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return uniqueStrings(result)
}
