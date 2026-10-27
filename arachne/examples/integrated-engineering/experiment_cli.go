package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/haha-systems/arachne2/internal/experiment"
)

const experimentTaskName = "integrated-engineering"

func runExperimentCLI(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: integrated-engineering experiment <run|compare> [options]")
	}
	switch args[0] {
	case "run":
		return runConfiguredExperiment(args[1:])
	case "compare":
		return compareExperiment(args[1:])
	default:
		return fmt.Errorf("unknown experiment command %q", args[0])
	}
}

func runConfiguredExperiment(args []string) error {
	flags := flag.NewFlagSet("experiment run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	configPath := flags.String("config", "", "JSON experiment specification")
	silkPath := flags.String("silk", "", "path to Silk executable")
	outputPath := flags.String("output", "-", "JSONL output path, or - for stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *configPath == "" || *silkPath == "" {
		return errors.New("usage: integrated-engineering experiment run -config SPEC.json -silk PATH [-output trials.jsonl]")
	}
	configBytes, err := os.ReadFile(*configPath)
	if err != nil {
		return fmt.Errorf("read experiment config: %w", err)
	}
	var spec experiment.Spec
	if err := json.Unmarshal(configBytes, &spec); err != nil {
		return fmt.Errorf("decode experiment config: %w", err)
	}
	if spec.Task != experimentTaskName {
		return fmt.Errorf("unsupported task %q; this runner supports %q", spec.Task, experimentTaskName)
	}
	if err := validateIntegratedProfiles(spec); err != nil {
		return err
	}

	var writer io.Writer = os.Stdout
	var file *os.File
	if *outputPath != "-" {
		file, err = os.Create(*outputPath)
		if err != nil {
			return fmt.Errorf("create JSONL output: %w", err)
		}
		writer = file
	}
	runErr := experiment.Run(context.Background(), writer, spec, func(ctx context.Context, profile experiment.Profile, _ int64) (experiment.Result, func() error, error) {
		state, err := newRunState(ctx, *silkPath, false)
		if err != nil {
			return experiment.Result{}, nil, err
		}
		state.subsystems = cloneSubsystems(profile.Subsystems)
		cleanup := func() error { return state.client.Close() }
		report, err := runExperiment(ctx, state)
		if err != nil {
			events, readErr := readEvents(ctx, state.events)
			measurements := map[string]any{"event_count": len(events), "seed_consumed_by_task": false}
			partial := experiment.Result{Events: events, Measurements: measurements}
			if readErr != nil {
				return partial, cleanup, errors.Join(err, fmt.Errorf("failed to read partial events: %w", readErr))
			}
			return partial, cleanup, err
		}
		success := 1.0
		if report.TaskFailure != nil {
			success = 0
		}
		measurements := map[string]any{
			"event_count":                len(report.Events),
			"initial_routing":            report.InitialRouting,
			"final_routing":              report.FinalRouting,
			"selected_specialist":        report.SelectedProposal.SpecialistID,
			"consolidated_pattern_count": len(report.Consolidation.PatternIDs),
			"replay_count":               len(report.Replay.Cues),
			"seed_consumed_by_task":      false,
		}
		return experiment.Result{
			Output: report, Events: report.Events,
			Scores:       map[string]float64{"success": success},
			Measurements: measurements,
		}, cleanup, nil
	})
	if file != nil {
		if closeErr := file.Close(); runErr == nil {
			runErr = closeErr
		}
	}
	return runErr
}

func compareExperiment(args []string) error {
	flags := flag.NewFlagSet("experiment compare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputPath := flags.String("input", "", "JSONL trial output")
	score := flags.String("score", "success", "score key to summarize")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *inputPath == "" {
		return errors.New("usage: integrated-engineering experiment compare -input trials.jsonl [-score success]")
	}
	file, err := os.Open(*inputPath)
	if err != nil {
		return fmt.Errorf("open JSONL trials: %w", err)
	}
	compareErr := experiment.Compare(file, os.Stdout, *score)
	if closeErr := file.Close(); compareErr == nil {
		compareErr = closeErr
	}
	return compareErr
}

func validateIntegratedProfiles(spec experiment.Spec) error {
	allowed := map[string]bool{"developmental_learning": true, "inspection": true, "memory_replay": true}
	for _, profile := range spec.Profiles {
		for subsystem := range profile.Subsystems {
			if !allowed[subsystem] {
				return fmt.Errorf("profile %q names unsupported or required subsystem %q; this task exposes %s", profile.Name, subsystem, strings.Join(sortedKeys(allowed), ", "))
			}
		}
		for name := range allowed {
			if _, ok := profile.Subsystems[name]; !ok {
				return fmt.Errorf("profile %q must explicitly set subsystem %q", profile.Name, name)
			}
		}
	}
	return nil
}

func cloneSubsystems(source map[string]bool) map[string]bool {
	clone := make(map[string]bool, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
