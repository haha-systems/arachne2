// Command nursery validates and analyzes the isolated Experiment 002 environment.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/haha-systems/arachne2/experiments/nursery"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "validate":
		if err := validate(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "analyze":
		if err := analyze(os.Args[2:]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func validate(arguments []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	seed := flags.Uint64("seed", 2026002, "world-generation seed")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	world, err := nursery.NewWorld(nursery.Config{Seed: *seed})
	if err != nil {
		return err
	}
	perception := world.Perceive()
	duplicate, err := nursery.NewWorld(nursery.Config{Seed: *seed})
	if err != nil {
		return err
	}
	worldJSON, err := json.Marshal(world)
	if err != nil {
		return err
	}
	duplicateJSON, err := json.Marshal(duplicate)
	if err != nil {
		return err
	}
	if string(worldJSON) != string(duplicateJSON) {
		return fmt.Errorf("seed %d did not reproduce the same world state", *seed)
	}
	encoded, err := json.Marshal(perception)
	if err != nil {
		return err
	}
	if len(perception.Cells) >= len(world.Cells) || len(perception.Entities) >= len(world.Entities) {
		return fmt.Errorf("seed %d produced a non-local perception", *seed)
	}
	if err := nursery.ValidateAction(nursery.Action{Kind: "wait"}); err != nil {
		return err
	}
	if err := nursery.ValidateAction(nursery.Action{Kind: "shell"}); err == nil {
		return fmt.Errorf("sealed action boundary accepted an undeclared action")
	}
	organism := &validationOrganism{}
	session, err := nursery.NewSession(context.Background(), nursery.ExperimentConfig{
		Name: "environment-validation", World: nursery.Config{Seed: *seed}, TeacherActive: false,
	}, organism)
	if err != nil {
		return err
	}
	if _, err := session.Run(context.Background(), 3); err != nil {
		return err
	}
	checkpoint, err := session.CreateCheckpoint(context.Background())
	if err != nil {
		return err
	}
	branch, err := nursery.Fork(context.Background(), checkpoint, &validationOrganism{}, "validation-branch")
	if err != nil {
		return err
	}
	if _, err := branch.Step(context.Background()); err != nil {
		return err
	}
	result := map[string]any{
		"experiment": "002-nursery-environment-validation",
		"seed":       *seed,
		"checks": map[string]any{
			"deterministic_generation": true,
			"local_perception":         true,
			"world_persistence":        session.World.Tick == 3,
			"checkpoint_restore":       branch.World.Tick == 4,
			"checkpoint_branching":     branch.BranchID == "validation-branch" && session.World.Tick == 3,
			"sealed_action_boundary":   true,
			"teacher_inactive":         !session.Config.TeacherActive,
			"perception_bytes":         len(encoded),
		},
		"world": map[string]any{
			"width": world.Config.Width, "height": world.Config.Height,
			"visible_cells": len(perception.Cells), "total_cells": len(world.Cells),
			"visible_entities": len(perception.Entities), "total_entities": len(world.Entities),
		},
		"checkpoint_id": checkpoint.ID,
		"note":          "environment validation uses a wait-only test adapter; this is not an Arachne pilot",
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func analyze(arguments []string) error {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	input := flags.String("input", "", "JSONL tick history")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *input == "" {
		return fmt.Errorf("-input is required")
	}
	file, err := os.Open(*input)
	if err != nil {
		return err
	}
	history, err := nursery.ReadJSONL(file)
	if err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close tick history: %w", err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(nursery.Analyze(history))
}

type validationOrganism struct{ ticks uint64 }

func (o *validationOrganism) Decide(context.Context, nursery.Perception) (nursery.Action, nursery.CognitiveTrace, error) {
	o.ticks++
	return nursery.Action{Kind: "wait"}, nursery.CognitiveTrace{}, nil
}

func (o *validationOrganism) SnapshotState(context.Context) (json.RawMessage, error) {
	return json.Marshal(o.ticks)
}

func (o *validationOrganism) RestoreState(_ context.Context, state json.RawMessage) error {
	return json.Unmarshal(state, &o.ticks)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: nursery <validate|analyze> [flags]")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
