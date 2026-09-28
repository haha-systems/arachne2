// Package cognition provides the shared, append-only event model for Arachne cognition.
package cognition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SchemaVersion identifies the serialized cognitive event envelope.
const SchemaVersion = "arachne.cognitive_event.v1"

// Event kinds are strings so later cognitive subsystems can extend the vocabulary.
const (
	KindPerception      = "perception"
	KindActivation      = "activation"
	KindProposal        = "proposal"
	KindSelection       = "selection"
	KindDecision        = "decision"
	KindAction          = "action"
	KindPredictionError = "prediction_error"
	KindReplay          = "replay"
	KindRegulation      = "regulation"
	KindGovernance      = "governance"
	KindDevelopment     = "development"
	KindSilkTrace       = "silk_trace"
)

// ErrEventStoreFull reports that a bounded event store cannot append another record.
var ErrEventStoreFull = errors.New("cognitive event store is full")

// Event is one attributed, ordered record in an organism's cognitive history.
type Event struct {
	SchemaVersion  string          `json:"schema_version"`
	EventID        string          `json:"event_id"`
	Sequence       uint64          `json:"sequence"`
	OccurredAt     time.Time       `json:"occurred_at"`
	OrganismID     string          `json:"organism_id"`
	AgentID        string          `json:"agent_id,omitempty"`
	SessionID      string          `json:"session_id,omitempty"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	ParentEventIDs []string        `json:"parent_event_ids,omitempty"`
	Kind           string          `json:"kind"`
	Payload        json.RawMessage `json:"payload"`
	Silk           *SilkTraceRef   `json:"silk,omitempty"`
}

// SilkTraceRef links a cognitive event to one exact event from the Silk trace stream.
type SilkTraceRef struct {
	TraceID       string  `json:"trace_id,omitempty"`
	Sequence      *uint64 `json:"sequence,omitempty"`
	CorrelationID string  `json:"correlation_id,omitempty"`
	ParentCallID  string  `json:"parent_correlation_id,omitempty"`
	Procedure     string  `json:"procedure,omitempty"`
	EventKind     string  `json:"event_kind,omitempty"`
}

// Draft is the caller-supplied content from which Spine assigns identity and order.
type Draft struct {
	AgentID        string
	SessionID      string
	CorrelationID  string
	ParentEventIDs []string
	Kind           string
	Payload        json.RawMessage
	Silk           *SilkTraceRef
}

// EventStore persists events in append order and returns them by sequence.
type EventStore interface {
	Append(context.Context, Event) error
	Read(context.Context, uint64, int) ([]Event, error)
}

// MemoryStore is a bounded in-memory event history; it never evicts silently.
type MemoryStore struct {
	mu       sync.RWMutex
	capacity int
	events   []Event
}

// NewMemoryStore creates an event history with a fixed maximum record count.
func NewMemoryStore(capacity int) (*MemoryStore, error) {
	if capacity < 1 {
		return nil, errors.New("event store capacity must be positive")
	}
	return &MemoryStore{capacity: capacity, events: make([]Event, 0, capacity)}, nil
}

// Append stores the next event unless the bounded history is full.
func (s *MemoryStore) Append(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) >= s.capacity {
		return ErrEventStoreFull
	}
	event.Payload = append(json.RawMessage(nil), event.Payload...)
	event.ParentEventIDs = append([]string(nil), event.ParentEventIDs...)
	event.Silk = cloneSilkTraceRef(event.Silk)
	s.events = append(s.events, event)
	return nil
}

// Read returns events after a sequence, capped by limit; limit must be positive.
func (s *MemoryStore) Read(ctx context.Context, after uint64, limit int) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, errors.New("event read limit must be positive")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Event, 0, limit)
	for _, event := range s.events {
		if event.Sequence <= after {
			continue
		}
		copyEvent := event
		copyEvent.Payload = append(json.RawMessage(nil), event.Payload...)
		copyEvent.ParentEventIDs = append([]string(nil), event.ParentEventIDs...)
		copyEvent.Silk = cloneSilkTraceRef(event.Silk)
		result = append(result, copyEvent)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func cloneSilkTraceRef(ref *SilkTraceRef) *SilkTraceRef {
	if ref == nil {
		return nil
	}
	clone := *ref
	if ref.Sequence != nil {
		sequence := *ref.Sequence
		clone.Sequence = &sequence
	}
	return &clone
}

// Spine assigns stable IDs, a global monotonic sequence, timestamps, and organism attribution.
type Spine struct {
	mu         sync.Mutex
	organismID string
	sequence   uint64
	store      EventStore
	now        func() time.Time
}

// NewSpine creates a shared event spine for one organism.
func NewSpine(organismID string, store EventStore) (*Spine, error) {
	if strings.TrimSpace(organismID) == "" {
		return nil, errors.New("organism ID must not be empty")
	}
	if store == nil {
		return nil, errors.New("event store must not be nil")
	}
	return &Spine{organismID: organismID, store: store, now: time.Now}, nil
}

// Emit stores one event and returns its assigned identity and sequence.
func (s *Spine) Emit(ctx context.Context, draft Draft) (Event, error) {
	if strings.TrimSpace(draft.Kind) == "" {
		return Event{}, errors.New("event kind must not be empty")
	}
	payload := draft.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return Event{}, errors.New("event payload must be valid JSON")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.sequence + 1
	event := Event{
		SchemaVersion:  SchemaVersion,
		EventID:        fmt.Sprintf("%s:event:%d", s.organismID, next),
		Sequence:       next,
		OccurredAt:     s.now().UTC(),
		OrganismID:     s.organismID,
		AgentID:        draft.AgentID,
		SessionID:      draft.SessionID,
		CorrelationID:  draft.CorrelationID,
		ParentEventIDs: append([]string(nil), draft.ParentEventIDs...),
		Kind:           draft.Kind,
		Payload:        append(json.RawMessage(nil), payload...),
		Silk:           draft.Silk,
	}
	if err := s.store.Append(ctx, event); err != nil {
		return Event{}, err
	}
	s.sequence = next
	return event, nil
}

// Read returns events after a global sequence using the configured store.
func (s *Spine) Read(ctx context.Context, after uint64, limit int) ([]Event, error) {
	return s.store.Read(ctx, after, limit)
}

// RecordSilkTrace wraps one structured Silk event with Arachne attribution and parent links.
func (s *Spine) RecordSilkTrace(ctx context.Context, agentID, sessionID, correlationID string, parents []string, trace json.RawMessage) (Event, error) {
	var silkTrace struct {
		TraceID       string          `json:"trace_id"`
		Sequence      *uint64         `json:"sequence"`
		CorrelationID string          `json:"correlation_id"`
		ParentCallID  string          `json:"parent_correlation_id"`
		Procedure     string          `json:"procedure"`
		Event         json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(trace, &silkTrace); err != nil {
		return Event{}, fmt.Errorf("decode Silk trace event: %w", err)
	}
	eventKind := ""
	var tagged struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(silkTrace.Event, &tagged) == nil {
		eventKind = tagged.Kind
	}
	return s.Emit(ctx, Draft{
		AgentID: agentID, SessionID: sessionID, CorrelationID: correlationID,
		ParentEventIDs: parents, Kind: KindSilkTrace, Payload: trace,
		Silk: &SilkTraceRef{
			TraceID: silkTrace.TraceID, Sequence: silkTrace.Sequence,
			CorrelationID: silkTrace.CorrelationID, ParentCallID: silkTrace.ParentCallID,
			Procedure: silkTrace.Procedure, EventKind: eventKind,
		},
	})
}
