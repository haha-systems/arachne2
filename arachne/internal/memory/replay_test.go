package memory

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestReplayUsesImmutableEpisodesAndEmitsOnlyAttributableCues(t *testing.T) {
	ctx := context.Background()
	store, err := NewMemoryStore("org-a")
	if err != nil {
		t.Fatal(err)
	}
	eventStore, err := cognition.NewMemoryStore(32)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine("org-a", eventStore)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService("org-a", store, events)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	service.now = func() time.Time { return now }

	content := json.RawMessage(`{"value":1}`)
	sourceEvents := []string{"episode-source-a"}
	episodeA, err := service.RecordEpisode(ctx, Episode{
		ID: "episode-a", Source: SourceRef{Kind: "test", ID: "a"}, SourceEventIDs: sourceEvents,
		OccurredAt: now.Add(time.Minute), Kind: "observation", Content: content,
	})
	if err != nil {
		t.Fatalf("record first episode: %v", err)
	}
	content[9] = '9'
	sourceEvents[0] = "mutated-source"
	episodeB, err := service.RecordEpisode(ctx, Episode{
		ID: "episode-b", Source: SourceRef{Kind: "test", ID: "b"}, SourceEventIDs: []string{"episode-source-b"},
		OccurredAt: now, Kind: "observation", Content: json.RawMessage(`{"value":2}`),
	})
	if err != nil {
		t.Fatalf("record second episode: %v", err)
	}
	if episodeA.OrganismID != "org-a" || episodeA.RecordedAt.IsZero() || episodeB.OrganismID != "org-a" {
		t.Fatalf("episode attribution defaults were not applied: a=%+v b=%+v", episodeA, episodeB)
	}
	semantic, err := service.RecordSemanticCandidate(ctx, SemanticRecord{
		ID: "semantic-1", Assertion: "the observation is useful", Source: SourceRef{Kind: "candidate", ID: "candidate-1"},
		SourceEpisodeIDs: []string{"episode-a"}, SourceEventIDs: []string{"semantic-source"}, Confidence: 0.7,
	})
	if err != nil {
		t.Fatalf("record semantic candidate: %v", err)
	}
	if semantic.Status != SemanticCandidate {
		t.Fatalf("semantic status = %q, want candidate", semantic.Status)
	}

	planInput := ReplayPlan{
		InteractionID: "interaction-1", RequestedBy: "agent-1", ScheduledAt: now,
		EpisodeIDs: []string{"episode-a", "episode-b"}, SourceEventIDs: []string{"schedule-source"},
	}
	plan, err := service.ScheduleReplay(ctx, planInput)
	if err != nil {
		t.Fatalf("schedule replay: %v", err)
	}
	planInput.EpisodeIDs[0] = "missing"
	planInput.SourceEventIDs[0] = "mutated-schedule-source"
	if plan.ID == "" || plan.PlanDigest == "" || plan.ScheduleEventID == "" {
		t.Fatalf("scheduled plan is missing immutable identity: %+v", plan)
	}

	run, err := service.RunReplay(ctx, plan)
	if err != nil {
		t.Fatalf("run replay: %v", err)
	}
	if !reflect.DeepEqual(run.InputEpisodeIDs, []string{"episode-b", "episode-a"}) {
		t.Fatalf("replay order = %v, want chronological episode order", run.InputEpisodeIDs)
	}
	if len(run.Cues) != 2 || run.Cues[1].EpisodeID != "episode-a" ||
		!reflect.DeepEqual(run.Cues[1].SourceEventIDs, []string{"episode-source-a"}) || run.Cues[1].ContentDigest == "" {
		t.Fatalf("replay cues lost episode lineage: %+v", run.Cues)
	}
	storedA, err := store.GetEpisode(ctx, "episode-a")
	if err != nil {
		t.Fatal(err)
	}
	if string(storedA.Content) != `{"value":1}` || !reflect.DeepEqual(storedA.SourceEventIDs, []string{"episode-source-a"}) {
		t.Fatalf("replay input episode was mutated through caller data: %+v", storedA)
	}
	episodes, err := store.QueryEpisodes(ctx, EpisodeQuery{OrganismID: "org-a", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	semantics, err := store.QuerySemantics(ctx, SemanticQuery{OrganismID: "org-a", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 2 || len(semantics) != 1 || semantics[0].ID != semantic.ID {
		t.Fatalf("replay changed memory state: episodes=%d semantics=%d", len(episodes), len(semantics))
	}

	history, err := eventStore.Read(ctx, 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	var replayEvents []cognition.Event
	for _, event := range history {
		if event.Kind == cognition.KindAction || event.Kind == cognition.KindDevelopment || event.Kind == cognition.KindGovernance {
			t.Fatalf("replay performed an authoritative operation: %+v", event)
		}
		if event.Kind == cognition.KindReplay {
			replayEvents = append(replayEvents, event)
		}
	}
	if len(replayEvents) != 2 || replayEvents[1].EventID != run.EventID {
		t.Fatalf("expected only schedule and completion replay records, got %+v", replayEvents)
	}
	wantParents := []string{"episode-source-a", "episode-source-b", plan.ScheduleEventID, "schedule-source"}
	if !reflect.DeepEqual(replayEvents[1].ParentEventIDs, wantParents) {
		t.Fatalf("completion parents = %v, want %v", replayEvents[1].ParentEventIDs, wantParents)
	}
}

func TestReplayRejectsTamperedOrNotYetDuePlans(t *testing.T) {
	ctx := context.Background()
	store, _ := NewMemoryStore("org-a")
	eventStore, _ := cognition.NewMemoryStore(8)
	events, _ := cognition.NewSpine("org-a", eventStore)
	service, _ := NewService("org-a", store, events)
	now := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.RecordEpisode(ctx, Episode{
		ID: "episode-1", Source: SourceRef{Kind: "test", ID: "source"},
		OccurredAt: now, Kind: "observation", Content: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := service.ScheduleReplay(ctx, ReplayPlan{
		InteractionID: "interaction", RequestedBy: "agent", ScheduledAt: now.Add(time.Hour), EpisodeIDs: []string{"episode-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunReplay(ctx, plan); err == nil {
		t.Fatal("future replay ran before its scheduled time")
	}
	plan.EpisodeIDs[0] = "different-episode"
	if _, err := service.RunReplay(ctx, plan); err == nil {
		t.Fatal("tampered replay plan was accepted")
	}
}
