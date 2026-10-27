package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/experiment"
)

func testControls() controls {
	return controls{
		ModelProvider: "deterministic_synthetic", Model: "none", Tools: []string{},
		StepBudget: 1, TimeLimitMS: 1000, Memory: "disabled", Replay: "disabled",
		Learning: "disabled", ActiveInference: "disabled", Governance: "not_used_synthetic_task",
		InitialState: "fresh_event_store_and_supervisor_per_trial", WorkspaceCapacity: 4,
		Agents: []string{"evidence-0", "evidence-1", "evidence-2", "evidence-3", "evidence-decoy"},
	}
}

func profiles() (experiment.Profile, experiment.Profile) {
	return experiment.Profile{Name: "workspace_enabled", Subsystems: map[string]bool{"shared_workspace": true}},
		experiment.Profile{Name: "workspace_disabled", Subsystems: map[string]bool{"shared_workspace": false}}
}

func TestTaskGenerationIsDeterministicAndSeeded(t *testing.T) {
	first := generateTask(88)
	again := generateTask(88)
	other := generateTask(89)
	if first != again {
		t.Fatal("same seed generated different task")
	}
	if first == other {
		t.Fatal("different seed generated the same task")
	}
}

func TestDisabledWorkspaceHasNoPublicationOrBroadcastPath(t *testing.T) {
	_, disabled := profiles()
	result, cleanup, err := runTrial(context.Background(), testControls(), disabled, 101)
	if cleanup != nil {
		t.Fatal("trial unexpectedly returned cleanup")
	}
	if err != nil {
		t.Fatalf("disabled trial failed: %v", err)
	}
	out := result.Output.(trialOutput)
	if out.Success || out.FinalAnswer != nil || out.TerminationReason != "no_shared_evidence" {
		t.Fatalf("disabled solver got evidence: %+v", out)
	}
	events := result.Events.([]cognition.Event)
	publications, broadcasts, messages := countWorkspaceEvents(events)
	if publications != 0 || broadcasts != 0 || messages != 0 {
		t.Fatalf("disabled condition emitted workspace activity: publications=%d broadcasts=%d messages=%d", publications, broadcasts, messages)
	}
	if _, ok := result.Measurements["workspace_report"]; ok {
		t.Fatal("disabled trial recorded a workspace report")
	}
	if got := result.Measurements["messages"]; got != 0 {
		t.Fatalf("disabled trial delivered messages: %v", got)
	}
	if got := result.Measurements["workspace_reads"]; got != 0 {
		t.Fatalf("disabled condition read workspace: %v", got)
	}
}

func TestEnabledWorkspacePublishesAndBroadcastsEvidence(t *testing.T) {
	enabled, _ := profiles()
	result, _, err := runTrial(context.Background(), testControls(), enabled, 101)
	if err != nil {
		t.Fatalf("enabled trial failed: %v", err)
	}
	events := result.Events.([]cognition.Event)
	publications, broadcasts, messages := countWorkspaceEvents(events)
	if publications != 5 || broadcasts != 1 || messages != 1 {
		t.Fatalf("enabled workspace activity: publications=%d broadcasts=%d messages=%d", publications, broadcasts, messages)
	}
	if got := result.Measurements["workspace_reads"]; got != 2 {
		t.Fatalf("enabled trial recorded %v workspace reads, want two", got)
	}
	if _, ok := result.Output.(trialOutput); !ok {
		t.Fatalf("unexpected output type %T", result.Output)
	}
}

func TestConditionsKeepPairedTaskAndFreshTrialState(t *testing.T) {
	enabled, disabled := profiles()
	left, _, err := runTrial(context.Background(), testControls(), enabled, 233)
	if err != nil {
		t.Fatal(err)
	}
	right, _, err := runTrial(context.Background(), testControls(), disabled, 233)
	if err != nil {
		t.Fatal(err)
	}
	leftOut := left.Output.(trialOutput)
	rightOut := right.Output.(trialOutput)
	if leftOut.TaskInput != rightOut.TaskInput || leftOut.ExpectedAnswer != rightOut.ExpectedAnswer {
		t.Fatalf("paired task changed between conditions: %+v versus %+v", leftOut, rightOut)
	}
	leftEvents := left.Events.([]cognition.Event)
	rightEvents := right.Events.([]cognition.Event)
	if len(leftEvents) == 0 || len(rightEvents) == 0 || leftEvents[0].Sequence != 1 || rightEvents[0].Sequence != 1 {
		t.Fatalf("trial event state was not fresh: enabled=%+v disabled=%+v", leftEvents, rightEvents)
	}
	repeated, _, err := runTrial(context.Background(), testControls(), enabled, 233)
	if err != nil {
		t.Fatal(err)
	}
	repeatedEvents := repeated.Events.([]cognition.Event)
	if len(repeatedEvents) != len(leftEvents) || repeatedEvents[0].Sequence != 1 {
		t.Fatal("state from a prior trial contaminated repeated enabled seed")
	}
	for index := range leftEvents {
		if leftEvents[index].EventID != repeatedEvents[index].EventID || leftEvents[index].Kind != repeatedEvents[index].Kind || string(leftEvents[index].Payload) != string(repeatedEvents[index].Payload) {
			t.Fatalf("seeded event history changed at index %d", index)
		}
	}
}

func TestConfigOnlyChangesWorkspaceSubsystem(t *testing.T) {
	cfg, err := loadConfig("../../experiments/workspace-ablation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a := cfg.Experiment.Profiles[0]
	b := cfg.Experiment.Profiles[1]
	delete(a.Subsystems, "shared_workspace")
	delete(b.Subsystems, "shared_workspace")
	if a.Name == b.Name || len(a.Subsystems) != len(b.Subsystems) {
		t.Fatal("unexpected profile shape")
	}
	for key, value := range a.Subsystems {
		if b.Subsystems[key] != value {
			t.Fatalf("non-workspace subsystem differs: %s", key)
		}
	}
	raw, err := json.Marshal(cfg.Controls)
	if err != nil || len(raw) == 0 {
		t.Fatalf("controls are not serializable: %v", err)
	}
}
