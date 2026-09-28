package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	// ErrSupervisorStarted indicates that agent registration is no longer allowed.
	ErrSupervisorStarted = errors.New("agent supervisor has already started")
	// ErrSupervisorNotStarted indicates that the operation requires a running supervisor.
	ErrSupervisorNotStarted = errors.New("agent supervisor has not started")
)

// Supervisor starts, cancels, and joins a fixed set of agents.
type Supervisor struct {
	mu       sync.Mutex
	agents   map[string]Agent
	router   *router
	started  bool
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
	firstErr error
}

// NewSupervisor creates a supervisor with bounded per-agent mailboxes.
func NewSupervisor(mailboxCapacity int) (*Supervisor, error) {
	if mailboxCapacity < 1 {
		return nil, errors.New("mailbox capacity must be positive")
	}
	return &Supervisor{
		agents: make(map[string]Agent),
		router: newRouter(mailboxCapacity),
		done:   make(chan struct{}),
	}, nil
}

// Register adds an agent before Start. Agent IDs must be unique and nonempty.
func (s *Supervisor) Register(id string, agent Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrSupervisorStarted
	}
	if strings.TrimSpace(id) == "" {
		return errors.New("agent ID must not be empty")
	}
	if agent == nil {
		return errors.New("agent implementation must not be nil")
	}
	if _, exists := s.agents[id]; exists {
		return fmt.Errorf("agent %q: already registered", id)
	}
	s.agents[id] = agent
	return nil
}

// Start registers every inbox and starts agents in sorted ID order.
func (s *Supervisor) Start(parent context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return ErrSupervisorStarted
	}
	ids := make([]string, 0, len(s.agents))
	for id := range s.agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	inboxes := make(map[string]<-chan Message, len(ids))
	for _, id := range ids {
		inbox, err := s.router.register(id)
		if err != nil {
			return err
		}
		inboxes[id] = inbox
	}

	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.started = true
	var workers sync.WaitGroup
	workers.Add(len(ids))
	for _, id := range ids {
		id := id
		implementation := s.agents[id]
		go func() {
			defer workers.Done()
			sender := agentSender{from: id, router: s.router}
			if err := implementation.Run(ctx, inboxes[id], sender); err != nil &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, context.DeadlineExceeded) {
				s.recordFailure(fmt.Errorf("agent %q: %w", id, err))
			}
		}()
	}
	go func() {
		if len(ids) == 0 {
			<-ctx.Done()
		} else {
			workers.Wait()
		}
		s.router.close()
		cancel()
		close(s.done)
	}()
	return nil
}

// Done returns a channel closed after all supervised agents have exited.
func (s *Supervisor) Done() <-chan struct{} {
	return s.done
}

// Send sends one message through the running supervisor's bounded router.
func (s *Supervisor) Send(ctx context.Context, from, to, kind string, payload json.RawMessage) error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return ErrSupervisorNotStarted
	}
	return s.router.send(ctx, from, to, kind, payload)
}

// Stop cancels all agents and waits for them to return or for ctx to expire.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return ErrSupervisorNotStarted
	}
	cancel := s.cancel
	s.mu.Unlock()
	s.stopOnce.Do(func() {
		cancel()
		s.router.close()
	})
	return s.Wait(ctx)
}

// Wait blocks until every agent has returned or ctx expires.
func (s *Supervisor) Wait(ctx context.Context) error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return ErrSupervisorNotStarted
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.firstErr
	}
}

func (s *Supervisor) recordFailure(err error) {
	s.mu.Lock()
	if s.firstErr == nil {
		s.firstErr = err
	}
	cancel := s.cancel
	s.mu.Unlock()
	s.stopOnce.Do(func() {
		cancel()
		s.router.close()
	})
}

type agentSender struct {
	from   string
	router *router
}

func (s agentSender) Send(ctx context.Context, to, kind string, payload json.RawMessage) error {
	return s.router.send(ctx, s.from, to, kind, payload)
}
