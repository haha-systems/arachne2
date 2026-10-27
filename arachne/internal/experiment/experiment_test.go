package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRunPairsSeedsAndCreatesFreshTrialState(t *testing.T) {
	spec := Spec{
		Name: "pair", Task: "example", Trials: 2, Seed: 40,
		Profiles: []Profile{
			{Name: "full", Subsystems: map[string]bool{"developmental_learning": true}},
			{Name: "ablated", Subsystems: map[string]bool{"developmental_learning": false}},
		},
	}
	var output bytes.Buffer
	calls := 0
	err := Run(context.Background(), &output, spec, func(_ context.Context, profile Profile, _ int64) (Result, func() error, error) {
		calls++
		// Each executor call starts from its own task state. A shared state object
		// would produce values greater than one here.
		localState := struct{ count int }{}
		localState.count++
		return Result{
			Output:       map[string]any{"local_count": localState.count},
			Scores:       map[string]float64{"success": 1},
			Measurements: map[string]any{"enabled": profile.Subsystems["developmental_learning"]},
		}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 {
		t.Fatalf("executor called %d times, want 4", calls)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d output lines, want 4", len(lines))
	}
	var rows []Trial
	for _, line := range lines {
		var row Trial
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	wantSeeds := []int64{40, 40, 41, 41}
	for i, row := range rows {
		if row.Seed != wantSeeds[i] {
			t.Errorf("row %d seed = %d, want %d", i, row.Seed, wantSeeds[i])
		}
		if got := row.Output.(map[string]any)["local_count"]; got != float64(1) {
			t.Errorf("row %d inherited task state: local_count=%v", i, got)
		}
	}
	if rows[1].Configuration.Subsystems["developmental_learning"] {
		t.Fatal("disabled profile was not captured in output configuration")
	}
}

func TestRunRecordsErrorsAndContinues(t *testing.T) {
	spec := Spec{
		Name: "errors", Task: "example", Trials: 2,
		Profiles: []Profile{{Name: "full", Subsystems: map[string]bool{}}},
	}
	var output bytes.Buffer
	calls := 0
	err := Run(context.Background(), &output, spec, func(context.Context, Profile, int64) (Result, func() error, error) {
		calls++
		if calls == 1 {
			return Result{Events: []string{"partial-event"}, Measurements: map[string]any{"progress": 1}}, nil, errors.New("trial failure")
		}
		return Result{Scores: map[string]float64{"success": 1}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("executor stopped after error; calls=%d", calls)
	}
	if !strings.Contains(output.String(), `"error":"trial failure"`) {
		t.Fatalf("error was not serialized: %s", output.String())
	}
	if !strings.Contains(output.String(), "partial-event") || !strings.Contains(output.String(), `"progress":1`) {
		t.Fatalf("partial data was dropped from error row: %s", output.String())
	}
}

func TestCompareProducesStableSummary(t *testing.T) {
	input := strings.Join([]string{
		`{"profile":"z","duration_ms":8,"scores":{"success":1}}`,
		`{"profile":"a","duration_ms":4,"scores":{"success":0},"error":"failed"}`,
	}, "\n")
	var output bytes.Buffer
	if err := Compare(strings.NewReader(input), &output, "success"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "[\n  {\n    \"profile\": \"a\"") {
		t.Fatalf("summary is not sorted: %s", output.String())
	}
	if !strings.Contains(output.String(), `"errors": 1`) {
		t.Fatalf("summary did not count errors: %s", output.String())
	}
	if !strings.Contains(output.String(), `"score_trials": 0`) {
		t.Fatalf("comparison included the score from an errored trial: %s", output.String())
	}
}
