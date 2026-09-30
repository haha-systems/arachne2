// Package cognition provides the shared, append-only event model for Arachne cognition.
package cognition

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	KindPerception = "perception"
	KindActivation = "activation"
	KindProposal   = "proposal"
	KindSelection  = "selection"
	KindDecision   = "decision"
	KindAction     = "action"
	// KindOutcome records the environment result of an attempted action or explicit no-op.
	KindOutcome         = "outcome"
	KindPredictionError = "prediction_error"
	KindReplay          = "replay"
	KindRegulation      = "regulation"
	KindGovernance      = "governance"
	KindDevelopment     = "development"
	KindMemory          = "memory"
	KindLearning        = "learning"
	KindSilkTrace       = "silk_trace"
)

// ErrEventStoreFull reports that a bounded event store cannot append another record.
var ErrEventStoreFull = errors.New("cognitive event store is full")

// Event is one attributed, ordered record in an organism's cognitive history.
type Event struct {
	SchemaVersion    string          `json:"schema_version"`
	EventID          string          `json:"event_id"`
	Sequence         uint64          `json:"sequence"`
	OccurredAt       time.Time       `json:"occurred_at"`
	OrganismID       string          `json:"organism_id"`
	BranchID         string          `json:"branch_id,omitempty"`
	BranchInstanceID string          `json:"branch_instance_id,omitempty"`
	AgentID          string          `json:"agent_id,omitempty"`
	SessionID        string          `json:"session_id,omitempty"`
	CorrelationID    string          `json:"correlation_id,omitempty"`
	ParentEventIDs   []string        `json:"parent_event_ids,omitempty"`
	Kind             string          `json:"kind"`
	Payload          json.RawMessage `json:"payload"`
	Silk             *SilkTraceRef   `json:"silk,omitempty"`
}

// BranchOrigin records a fork edge without adding branch labels to cognitive input.
type BranchOrigin struct {
	BranchID         string `json:"branch_id"`
	InstanceID       string `json:"instance_id"`
	ParentBranchID   string `json:"parent_branch_id,omitempty"`
	ParentInstanceID string `json:"parent_instance_id,omitempty"`
	ForkEventID      string `json:"fork_event_id"`
}

// EventSnapshot is a stable history envelope for one continuation point.
type EventSnapshot struct {
	SchemaVersion    string         `json:"schema_version"`
	OrganismID       string         `json:"organism_id"`
	Sequence         uint64         `json:"sequence"`
	Capacity         int            `json:"capacity,omitempty"`
	BranchID         string         `json:"branch_id,omitempty"`
	BranchInstanceID string         `json:"branch_instance_id,omitempty"`
	Lineage          []BranchOrigin `json:"lineage,omitempty"`
	Events           []Event        `json:"events"`
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

// Capacity returns the fixed event capacity used by this store.
func (s *MemoryStore) Capacity() int { return s.capacity }

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
	event.Silk = CloneSilkTraceRef(event.Silk)
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
		copyEvent.Silk = CloneSilkTraceRef(event.Silk)
		result = append(result, copyEvent)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

// CloneSilkTraceRef returns an independent copy of a Silk trace reference.
func CloneSilkTraceRef(ref *SilkTraceRef) *SilkTraceRef {
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
	branchID   string
	instanceID string
	lineage    []BranchOrigin
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
	eventID := fmt.Sprintf("%s:event:%d", s.organismID, next)
	if s.branchID != "" {
		eventID = fmt.Sprintf("%s:branch:%s:event:%d", s.organismID, s.instanceID, next)
	}
	event := Event{
		SchemaVersion:    SchemaVersion,
		EventID:          eventID,
		Sequence:         next,
		OccurredAt:       s.now().UTC(),
		OrganismID:       s.organismID,
		BranchID:         s.branchID,
		BranchInstanceID: s.instanceID,
		AgentID:          draft.AgentID,
		SessionID:        draft.SessionID,
		CorrelationID:    draft.CorrelationID,
		ParentEventIDs:   append([]string(nil), draft.ParentEventIDs...),
		Kind:             draft.Kind,
		Payload:          append(json.RawMessage(nil), payload...),
		Silk:             draft.Silk,
	}
	if err := s.store.Append(ctx, event); err != nil {
		return Event{}, err
	}
	s.sequence = next
	return event, nil
}

// EmitJSON encodes a value as an event payload and appends it to the spine.
func (s *Spine) EmitJSON(ctx context.Context, draft Draft, value any) (Event, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	draft.Payload = payload
	return s.Emit(ctx, draft)
}

// Read returns events after a global sequence using the configured store.
func (s *Spine) Read(ctx context.Context, after uint64, limit int) ([]Event, error) {
	return s.store.Read(ctx, after, limit)
}

// Snapshot exports complete event history and branch ancestry as plain data.
func (s *Spine) Snapshot(ctx context.Context) (EventSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]Event, 0, s.sequence)
	var after uint64
	for {
		batch, err := s.store.Read(ctx, after, 256)
		if err != nil {
			return EventSnapshot{}, fmt.Errorf("read event history: %w", err)
		}
		for _, event := range batch {
			events = append(events, cloneEvent(event))
			after = event.Sequence
		}
		if len(batch) < 256 {
			break
		}
	}
	if uint64(len(events)) != s.sequence {
		return EventSnapshot{}, errors.New("event history sequence is incomplete")
	}
	capacity := 0
	if bounded, ok := s.store.(interface{ Capacity() int }); ok {
		capacity = bounded.Capacity()
	}
	return EventSnapshot{
		SchemaVersion: "arachne.event_snapshot.v1", OrganismID: s.organismID,
		Sequence: s.sequence, Capacity: capacity, BranchID: s.branchID,
		BranchInstanceID: s.instanceID, Lineage: append([]BranchOrigin(nil), s.lineage...), Events: events,
	}, nil
}

