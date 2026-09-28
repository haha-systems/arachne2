package main

import (
	"context"
	"testing"
)

func TestDisabledInspectionDoesNotAccessEventSpine(t *testing.T) {
	state := runState{subsystems: map[string]bool{"inspection": false}}
	inspector, report, err := state.inspectInitial(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if inspector != nil || report.OrganismID != "" || report.Through != 0 || len(report.Activities) != 0 {
		t.Fatalf("disabled inspection returned data: inspector=%v report=%+v", inspector, report)
	}
}

func TestDisabledMemoryReplayDoesNotAccessMemoryService(t *testing.T) {
	state := runState{subsystems: map[string]bool{"memory_replay": false}}
	replay, err := state.replay(context.Background(), []string{"episode-a"})
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID != "" || replay.EventID != "" || len(replay.Cues) != 0 {
		t.Fatalf("disabled replay returned data: %+v", replay)
	}
}

func TestDisabledReplayIDsCannotEnterEvidenceOrFailureEvents(t *testing.T) {
	got := filterEventIDs("episode-a", "", "specialist-event", "")
	if len(got) != 2 || got[0] != "episode-a" || got[1] != "specialist-event" {
		t.Fatalf("empty replay ID leaked through: %#v", got)
	}
}
