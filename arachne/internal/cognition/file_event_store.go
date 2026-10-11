package cognition

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"sync"
)

const eventLogFormat = "arachne.event_log.v1"

var (
	// ErrEventLogVersionMismatch identifies an unsupported log or event schema version.
	ErrEventLogVersionMismatch = errors.New("event log version mismatch")
	// ErrEventLogOrganismMismatch identifies a log or event for another organism.
	ErrEventLogOrganismMismatch = errors.New("event log organism mismatch")
	// ErrDuplicateEventSequence identifies a sequence already present in the log.
	ErrDuplicateEventSequence = errors.New("duplicate event sequence")
	// ErrNonMonotonicEventSequence identifies a sequence that skips or reverses order.
	ErrNonMonotonicEventSequence = errors.New("non-monotonic event sequence")
	// ErrEventLogCorrupt identifies malformed or checksum-invalid log records.
	ErrEventLogCorrupt = errors.New("event log is corrupt")
	// ErrEventLogWrite identifies a write or sync failure whose rollback succeeded.
	ErrEventLogWrite = errors.New("event log write failed")
	// ErrEventLogRollback identifies a failed rollback that leaves disk state uncertain.
	ErrEventLogRollback = errors.New("event log rollback failed")
	// ErrEventLogClosed identifies operations attempted after Close.
	ErrEventLogClosed = errors.New("event log is closed")
)

// EventStoreError identifies a durable event log failure and its location.
type EventStoreError struct {
	Kind     error
	Line     int
	Sequence uint64
	Expected uint64
	Cause    error
}

func (e *EventStoreError) Error() string {
	message := e.Kind.Error()
	if e.Line > 0 {
		message = fmt.Sprintf("%s at line %d", message, e.Line)
	}
	if e.Sequence != 0 || e.Expected != 0 {
		message = fmt.Sprintf("%s: sequence %d, expected %d", message, e.Sequence, e.Expected)
	}
	if e.Cause != nil {
		message = fmt.Sprintf("%s: %v", message, e.Cause)
	}
	return message
}

// Unwrap exposes the low-level cause while Is matches the typed failure kind.
func (e *EventStoreError) Unwrap() error { return e.Cause }

// Is reports whether the error has the requested event log failure kind.
func (e *EventStoreError) Is(target error) bool { return e.Kind == target }

// FileEventStoreRepairInfo describes a final incomplete line removed on open.
type FileEventStoreRepairInfo struct {
	Repaired       bool
	TruncatedBytes int64
	TruncatedFrom  int64
}

type eventLogHeader struct {
	Format     string `json:"format"`
	OrganismID string `json:"organism_id"`
}

type eventLogRecord struct {
	Event    Event  `json:"event"`
	Checksum string `json:"checksum"`
}

// FileEventStore is a bounded append-only JSONL event log. It assumes one
// process is the only writer to its file.
type FileEventStore struct {
	mu         sync.RWMutex
	file       *os.File
	organismID string
	capacity   int
	events     []Event
	repair     FileEventStoreRepairInfo
	poisoned   error
	closed     bool
	writeFile  func([]byte) (int, error)
	syncFile   func() error
}

var _ EventStore = (*FileEventStore)(nil)

