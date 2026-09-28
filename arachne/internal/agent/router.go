package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	// ErrAgentNotFound indicates that no active inbox is registered for a recipient.
	ErrAgentNotFound = errors.New("agent inbox is not registered")
	// ErrRouterClosed indicates that the supervisor has stopped message delivery.
	ErrRouterClosed = errors.New("agent message router is closed")
)

type mailbox struct {
	messages chan Message
	sendMu   sync.Mutex
}

type router struct {
	mu        sync.RWMutex
	mailboxes map[string]*mailbox
	capacity  int
	sequence  atomic.Uint64
	closed    chan struct{}
	closeOnce sync.Once
}

func newRouter(capacity int) *router {
	return &router{
		mailboxes: make(map[string]*mailbox),
		capacity:  capacity,
		closed:    make(chan struct{}),
	}
}

func (r *router) register(id string) (<-chan Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.mailboxes[id]; exists {
		return nil, fmt.Errorf("agent %q: already registered", id)
	}
	box := &mailbox{messages: make(chan Message, r.capacity)}
	r.mailboxes[id] = box
	return box.messages, nil
}

func (r *router) send(ctx context.Context, from, to, kind string, payload json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(from) == "" {
		return errors.New("message sender must not be empty")
	}
	if strings.TrimSpace(kind) == "" {
		return errors.New("message kind must not be empty")
	}
	if !json.Valid(payload) {
		return errors.New("message payload must be valid JSON")
	}
	r.mu.RLock()
	box := r.mailboxes[to]
	r.mu.RUnlock()
	if box == nil {
		return fmt.Errorf("recipient %q: %w", to, ErrAgentNotFound)
	}
	box.sendMu.Lock()
	defer box.sendMu.Unlock()
	sequence := r.sequence.Add(1)
	message := Message{
		Sequence:      sequence,
		CorrelationID: fmt.Sprintf("message-%d", sequence),
		From:          from,
		To:            to,
		Kind:          kind,
		Payload:       append(json.RawMessage(nil), payload...),
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.closed:
		return ErrRouterClosed
	case box.messages <- message:
		select {
		case <-r.closed:
			return ErrRouterClosed
		default:
		}
		return nil
	}
}

func (r *router) close() {
	r.closeOnce.Do(func() {
		close(r.closed)
	})
}
