// Package main runs and reports Experiment 001 workspace ablations.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/haha-systems/arachne2/internal/experiment"
)

const experimentName = "workspace-ablation"

func main() {
	if err := runCLI(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func runCLI(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: workspace-ablation <run|compare> [flags]")
	}
	switch args[0] {
	case "run":
		flags := flag.NewFlagSet("run", flag.ContinueOnError)
		configPath := flags.String("config", "experiments/workspace-ablation.json", "experiment JSON configuration")
		outputPath := flags.String("out", "experiments/results/experiment-001-workspace-ablation.jsonl", "JSONL result path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		cfg, err := loadConfig(*configPath)
		if err != nil {
			return err
		}
		if err := validateConfig(cfg); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(*outputPath), 0o750); err != nil {
			return err
		}
		file, err := os.Create(*outputPath)
		if err != nil {
			return err
		}
		runErr := experiment.Run(ctx, file, cfg.Experiment, trialExecutor(cfg.Controls))
		closeErr := file.Close()
		if runErr != nil {
			return runErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Printf("wrote %s with %d paired seeds\n", *outputPath, cfg.Experiment.Trials)
		return nil
	case "compare":
		flags := flag.NewFlagSet("compare", flag.ContinueOnError)
		inputPath := flags.String("input", "experiments/results/experiment-001-workspace-ablation.jsonl", "JSONL result path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		summary, err := compareFile(*inputPath)
		if err != nil {
			return err
		}
		return printComparison(os.Stdout, summary)
	default:
		return fmt.Errorf("unknown command %q; expected run or compare", args[0])
	}
}