// OpenFileEventStore opens or creates an organism-bound durable event log.
func OpenFileEventStore(path, organismID string, capacity int) (*FileEventStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("event log path must not be empty")
	}
	if strings.TrimSpace(organismID) == "" {
		return nil, errors.New("organism ID must not be empty")
	}
	if capacity < 1 {
		return nil, errors.New("event store capacity must be positive")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}
	store := &FileEventStore{
		file: file, organismID: organismID, capacity: capacity,
		events:    make([]Event, 0, capacity),
		writeFile: file.Write, syncFile: file.Sync,
	}
	closeOnError := func(err error) (*FileEventStore, error) {
		_ = file.Close()
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return closeOnError(fmt.Errorf("stat event log: %w", err))
	}
	if info.Size() == 0 {
		if err := file.Chmod(0o600); err != nil {
			return closeOnError(fmt.Errorf("secure event log: %w", err))
		}
		if err := store.writeInitialHeader(); err != nil {
			return closeOnError(err)
		}
		return store, nil
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return closeOnError(fmt.Errorf("seek event log: %w", err))
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return closeOnError(fmt.Errorf("read event log: %w", err))
	}
	repairFrom, hasTail, err := store.recover(contents)
	if err != nil {
		return closeOnError(err)
	}
	if len(store.events) > capacity {
		return closeOnError(ErrEventStoreFull)
	}
	if hasTail {
		truncated := int64(len(contents)) - repairFrom
		if err := file.Truncate(repairFrom); err != nil {
			return closeOnError(&EventStoreError{Kind: ErrEventLogWrite, Cause: fmt.Errorf("truncate incomplete final line: %w", err)})
		}
		if err := file.Sync(); err != nil {
			return closeOnError(&EventStoreError{Kind: ErrEventLogWrite, Cause: fmt.Errorf("sync repaired event log: %w", err)})
		}
		store.repair = FileEventStoreRepairInfo{Repaired: true, TruncatedBytes: truncated, TruncatedFrom: repairFrom}
	}
	if err := file.Chmod(0o600); err != nil {
		return closeOnError(fmt.Errorf("secure event log: %w", err))
	}
	return store, nil
}

// Capacity returns the fixed maximum number of events retained by the store.
func (s *FileEventStore) Capacity() int { return s.capacity }

// RepairInfo reports whether opening the store truncated an incomplete tail.
func (s *FileEventStore) RepairInfo() FileEventStoreRepairInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repair
}

// Append durably appends the next event before publishing it to readers.
func (s *FileEventStore) Append(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrEventLogClosed
	}
	if s.poisoned != nil {
		return &EventStoreError{Kind: ErrEventLogRollback, Cause: s.poisoned}
	}
	if len(s.events) >= s.capacity {
		return ErrEventStoreFull
	}

	event = cloneEvent(event)
	if err := s.validateEvent(event, uint64(len(s.events))+1, 0); err != nil {
		return err
	}
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return &EventStoreError{Kind: ErrEventLogWrite, Cause: fmt.Errorf("encode event: %w", err)}
	}
	line, err := json.Marshal(eventLogRecord{Event: event, Checksum: fmt.Sprintf("%08x", crc32.ChecksumIEEE(eventJSON))})
	if err != nil {
		return &EventStoreError{Kind: ErrEventLogWrite, Cause: fmt.Errorf("encode event record: %w", err)}
	}
	line = append(line, '\n')
	info, err := s.file.Stat()
	if err != nil {
		return &EventStoreError{Kind: ErrEventLogWrite, Cause: fmt.Errorf("stat event log before append: %w", err)}
	}
	offset := info.Size()
	if err := s.writeAll(line); err != nil {
		return s.rollbackAppend(offset, err)
	}
	if err := s.syncFile(); err != nil {
		return s.rollbackAppend(offset, fmt.Errorf("sync event append: %w", err))
	}
	s.events = append(s.events, event)
	return nil
}

