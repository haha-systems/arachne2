// Package main starts the Arachne daemon command.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/haha-systems/arachne2/internal/daemon"
)

func main() {
	os.Exit(execute())
}

func execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, os.Args[1:], os.Stdout, os.Stderr)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	config := daemon.DefaultConfig()
	flags := flag.NewFlagSet("arachned", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config.OrganismID, "organism-id", config.OrganismID, "organism identifier")
	flags.StringVar(&config.LogLevel, "log-level", config.LogLevel, "debug, info, warn, or error")
	flags.IntVar(&config.MailboxCapacity, "mailbox-capacity", config.MailboxCapacity, "messages buffered per agent")
	flags.DurationVar(&config.ShutdownTimeout, "shutdown-timeout", config.ShutdownTimeout, "maximum time to wait for agents to stop")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		if _, err := fmt.Fprintln(stderr, "arachned does not accept positional arguments"); err != nil {
			return 1
		}
		return 2
	}

	level, err := daemon.ParseLogLevel(config.LogLevel)
	if err != nil {
		if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
			return 1
		}
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: level}))
	runtime, err := daemon.New(config, logger)
	if err != nil {
		if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
			return 1
		}
		return 2
	}
	if err := runtime.Run(ctx); err != nil {
		if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
			return 1
		}
		return 1
	}
	return 0
}