// Restore imports a complete history into an empty store and creates an independent branch.
func (s *Spine) Restore(ctx context.Context, snapshot EventSnapshot, branchID string) error {
	if snapshot.SchemaVersion != "arachne.event_snapshot.v1" || snapshot.OrganismID != s.organismID {
		return errors.New("event snapshot schema or organism identity is incompatible")
	}
	if strings.TrimSpace(branchID) == "" {
		return errors.New("branch ID is required")
	}
	if uint64(len(snapshot.Events)) != snapshot.Sequence {
		return errors.New("event snapshot is missing required history")
	}
	for i, event := range snapshot.Events {
		if event.SchemaVersion != SchemaVersion || event.OrganismID != snapshot.OrganismID ||
			event.Sequence != uint64(i+1) || event.EventID == "" || event.Kind == "" ||
			len(event.Payload) == 0 || !json.Valid(event.Payload) {
			return fmt.Errorf("event snapshot contains malformed event at index %d", i)
		}
	}
	existing, err := s.store.Read(ctx, 0, 1)
	if err != nil {
		return fmt.Errorf("check event store before restore: %w", err)
	}
	if len(existing) != 0 {
		return errors.New("event store must be empty before restore")
	}
	lineage := append([]BranchOrigin(nil), snapshot.Lineage...)
	for _, origin := range lineage {
		if origin.BranchID == branchID {
			return errors.New("branch ID already appears in event ancestry")
		}
	}
	parentBranch := snapshot.BranchID
	forkEvent := ""
	if snapshot.Sequence > 0 {
		forkEvent = snapshot.Events[len(snapshot.Events)-1].EventID
	}
	var branchToken [16]byte
	if _, err := rand.Read(branchToken[:]); err != nil {
		return fmt.Errorf("generate branch event namespace: %w", err)
	}
	instanceID := hex.EncodeToString(branchToken[:])
	lineage = append(lineage, BranchOrigin{
		BranchID: branchID, InstanceID: instanceID,
		ParentBranchID: parentBranch, ParentInstanceID: snapshot.BranchInstanceID, ForkEventID: forkEvent,
	})
	importer, ok := s.store.(EventStateStore)
	if !ok {
		return errors.New("event store does not support isolated snapshot restore")
	}
	if snapshot.Capacity > 0 {
		bounded, ok := s.store.(interface{ Capacity() int })
		if !ok || bounded.Capacity() != snapshot.Capacity {
			return errors.New("event store capacity does not match snapshot")
		}
	}
	if err := importer.ReplaceEvents(ctx, snapshot.Events); err != nil {
		return fmt.Errorf("restore event history: %w", err)
	}
	s.mu.Lock()
	s.sequence, s.branchID, s.instanceID, s.lineage = snapshot.Sequence, branchID, instanceID, lineage
	s.mu.Unlock()
	return nil
}

// EventStateStore atomically imports history into a fresh event store.
type EventStateStore interface {
	ReplaceEvents(context.Context, []Event) error
}

// ReplaceEvents imports a complete history only into an empty bounded store.
func (s *MemoryStore) ReplaceEvents(ctx context.Context, events []Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events) != 0 {
		return errors.New("event store must be empty before restore")
	}
	if len(events) > s.capacity {
		return ErrEventStoreFull
	}
	cloned := make([]Event, len(events))
	for i := range events {
		cloned[i] = cloneEvent(events[i])
	}
	s.events = cloned
	return nil
}

func cloneEvent(event Event) Event {
	event.Payload = append(json.RawMessage(nil), event.Payload...)
	event.ParentEventIDs = append([]string(nil), event.ParentEventIDs...)
	event.Silk = CloneSilkTraceRef(event.Silk)
	return event
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
