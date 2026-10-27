package memory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestMemoryStateSnapshotRoundTripsAttributedEpisodesIndependently(t *testing.T) {
	ctx := context.Background()
	eventsStore, _ := cognition.NewMemoryStore(32)
	events, _ := cognition.NewSpine("memory-snapshot", eventsStore)
	store, _ := NewMemoryStore("memory-snapshot")
	service, _ := NewService("memory-snapshot", store, events)
	content := json.RawMessage(`{"observation":"sample"}`)
	episode, err := service.RecordEpisode(ctx, Episode{
		ID: "episode-1", Source: SourceRef{Kind: "test", ID: "interaction-1"},
		SourceEventIDs: []string{"source-event"}, OccurredAt: time.Unix(1, 0).UTC(),
		RecordedAt: time.Unix(1, 0).UTC(), Kind: "action_outcome", Content: content,
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.SnapshotState()
	if err != nil {
		t.Fatal(err)
	}
	restoreState, err := service.SnapshotState()
	if err != nil {
		t.Fatal(err)
	}
	state.Episodes[0].Content[0] = ' '
	original, err := store.GetEpisode(ctx, episode.ID)
	if err != nil || !json.Valid(original.Content) {
		t.Fatalf("snapshot shared stored content: %v", err)
	}

	target, _ := NewMemoryStore("memory-snapshot")
	restored, _ := NewService("memory-snapshot", target, events)
	if err := target.RestoreState(ctx, restoreState); err != nil {
		t.Fatal(err)
	}
	episodes, err := restored.RetrieveEpisodes(ctx, EpisodeQuery{OrganismID: "memory-snapshot", Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].ID != episode.ID || episodes[0].EventID != episode.EventID ||
		len(episodes[0].SourceEventIDs) != 1 || episodes[0].SourceEventIDs[0] != "source-event" {
		t.Fatalf("attributed memory did not round trip: %#v", episodes)
	}
	if !json.Valid(episodes[0].Content) {
		t.Fatal("episode content was not preserved")
	}
}
