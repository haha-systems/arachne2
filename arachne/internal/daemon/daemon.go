package daemon

import (
	"context"
	"log/slog"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
)

// Daemon owns the cancellable lifecycle of one Arachne organism process.
type Daemon struct {
	config Config
	logger *slog.Logger
	agents *agent.Supervisor
	events *cognition.Spine
}

// New validates configuration and creates a daemon using the supplied logger.
func New(config Config, logger *slog.Logger) (*Daemon, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	agents, err := agent.NewSupervisor(config.MailboxCapacity)
	if err != nil {
		return nil, err
	}
	eventStore, err := cognition.NewMemoryStore(config.EventCapacity)
	if err != nil {
		return nil, err
	}
	events, err := cognition.NewSpine(config.OrganismID, eventStore)
	if err != nil {
		return nil, err
	}
	return &Daemon{config: config, logger: logger, agents: agents, events: events}, nil
}

// Events exposes the daemon's shared cognitive event spine to its bound agents and host.
func (d *Daemon) Events() *cognition.Spine {
	return d.events
}

// RegisterAgent adds an agent before Run starts the supervised workers.
func (d *Daemon) RegisterAgent(id string, implementation agent.Agent) error {
	return d.agents.Register(id, implementation)
}

// Run starts the supervised workers and shuts them down when cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	if _, err := d.events.Emit(ctx, cognition.Draft{
		Kind:    cognition.KindDevelopment,
		Payload: []byte(`{"lifecycle":"starting"}`),
	}); err != nil {
		return err
	}
	if err := d.agents.Start(ctx); err != nil {
		return err
	}
	d.logger.Info("Arachne daemon ready",
		"event", "daemon.ready",
		"organism_id", d.config.OrganismID,
	)
	reason := "context_canceled"
	select {
	case <-ctx.Done():
	case <-d.agents.Done():
		reason = "agents_finished"
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), d.config.ShutdownTimeout)
	defer cancel()
	if err := d.agents.Stop(shutdownCtx); err != nil {
		d.logger.Error("Arachne daemon shutdown incomplete",
			"event", "daemon.shutdown_failed",
			"organism_id", d.config.OrganismID,
			"error", err,
		)
		return err
	}
	if _, err := d.events.Emit(shutdownCtx, cognition.Draft{
		Kind:    cognition.KindDevelopment,
		Payload: []byte(`{"lifecycle":"stopped","reason":"` + reason + `"}`),
	}); err != nil {
		return err
	}
	d.logger.Info("Arachne daemon stopped",
		"event", "daemon.stopped",
		"organism_id", d.config.OrganismID,
		"reason", reason,
	)
	return nil
}
