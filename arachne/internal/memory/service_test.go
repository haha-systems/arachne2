package memory

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestAttributedMemoryKeepsEpisodesAndSemanticCandidatesDistinct(t *testing.T) {
	ctx := context.Background()
	store, err := NewMemoryStore("org-a")
	if err != nil {
		t.Fatal(err)
	}
	eventStore, err := cognition.NewMemoryStore(16)
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
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	service.now = func() time.Time { return now }
	episode, err := service.RecordEpisode(ctx, Episode{
		ID: "episode-1", Source: SourceRef{Kind: "observation", ID: "obs-1"},
		SourceEventIDs: []string{"event-1"}, OccurredAt: now, Kind: "observation",
		Content: json.RawMessage(`{"observed":true}`),
	})
	if err != nil {
		t.Fatalf("record episode: %v", err)
	}
	candidate, err := service.RecordSemanticCandidate(ctx, SemanticRecord{
		ID: "semantic-1", Assertion: "the observation may indicate a pattern",
		Source:           SourceRef{Kind: "interpretation", ID: "model-1"},
		SourceEpisodeIDs: []string{episode.ID}, SourceEventIDs: []string{"event-1", "inference-1"},
		Contradicts: []string{"semantic-old"}, Confidence: 0.65,
	})
	if err != nil {
		t.Fatalf("record semantic candidate: %v", err)
	}
	if candidate.Status != SemanticCandidate || candidate.OrganismID != "org-a" || candidate.CreatedAt.IsZero() {
		t.Fatalf("candidate defaults or status were lost: %+v", candidate)
	}
	if !reflect.DeepEqual(candidate.SourceEpisodeIDs, []string{"episode-1"}) ||
		!reflect.DeepEqual(candidate.SourceEventIDs, []string{"event-1", "inference-1"}) ||
		!reflect.DeepEqual(candidate.Contradicts, []string{"semantic-old"}) {
		t.Fatalf("candidate provenance or contradiction links were lost: %+v", candidate)
	}
	if _, err := service.RecordSemanticCandidate(ctx, SemanticRecord{
		ID: "promoted", OrganismID: "org-a", Assertion: "must be governed",
		Source: SourceRef{Kind: "review", ID: "review-1"}, CreatedAt: now, Confidence: 1,
		Status: SemanticRejected,
	}); err == nil {
		t.Fatal("memory service accepted a non-candidate status transition")
	}

	episodes, err := store.QueryEpisodes(ctx, EpisodeQuery{OrganismID: "org-a", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	semantics, err := store.QuerySemantics(ctx, SemanticQuery{OrganismID: "org-a", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || len(semantics) != 1 || episodes[0].ID == semantics[0].ID {
		t.Fatalf("observed episodes and derived candidates were not kept distinct: episodes=%+v semantics=%+v", episodes, semantics)
	}

	rejected := SemanticRecord{
		ID: "semantic-rejected", OrganismID: "org-a", Assertion: "rejected interpretation",
		Source: SourceRef{Kind: "review", ID: "review-2"}, SourceEpisodeIDs: []string{"episode-1"},
		Contradicts: []string{"semantic-1"}, CreatedAt: now, Confidence: 0.2, Status: SemanticRejected,
	}
	if err := store.PutSemantic(ctx, rejected); err != nil {
		t.Fatalf("store explicit rejected record: %v", err)
	}
	stored, err := store.GetSemantic(ctx, rejected.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != SemanticRejected || !reflect.DeepEqual(stored.Contradicts, []string{"semantic-1"}) {
		t.Fatalf("semantic status/contradictions were not preserved: %+v", stored)
	}
}

func TestMemoryRejectsInvalidAndCrossOrganismRecords(t *testing.T) {
	ctx := context.Background()
	store, err := NewMemoryStore("org-a")
	if err != nil {
		t.Fatal(err)
	}
	events, _ := cognition.NewSpine("org-a", mustMemoryEventStore(t, 8))
	service, err := NewService("org-a", store, events)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	validEpisode := Episode{
		ID: "cross-org", OrganismID: "org-b", Source: SourceRef{Kind: "test", ID: "source"},
		OccurredAt: now, RecordedAt: now, Kind: "observation", Content: json.RawMessage(`{}`),
	}
	if err := store.PutEpisode(ctx, validEpisode); err == nil {
		t.Fatal("store accepted an episode attributed to another organism")
	}
	if _, err := service.RecordEpisode(ctx, validEpisode); err == nil {
		t.Fatal("service accepted a cross-organism episode")
	}
	validEpisode.OrganismID = "org-a"
	validEpisode.ID = "invalid-json"
	validEpisode.Content = json.RawMessage(`not-json`)
	if err := store.PutEpisode(ctx, validEpisode); err == nil {
		t.Fatal("store accepted malformed episode content")
	}
	if err := store.PutSemantic(ctx, SemanticRecord{
		ID: "cross-org-semantic", OrganismID: "org-b", Assertion: "other organism",
		Source: SourceRef{Kind: "test", ID: "source"}, CreatedAt: now, Confidence: 0.5, Status: SemanticCandidate,
	}); err == nil {
		t.Fatal("store accepted a semantic record attributed to another organism")
	}
}

func mustMemoryEventStore(t *testing.T, capacity int) *cognition.MemoryStore {
	t.Helper()
	store, err := cognition.NewMemoryStore(capacity)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
