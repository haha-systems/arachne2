package memory

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

func TestConsolidationPreservesEvidenceAndRevocationStatus(t *testing.T) {
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
	now := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)
	service.now = func() time.Time { return now }
	for _, episode := range []Episode{
		{
			ID: "episode-1", Source: SourceRef{Kind: "observation", ID: "obs-1"},
			SourceEventIDs: []string{"source-a", "shared-event"}, OccurredAt: now,
			Kind: "pattern", Content: json.RawMessage(`{"x":1,"y":2}`), Tags: []string{"alpha"},
		},
		{
			ID: "episode-2", Source: SourceRef{Kind: "observation", ID: "obs-2"},
			SourceEventIDs: []string{"source-b", "shared-event"}, OccurredAt: now.Add(time.Hour),
			Kind: "pattern", Content: json.RawMessage(`{"y":2,"x":1}`), Tags: []string{"beta"},
		},
	} {
		if _, err := service.RecordEpisode(ctx, episode); err != nil {
			t.Fatalf("record episode %q: %v", episode.ID, err)
		}
	}

	policy := ConsolidationPolicy{MinimumSupport: 2, EpisodeQuery: EpisodeQuery{Limit: 10}}
	run, patterns, err := service.Consolidate(ctx, policy)
	if err != nil {
		t.Fatalf("consolidate episodes: %v", err)
	}
	if run.Status != ConsolidationActive || len(patterns) != 1 || patterns[0].Support != 2 {
		t.Fatalf("unexpected consolidation result: run=%+v patterns=%+v", run, patterns)
	}
	pattern := patterns[0]
	if !reflect.DeepEqual(pattern.SourceEpisodeIDs, []string{"episode-1", "episode-2"}) ||
		!reflect.DeepEqual(pattern.SourceEventIDs, []string{"shared-event", "source-a", "source-b"}) ||
		!reflect.DeepEqual(pattern.Tags, []string{"alpha", "beta"}) {
		t.Fatalf("consolidated evidence was not preserved: %+v", pattern)
	}
	if string(pattern.CanonicalContent) != `{"x":1,"y":2}` {
		t.Fatalf("consolidation did not normalize repeated JSON: %s", pattern.CanonicalContent)
	}
	secondRun, secondPatterns, err := service.Consolidate(ctx, policy)
	if err != nil {
		t.Fatalf("repeat consolidation: %v", err)
	}
	if secondRun.ID != run.ID || !reflect.DeepEqual(secondPatterns, patterns) {
		t.Fatalf("repeat consolidation was not idempotent: %+v %+v", secondRun, secondPatterns)
	}

	revoked, err := service.RevokeConsolidation(ctx, run.ID, "reviewer", "evidence withdrawn")
	if err != nil {
		t.Fatalf("revoke consolidation: %v", err)
	}
	if revoked.Status != ConsolidationRevoked || revoked.RevokedBy != "reviewer" || revoked.RevokeReason != "evidence withdrawn" || revoked.RevokedAt == nil {
		t.Fatalf("revocation status was not preserved: %+v", revoked)
	}
	if _, _, err := service.RetrieveConsolidation(ctx, run.ID, false, "reader", "session"); !errors.Is(err, ErrConsolidationRevoked) {
		t.Fatalf("ordinary retrieval error = %v, want revoked", err)
	}
	inspectedRun, inspectedPatterns, err := service.RetrieveConsolidation(ctx, run.ID, true, "reader", "session")
	if err != nil {
		t.Fatalf("inspect revoked consolidation: %v", err)
	}
	if inspectedRun.Status != ConsolidationRevoked || len(inspectedPatterns) != 1 || inspectedPatterns[0].ID != pattern.ID {
		t.Fatalf("revoked inspection lost run or pattern: %+v %+v", inspectedRun, inspectedPatterns)
	}
}
