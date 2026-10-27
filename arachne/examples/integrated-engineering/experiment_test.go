package main

import (
	"context"
	"testing"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/development"
	"github.com/haha-systems/arachne2/internal/experiment"
)

func TestDisabledDevelopmentalLearningCannotInvokeUpdate(t *testing.T) {
	calls := 0
	state := runState{
		subsystems: map[string]bool{"developmental_learning": false},
		developmentUpdate: func(context.Context, runState, agent.Proposal, string, string) (development.Change, error) {
			calls++
			return development.Change{}, nil
		},
	}
	change, err := state.runDevelopmentalLearning(context.Background(), agent.Proposal{}, "action", "replay")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("disabled developmental learning invoked its implementation %d times", calls)
	}
	if change != nil {
		t.Fatal("disabled developmental learning returned a change")
	}
}

func TestEnabledDevelopmentalLearningUsesConfiguredImplementation(t *testing.T) {
	calls := 0
	state := runState{
		subsystems: map[string]bool{"developmental_learning": true},
		developmentUpdate: func(context.Context, runState, agent.Proposal, string, string) (development.Change, error) {
			calls++
			return development.Change{ID: "configured-update"}, nil
		},
	}
	change, err := state.runDevelopmentalLearning(context.Background(), agent.Proposal{}, "action", "replay")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || change == nil || change.ID != "configured-update" {
		t.Fatalf("configured update not applied: calls=%d change=%+v", calls, change)
	}
}

func TestIntegratedProfilesRejectRequiredSubsystemOverrides(t *testing.T) {
	spec := experiment.Spec{
		Name: "invalid", Task: experimentTaskName, Trials: 1,
		Profiles: []experiment.Profile{{
			Name: "unsafe", Subsystems: map[string]bool{
				"developmental_learning": false, "inspection": true, "governance": false,
			},
		}},
	}
	if err := validateIntegratedProfiles(spec); err == nil {
		t.Fatal("profile that disables required governance was accepted")
	}
}
