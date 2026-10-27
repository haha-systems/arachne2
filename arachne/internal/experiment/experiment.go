// Package experiment runs paired task profiles and records trials as JSONL.
package experiment

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"
)

// Profile explicitly selects which task subsystems participate in a trial.
type Profile struct {
	Name       string          `json:"name"`
	Subsystems map[string]bool `json:"subsystems"`
}

// Spec is a declarative, JSON-compatible experiment definition.
type Spec struct {
	Name     string    `json:"name"`
	Task     string    `json:"task"`
	Trials   int       `json:"trials"`
	Seed     int64     `json:"seed"`
	Profiles []Profile `json:"profiles"`
}

// Result contains task outputs and task-specific observations.
type Result struct {
	Output       any                `json:"output,omitempty"`
	Events       any                `json:"events,omitempty"`
	Scores       map[string]float64 `json:"scores,omitempty"`
	Measurements map[string]any     `json:"measurements,omitempty"`
}

// Execute constructs and runs one isolated task instance. The returned cleanup
// must release processes and resources owned by this trial.
type Execute func(context.Context, Profile, int64) (Result, func() error, error)

// Trial is one append-only record in the JSONL output.
type Trial struct {
	SchemaVersion int                `json:"schema_version"`
	Experiment    string             `json:"experiment"`
	Task          string             `json:"task"`
	Profile       string             `json:"profile"`
	Trial         int                `json:"trial"`
	Seed          int64              `json:"seed"`
	Configuration Profile            `json:"configuration"`
	StartedAt     time.Time          `json:"started_at"`
	DurationMS    int64              `json:"duration_ms"`
	Events        any                `json:"events,omitempty"`
	Output        any                `json:"output,omitempty"`
	Scores        map[string]float64 `json:"scores,omitempty"`
	Measurements  map[string]any     `json:"measurements,omitempty"`
	Error         string             `json:"error,omitempty"`
}

// Run validates a spec, executes each profile with a common seed per trial, and
// writes one JSON object per line. Errors are records and do not stop later trials.
func Run(ctx context.Context, writer io.Writer, spec Spec, execute Execute) error {
	if err := Validate(spec); err != nil {
		return err
	}
	if execute == nil {
		return errors.New("experiment task executor is required")
	}
	encoder := json.NewEncoder(writer)
	for trialIndex := 0; trialIndex < spec.Trials; trialIndex++ {
		seed := spec.Seed + int64(trialIndex)
		for _, profile := range spec.Profiles {
			record := Trial{
				SchemaVersion: 1, Experiment: spec.Name, Task: spec.Task,
				Profile: profile.Name, Trial: trialIndex + 1, Seed: seed,
				Configuration: profile, StartedAt: time.Now().UTC(),
			}
			start := time.Now()
			result, cleanup, err := execute(ctx, profile, seed)
			if cleanup != nil {
				if cleanupErr := cleanup(); err == nil {
					err = cleanupErr
				}
			}
			record.DurationMS = time.Since(start).Milliseconds()
			record.Events, record.Output = result.Events, result.Output
			record.Scores, record.Measurements = result.Scores, result.Measurements
			if err != nil {
				record.Error = err.Error()
			}
			if err := encoder.Encode(record); err != nil {
				return fmt.Errorf("write trial %d profile %q: %w", record.Trial, profile.Name, err)
			}
		}
	}
	return nil
}

// Validate rejects ambiguous or unusable specifications before any task starts.
func Validate(spec Spec) error {
	if spec.Name == "" || spec.Task == "" {
		return errors.New("experiment name and task are required")
	}
	if spec.Trials < 1 {
		return errors.New("trials must be at least 1")
	}
	if len(spec.Profiles) == 0 {
		return errors.New("at least one profile is required")
	}
	seen := make(map[string]bool, len(spec.Profiles))
	for _, profile := range spec.Profiles {
		if profile.Name == "" || seen[profile.Name] {
			return fmt.Errorf("profile names must be non-empty and unique: %q", profile.Name)
		}
		seen[profile.Name] = true
		if profile.Subsystems == nil {
			return fmt.Errorf("profile %q must declare subsystem settings", profile.Name)
		}
	}
	return nil
}

// Summary is the compact output of the compare command.
type Summary struct {
	Profile        string  `json:"profile"`
	Trials         int     `json:"trials"`
	Errors         int     `json:"errors"`
	MeanDurationMS float64 `json:"mean_duration_ms"`
	MeanScore      float64 `json:"mean_score"`
	ScoreTrials    int     `json:"score_trials"`
}

// Compare groups JSONL trials by profile and summarizes score and duration.
func Compare(reader io.Reader, writer io.Writer, scoreName string) error {
	groups := map[string][]Trial{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var trial Trial
		if err := json.Unmarshal(scanner.Bytes(), &trial); err != nil {
			return fmt.Errorf("decode JSONL line %d: %w", line, err)
		}
		if trial.Profile == "" {
			return fmt.Errorf("JSONL line %d has no profile", line)
		}
		groups[trial.Profile] = append(groups[trial.Profile], trial)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(groups) == 0 {
		return errors.New("no trials found")
	}
	summaries := make([]Summary, 0, len(groups))
	for profile, trials := range groups {
		summary := Summary{Profile: profile, Trials: len(trials)}
		scoreSum := 0.0
		durationSum := int64(0)
		for _, trial := range trials {
			durationSum += trial.DurationMS
			if trial.Error != "" {
				summary.Errors++
			}
			if trial.Error == "" {
				if score, ok := trial.Scores[scoreName]; ok && !math.IsNaN(score) && !math.IsInf(score, 0) {
					scoreSum += score
					summary.ScoreTrials++
				}
			}
		}
		summary.MeanDurationMS = float64(durationSum) / float64(len(trials))
		if summary.ScoreTrials > 0 {
			summary.MeanScore = scoreSum / float64(summary.ScoreTrials)
		}
		summaries = append(summaries, summary)
	}
	// Sort to make reports stable across Go map iteration order.
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Profile < summaries[j].Profile })
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summaries)
}
