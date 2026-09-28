package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
)

func TestRunReportsReadinessAndStopsOnCancellation(t *testing.T) {
	reader, writer := io.Pipe()
	daemon, err := New(DefaultConfig(), slog.New(slog.NewJSONHandler(writer, nil)))
	if err != nil {
		t.Fatalf("create daemon: %v", err)
	}
	agentStopped := make(chan struct{})
	if err := daemon.RegisterAgent("worker", agent.Func(func(ctx context.Context, _ <-chan agent.Message, _ agent.Sender) error {
		<-ctx.Done()
		close(agentStopped)
		return ctx.Err()
	})); err != nil {
		t.Fatalf("register agent: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.Run(ctx)
	}()
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close log reader: %v", err)
		}
	}()

	logReader := bufio.NewReader(reader)
	assertEvent := func(want string) {
		t.Helper()
		line, readErr := logReader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read %s event: %v", want, readErr)
		}
		var event struct {
			Event      string `json:"event"`
			OrganismID string `json:"organism_id"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode %s event: %v", want, err)
		}
		if event.Event != want || event.OrganismID != "arachne-local" {
			t.Fatalf("unexpected lifecycle event: %+v", event)
		}
	}

	assertEvent("daemon.ready")
	cancel()
	assertEvent("daemon.stopped")
	select {
	case <-agentStopped:
	case <-time.After(time.Second):
		t.Fatal("daemon did not stop its agent before shutting down")
	}
	if err := <-done; err != nil {
		t.Fatalf("run daemon: %v", err)
	}
}

func TestConfigRejectsUnsupportedLogLevel(t *testing.T) {
	config := DefaultConfig()
	config.LogLevel = "trace"
	if err := config.Validate(); err == nil {
		t.Fatal("expected unsupported log level to fail validation")
	}
}
