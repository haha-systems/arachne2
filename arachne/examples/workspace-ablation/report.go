package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/haha-systems/arachne2/internal/experiment"
)

type conditionSummary struct {
	Trials           int     `json:"trials"`
	Successes        int     `json:"successes"`
	SuccessRate      float64 `json:"success_rate"`
	AverageSteps     float64 `json:"average_steps"`
	AverageEvents    float64 `json:"average_events"`
	AverageRuntimeMS float64 `json:"average_runtime_ms"`
	Invalid          int     `json:"invalid_trials"`
}
type pairedOutcome struct {
	Seed              int64 `json:"seed"`
	WorkspaceEnabled  bool  `json:"workspace_enabled"`
	WorkspaceDisabled bool  `json:"workspace_disabled"`
}
type comparison struct {
	Experiment    string                      `json:"experiment"`
	Conditions    map[string]conditionSummary `json:"conditions"`
	Paired        []pairedOutcome             `json:"paired_outcomes"`
	InvalidTrials []string                    `json:"invalid_trials,omitempty"`
}

func compareFile(path string) (comparison, error) {
	// #nosec G304 -- the compare input is explicitly selected by the local operator.
	data, err := os.ReadFile(path)
	if err != nil {
		return comparison{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var rows []experiment.Trial
	for line := 1; ; line++ {
		var row experiment.Trial
		err := decoder.Decode(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return comparison{}, fmt.Errorf("JSONL line %d: %w", line, err)
		}
		rows = append(rows, row)
	}
	return summarize(rows), nil
}

func summarize(rows []experiment.Trial) comparison {
	report := comparison{
		Experiment: experimentName,
		Conditions: map[string]conditionSummary{
			"workspace_enabled": {}, "workspace_disabled": {},
		},
	}
	bySeed := make(map[int64]map[string]bool)
	for _, row := range rows {
		condition, ok := report.Conditions[row.Profile]
		if !ok {
			report.InvalidTrials = append(report.InvalidTrials, fmt.Sprintf("seed %d: unknown profile %q", row.Seed, row.Profile))
			continue
		}
		condition.Trials++
		condition.AverageRuntimeMS += measurement(row.Measurements, "runtime_ms_precise")
		if measurement(row.Measurements, "runtime_ms_precise") == 0 {
			condition.AverageRuntimeMS += float64(row.DurationMS)
		}
		condition.AverageSteps += measurement(row.Measurements, "agent_steps")
		condition.AverageEvents += measurement(row.Measurements, "event_count")
		success, ok := outputSuccess(row.Output)
		if row.Error != "" || !ok {
			condition.Invalid++
			report.InvalidTrials = append(report.InvalidTrials, fmt.Sprintf("seed %d %s: %s", row.Seed, row.Profile, row.Error))
		} else if success {
			condition.Successes++
		}
		if bySeed[row.Seed] == nil {
			bySeed[row.Seed] = make(map[string]bool)
		}
		bySeed[row.Seed][row.Profile] = success && row.Error == ""
		report.Conditions[row.Profile] = condition
	}
	for name, condition := range report.Conditions {
		if condition.Trials > 0 {
			condition.SuccessRate = float64(condition.Successes) / float64(condition.Trials)
			condition.AverageRuntimeMS /= float64(condition.Trials)
			condition.AverageSteps /= float64(condition.Trials)
			condition.AverageEvents /= float64(condition.Trials)
		}
		report.Conditions[name] = condition
	}
	seeds := make([]int64, 0, len(bySeed))
	for seed := range bySeed {
		seeds = append(seeds, seed)
	}
	sort.Slice(seeds, func(i, j int) bool { return seeds[i] < seeds[j] })
	for _, seed := range seeds {
		conditions := bySeed[seed]
		enabled, hasEnabled := conditions["workspace_enabled"]
		disabled, hasDisabled := conditions["workspace_disabled"]
		if !hasEnabled || !hasDisabled {
			report.InvalidTrials = append(report.InvalidTrials, fmt.Sprintf("seed %d is not paired", seed))
			continue
		}
		report.Paired = append(report.Paired, pairedOutcome{Seed: seed, WorkspaceEnabled: enabled, WorkspaceDisabled: disabled})
	}
	return report
}

func outputSuccess(value any) (bool, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return false, false
	}
	success, ok := object["success"].(bool)
	return success, ok
}

func measurement(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return 0
	}
}

func printComparison(writer io.Writer, summary comparison) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return err
	}
	for _, name := range []string{"workspace_enabled", "workspace_disabled"} {
		value := summary.Conditions[name]
		_, err := fmt.Fprintf(writer, "%s: %d/%d successes (%.1f%%), avg steps %.2f, events %.2f, runtime %.2f ms, invalid %d\n",
			name, value.Successes, value.Trials, value.SuccessRate*100, value.AverageSteps,
			value.AverageEvents, value.AverageRuntimeMS, value.Invalid)
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(writer, "paired seeds: %d; invalid records: %d\n", len(summary.Paired), len(summary.InvalidTrials))
	return err
}