// Read returns cloned events whose sequence is greater than after.
func (s *FileEventStore) Read(ctx context.Context, after uint64, limit int) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, errors.New("event read limit must be positive")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrEventLogClosed
	}
	result := make([]Event, 0, limit)
	for _, event := range s.events {
		if event.Sequence <= after {
			continue
		}
		result = append(result, cloneEvent(event))
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

// Close closes the underlying event log file. Repeated calls are harmless.
func (s *FileEventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.file.Close()
}

func (s *FileEventStore) writeInitialHeader() error {
	line, err := json.Marshal(eventLogHeader{Format: eventLogFormat, OrganismID: s.organismID})
	if err != nil {
		return &EventStoreError{Kind: ErrEventLogWrite, Cause: fmt.Errorf("encode event log header: %w", err)}
	}
	line = append(line, '\n')
	if err := s.writeAll(line); err != nil {
		return s.rollbackAppend(0, err)
	}
	if err := s.syncFile(); err != nil {
		return s.rollbackAppend(0, fmt.Errorf("sync event log header: %w", err))
	}
	return nil
}

func (s *FileEventStore) writeAll(contents []byte) error {
	for len(contents) > 0 {
		written, err := s.writeFile(contents)
		if written < 0 || written > len(contents) {
			return fmt.Errorf("invalid write count %d", written)
		}
		contents = contents[written:]
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func (s *FileEventStore) rollbackAppend(offset int64, cause error) error {
	truncateErr := s.file.Truncate(offset)
	syncErr := s.syncFile()
	if truncateErr != nil || syncErr != nil {
		rollbackCause := fmt.Errorf("append cause: %v; truncate cause: %v; sync cause: %v", cause, truncateErr, syncErr)
		s.poisoned = rollbackCause
		return &EventStoreError{Kind: ErrEventLogRollback, Cause: rollbackCause}
	}
	return &EventStoreError{Kind: ErrEventLogWrite, Cause: cause}
}

func (s *FileEventStore) recover(contents []byte) (int64, bool, error) {
	headerEnd := bytes.IndexByte(contents, '\n')
	if headerEnd < 0 {
		return 0, false, &EventStoreError{Kind: ErrEventLogCorrupt, Line: 1, Cause: errors.New("missing complete event log header")}
	}
	var header eventLogHeader
	if err := json.Unmarshal(contents[:headerEnd], &header); err != nil {
		return 0, false, &EventStoreError{Kind: ErrEventLogCorrupt, Line: 1, Cause: err}
	}
	if header.Format != eventLogFormat {
		return 0, false, &EventStoreError{Kind: ErrEventLogVersionMismatch, Line: 1, Cause: fmt.Errorf("format %q", header.Format)}
	}
	if header.OrganismID != s.organismID {
		return 0, false, &EventStoreError{Kind: ErrEventLogOrganismMismatch, Line: 1, Cause: fmt.Errorf("header organism %q, requested %q", header.OrganismID, s.organismID)}
	}

	lineNumber := 1
	offset := headerEnd + 1
	for offset < len(contents) {
		relativeEnd := bytes.IndexByte(contents[offset:], '\n')
		if relativeEnd < 0 {
			return int64(offset), true, nil
		}
		lineNumber++
		line := contents[offset : offset+relativeEnd]
		if len(line) == 0 {
			return 0, false, &EventStoreError{Kind: ErrEventLogCorrupt, Line: lineNumber, Cause: errors.New("empty event record")}
		}
		var record eventLogRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return 0, false, &EventStoreError{Kind: ErrEventLogCorrupt, Line: lineNumber, Cause: err}
		}
		eventJSON, err := json.Marshal(record.Event)
		if err != nil {
			return 0, false, &EventStoreError{Kind: ErrEventLogCorrupt, Line: lineNumber, Cause: err}
		}
		checksum := fmt.Sprintf("%08x", crc32.ChecksumIEEE(eventJSON))
		if record.Checksum != checksum {
			return 0, false, &EventStoreError{Kind: ErrEventLogCorrupt, Line: lineNumber, Cause: fmt.Errorf("checksum mismatch: got %q, want %q", record.Checksum, checksum)}
		}
		if err := s.validateEvent(record.Event, uint64(len(s.events))+1, lineNumber); err != nil {
			return 0, false, err
		}
		s.events = append(s.events, cloneEvent(record.Event))
		offset += relativeEnd + 1
	}
	return int64(len(contents)), false, nil
}

func (s *FileEventStore) validateEvent(event Event, expected uint64, line int) error {
	if event.SchemaVersion != SchemaVersion {
		return &EventStoreError{Kind: ErrEventLogVersionMismatch, Line: line, Cause: fmt.Errorf("event schema %q", event.SchemaVersion)}
	}
	if event.OrganismID != s.organismID {
		return &EventStoreError{Kind: ErrEventLogOrganismMismatch, Line: line, Cause: fmt.Errorf("event organism %q, requested %q", event.OrganismID, s.organismID)}
	}
	if event.EventID == "" || event.Kind == "" || len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return &EventStoreError{Kind: ErrEventLogCorrupt, Line: line, Cause: errors.New("event is missing its ID, kind, or valid JSON payload")}
	}
	if event.Sequence != expected {
		kind := ErrNonMonotonicEventSequence
		if event.Sequence != 0 && event.Sequence < expected {
			kind = ErrDuplicateEventSequence
		}
		return &EventStoreError{Kind: kind, Line: line, Sequence: event.Sequence, Expected: expected}
	}
	return nil
}
