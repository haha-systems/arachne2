package nursery

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWorldGenerationIsDeterministicAndPerceptionIsLocal(t *testing.T) {
	first, err := NewWorld(Config{Seed: 90210})
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewWorld(Config{Seed: 90210})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("same seed produced different full world states")
	}
	perception := first.Perceive()
	if perception.Imperative != viabilityImperative || perception.ViableRanges.Energy.Minimum >= perception.ViableRanges.Energy.Maximum {
		t.Fatalf("viability requirement/ranges are missing: %+v", perception)
	}
	if len(perception.Cells) >= len(first.Cells) || len(perception.Entities) >= len(first.Entities) {
		t.Fatalf("perception exposed the full world: local cells=%d/%d entities=%d/%d",
			len(perception.Cells), len(first.Cells), len(perception.Entities), len(first.Entities))
	}
	encoded, err := json.Marshal(perception)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"food", "poison", "healing_item", "safe_zone", "rng_state", "available", "effect",
		"reward", "success", "correct", "goal", "task_complete",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("perception disclosed %q: %s", forbidden, encoded)
		}
	}
}

func TestWorldPersistsCausalEffectsAndRejectsExternalActions(t *testing.T) {
	world, err := NewWorld(Config{Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	var energy *entity
	for index := range world.Entities {
		if world.Entities[index].Effect == effectEnergy {
			energy = &world.Entities[index]
			break
		}
	}
	if energy == nil {
		t.Fatal("seeded environment lacks its expected latent effect class")
	}
	world.Position = energy.Position
	world.Viability.Energy = 0.4
	energy.Available = 0.9
	changes, err := world.Apply(Action{Kind: "use", EntityID: energy.ID})
	if err != nil {
		t.Fatal(err)
	}
	if world.Viability.Energy <= 0.4 || len(changes) < 2 {
		t.Fatalf("use did not produce a persistent environmental consequence: viability=%+v changes=%+v", world.Viability, changes)
	}
	if world.Tick != 1 || len(world.Recent) == 0 || world.Recent[len(world.Recent)-1].Kind != "organism.viability" {
		t.Fatalf("tick history did not persist: tick=%d changes=%+v", world.Tick, world.Recent)
	}
	for _, action := range []Action{{Kind: "exec"}, {Kind: "find_food"}, {Kind: "network"}} {
		if _, err := world.Apply(action); err == nil {
			t.Fatalf("external or high-level action %q was accepted", action.Kind)
		}
	}
}

func TestDistinctDiscoverableEffectsCanAlterDifferentViabilityVariables(t *testing.T) {
	world, err := NewWorld(Config{Seed: 19})
	if err != nil {
		t.Fatal(err)
	}
	for _, effect := range []entityEffect{effectEnergy, effectIntegrity, effectStability} {
		target := findEffect(world, effect)
		if target == nil {
			t.Fatalf("generated world lacks latent effect class %d", effect)
		}
		world.Position = target.Position
		world.Viability = Viability{Energy: 0.45, Integrity: 0.5, Stability: 0.05}
		target.Available = 0.8
		before := world.Viability
		if _, err := world.Apply(Action{Kind: "use", EntityID: target.ID}); err != nil {
			t.Fatal(err)
		}
		switch effect {
		case effectEnergy:
			if world.Viability.Energy <= before.Energy {
				t.Fatalf("energy-correlated interaction changed no energy: before=%+v after=%+v", before, world.Viability)
			}
		case effectIntegrity:
			if world.Viability.Integrity <= before.Integrity {
				t.Fatalf("integrity-correlated interaction changed no integrity: before=%+v after=%+v", before, world.Viability)
			}
		case effectStability:
			if world.Viability.Stability <= before.Stability {
				t.Fatalf("ambient-correlated interaction changed no stability: before=%+v after=%+v", before, world.Viability)
			}
		}
		world.Tick = 0
		world.NextEventID = 1
		world.LastEventID = ""
	}
}

func TestCheckpointRestorationCreatesIndependentExactBranches(t *testing.T) {
	ctx := context.Background()
	organism := &statefulTestOrganism{}
	session, err := NewSession(ctx, ExperimentConfig{Name: "nursery-test", World: Config{Seed: 31}}, organism)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Run(ctx, 4); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := session.CreateCheckpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/checkpoint.json"
	if err := SaveCheckpoint(path, checkpoint); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadCheckpoint(path)
	if err != nil {
		t.Fatal(err)
	}
	branchA, err := Fork(ctx, loaded, &statefulTestOrganism{}, "branch-a")
	if err != nil {
		t.Fatal(err)
	}
	branchB, err := Fork(ctx, loaded, &statefulTestOrganism{}, "branch-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := branchA.Step(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := branchB.Step(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(branchA.World, branchB.World) || !reflect.DeepEqual(branchA.History, branchB.History) {
		t.Fatal("identically restored branches diverged under identical continuation")
	}
	branchA.World.Position = Point{X: 0, Y: 0}
	if branchA.World.Position == branchB.World.Position {
		t.Fatal("mutating one branch changed the other branch")
	}
	if session.World.Tick != checkpoint.Tick {
		t.Fatalf("branching mutated original session tick: %d vs %d", session.World.Tick, checkpoint.Tick)
	}
}

func TestForkInterventionIsRecordedAndDoesNotMutateItsSource(t *testing.T) {
	ctx := context.Background()
	session, err := NewSession(ctx, ExperimentConfig{Name: "nursery-test", World: Config{Seed: 41}}, &statefulTestOrganism{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Run(ctx, 2); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := session.CreateCheckpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	branch, err := ForkWithIntervention(ctx, checkpoint, &statefulTestOrganism{}, "branch-changed-cell",
		Intervention{ID: "intervention-1", Kind: "environment_change", Target: "cell:0,0", Description: "change one cell measurement"},
		func(_ context.Context, fork *Checkpoint) error {
			fork.World.Cells[0].Moisture = 0.001
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if branch.World.Cells[0].Moisture != 0.001 || session.World.Cells[0].Moisture == 0.001 {
		t.Fatal("fork intervention was not isolated from the source checkpoint")
	}
	if branch.ParentCheckpointID != checkpoint.ID || len(branch.Interventions) != 1 {
		t.Fatalf("fork ancestry/intervention was not retained: parent=%q interventions=%+v", branch.ParentCheckpointID, branch.Interventions)
	}
	if _, err := branch.Step(ctx); err != nil {
		t.Fatal(err)
	}
	branchedCheckpoint, err := branch.CreateCheckpoint(ctx)
	if err != nil {
		t.Fatalf("checkpoint after fork intervention: %v", err)
	}
	if err := ValidateCheckpoint(branchedCheckpoint); err != nil {
		t.Fatal(err)
	}
}

func TestActionAnalysisUsesRawCountsAndJSONLIsReconstructable(t *testing.T) {
	records := []TickRecord{
		{Tick: 1, Action: Action{Kind: "wait"}, Perception: Perception{Position: Point{X: 2, Y: 3}}, ViabilityAfter: Viability{Energy: 0.8}},
		{Tick: 2, Action: Action{Kind: "wait"}, Perception: Perception{Position: Point{X: 2, Y: 3}}, ViabilityAfter: Viability{Energy: 0.7}},
		{Tick: 3, Action: Action{Kind: "move", DX: 1}, Perception: Perception{Position: Point{X: 2, Y: 3}}, ViabilityAfter: Viability{Energy: 0.6}},
	}
	var encoded strings.Builder
	if err := WriteJSONL(&encoded, records); err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadJSONL(strings.NewReader(encoded.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(records) || decoded[1].ViabilityAfter.Energy != 0.7 {
		t.Fatalf("JSONL history did not reconstruct: %+v", decoded)
	}
	summary := Analyze(decoded)
	if summary.ActionCounts["wait"] != 2 || summary.ActionCounts["move"] != 1 || summary.ActionEntropy <= 0 {
		t.Fatalf("offline action measurements mismatch: %+v", summary)
	}
}

func TestTeacherInterfaceRemainsInactiveForBaselineConfiguration(t *testing.T) {
	ctx := context.Background()
	var interventions int
	session, err := NewSession(ctx, ExperimentConfig{
		Name: "nursery-baseline", World: Config{Seed: 12}, TeacherActive: false,
	}, &statefulTestOrganism{})
	if err != nil {
		t.Fatal(err)
	}
	session.Teacher = TeacherFunc(func(context.Context, uint64, Perception) (json.RawMessage, error) {
		interventions++
		return json.RawMessage(`{"signal":"present"}`), nil
	})
	record, err := session.Step(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if interventions != 0 || len(record.Perception.TeacherSignal) != 0 {
		t.Fatalf("baseline run received teacher intervention: calls=%d perception=%s", interventions, record.Perception.TeacherSignal)
	}
}

func TestActionSourceEventsRemainLinkedToEnvironmentalConsequences(t *testing.T) {
	session, err := NewSession(context.Background(), ExperimentConfig{
		Name: "lineage-test", World: Config{Seed: 84},
	}, &lineageTestOrganism{})
	if err != nil {
		t.Fatal(err)
	}
	record, err := session.Step(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Changes) == 0 || !slices.Contains(record.Changes[0].ParentEventIDs, "arachne:event:action-1") {
		t.Fatalf("world consequence lost action provenance: %+v", record.Changes)
	}
}

type statefulTestOrganism struct{ decisions int }

type lineageTestOrganism struct{}

func (*lineageTestOrganism) Decide(context.Context, Perception) (Action, CognitiveTrace, error) {
	return Action{Kind: "wait"}, CognitiveTrace{ActionSourceEventIDs: []string{"arachne:event:action-1"}}, nil
}

func (*lineageTestOrganism) SnapshotState(context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (*lineageTestOrganism) RestoreState(context.Context, json.RawMessage) error { return nil }

func findEffect(world *World, effect entityEffect) *entity {
	for index := range world.Entities {
		if world.Entities[index].Effect == effect {
			return &world.Entities[index]
		}
	}
	return nil
}

func (o *statefulTestOrganism) Decide(_ context.Context, _ Perception) (Action, CognitiveTrace, error) {
	o.decisions++
	return Action{Kind: "wait"}, CognitiveTrace{}, nil
}

func (o *statefulTestOrganism) SnapshotState(context.Context) (json.RawMessage, error) {
	return json.Marshal(map[string]int{"decisions": o.decisions})
}

func (o *statefulTestOrganism) RestoreState(_ context.Context, state json.RawMessage) error {
	if string(state) == "null" {
		o.decisions = 0
		return nil
	}
	var value struct {
		Decisions int `json:"decisions"`
	}
	if err := json.Unmarshal(state, &value); err != nil {
		return err
	}
	o.decisions = value.Decisions
	return nil
}
