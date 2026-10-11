package cognition

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestFileEventStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	spine, err := NewSpine("org-a", store)
	if err != nil {
		t.Fatal(err)
	}
	want := make([]Event, 0, 2)
	for _, kind := range []string{KindPerception, KindAction} {
		event, err := spine.Emit(ctx, Draft{Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, event)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("close reopened store: %v", err)
		}
	}()
	recovered, err := reopened.Read(ctx, 0, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !equalEvents(recovered, want) {
		t.Fatalf("recovered events = %#v, want %#v", recovered, want)
	}
}

func TestFileEventStoreSequenceContinuationAndRejection(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 1)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 2)); err != nil {
		t.Fatalf("append recovered next sequence: %v", err)
	}
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 2)); !errors.Is(err, ErrDuplicateEventSequence) {
		t.Fatalf("duplicate sequence error = %v", err)
	}
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 4)); !errors.Is(err, ErrNonMonotonicEventSequence) {
		t.Fatalf("non-monotonic sequence error = %v", err)
	}
}

func TestFileEventStoreTornTailRepair(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 1)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	tail := []byte(`{"event":`)
	if err := os.WriteFile(path, append(append([]byte(nil), valid...), tail...), 0o600); err != nil { // #nosec G703 -- path is created in this test's private temporary directory.
		t.Fatal(err)
	}

	store, err = OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	info := store.RepairInfo()
	if !info.Repaired || info.TruncatedBytes != int64(len(tail)) || info.TruncatedFrom != int64(len(valid)) {
		t.Fatalf("repair info = %+v, want truncation of %d bytes at %d", info, len(tail), len(valid))
	}
	after, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, valid) {
		t.Fatalf("repaired file differs from valid prefix: got %q, want %q", after, valid)
	}
	events, err := store.Read(ctx, 0, 8)
	if err != nil || len(events) != 1 || events[0].Sequence != 1 {
		t.Fatalf("recovered events = %+v, err=%v", events, err)
	}
}

func TestFileEventStoreMidFileCorruptionDoesNotModifyFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	for sequence := uint64(1); sequence <= 2; sequence++ {
		if err := store.Append(ctx, fileStoreTestEvent("org-a", sequence)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), contents...)
	checksum := bytes.Index(corrupt, []byte(`"checksum":"`))
	if checksum < 0 {
		t.Fatal("event checksum not found")
	}
	checksum += len(`"checksum":"`)
	if corrupt[checksum] == '0' {
		corrupt[checksum] = '1'
	} else {
		corrupt[checksum] = '0'
	}
	if err := os.WriteFile(path, corrupt, 0o600); err != nil { // #nosec G703 -- path is created in this test's private temporary directory.
		t.Fatal(err)
	}
	corrupt, err = os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}

	opened, err := OpenFileEventStore(path, "org-a", 8)
	if !errors.Is(err, ErrEventLogCorrupt) {
		if opened != nil {
			_ = opened.Close()
		}
		t.Fatalf("open corrupted file error = %v", err)
	}
	after, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, corrupt) {
		t.Fatal("opening a mid-file corruption changed file bytes")
	}
}

func TestFileEventStoreCapacity(t *testing.T) {
	ctx := context.Background()
	store, err := OpenFileEventStore(filepath.Join(t.TempDir(), "events.jsonl"), "org-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 1)); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 2)); !errors.Is(err, ErrEventStoreFull) {
		t.Fatalf("append at capacity error = %v", err)
	}
	events, err := store.Read(ctx, 0, 8)
	if err != nil || len(events) != 1 || events[0].Sequence != 1 {
		t.Fatalf("retained events = %+v, err=%v", events, err)
	}
}

func TestFileEventStorePermissions0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertFileMode0600(t, path)

	store, err = OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	assertFileMode0600(t, path)
}

func TestFileEventStoreWriteFailureSafety(t *testing.T) {
	ctx := context.Background()
	store, err := OpenFileEventStore(filepath.Join(t.TempDir(), "events.jsonl"), "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	event := fileStoreTestEvent("org-a", 1)
	originalWrite := store.writeFile
	store.writeFile = func(contents []byte) (int, error) {
		partial := len(contents) / 2
		written, err := originalWrite(contents[:partial])
		if err != nil {
			return written, err
		}
		return written, errors.New("injected partial write failure")
	}
	if err := store.Append(ctx, event); !errors.Is(err, ErrEventLogWrite) {
		t.Fatalf("partial append error = %v", err)
	}
	store.writeFile = originalWrite
	events, err := store.Read(ctx, 0, 8)
	if err != nil || len(events) != 0 {
		t.Fatalf("events after failed append = %+v, err=%v", events, err)
	}
	if err := store.Append(ctx, event); err != nil {
		t.Fatalf("retry sequence after rollback: %v", err)
	}
	events, err = store.Read(ctx, 0, 8)
	if err != nil || len(events) != 1 || events[0].Sequence != 1 {
		t.Fatalf("events after retry = %+v, err=%v", events, err)
	}
}

func TestFileEventStoreHeaderIdentityErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenFileEventStore(path, "org-b", 8); !errors.Is(err, ErrEventLogOrganismMismatch) {
		if opened != nil {
			_ = opened.Close()
		}
		t.Fatalf("organism mismatch error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{\"format\":\"arachne.event_log.v0\",\"organism_id\":\"org-a\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenFileEventStore(path, "org-a", 8); !errors.Is(err, ErrEventLogVersionMismatch) {
		if opened != nil {
			_ = opened.Close()
		}
		t.Fatalf("version mismatch error = %v", err)
	}

}

func fileStoreTestEvent(organismID string, sequence uint64) Event {
	return Event{
		SchemaVersion: SchemaVersion,
		EventID:       fmt.Sprintf("%s:event:%d", organismID, sequence),
		Sequence:      sequence,
		OccurredAt:    time.Unix(1, 0).UTC(),
		OrganismID:    organismID,
		Kind:          KindAction,
		Payload:       []byte(`{"ok":true}`),
	}
}

func equalEvents(left, right []Event) bool {
	return reflect.DeepEqual(left, right)
}

func assertFileMode0600(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file permissions = %04o, want 0600", got)
	}
}

func TestFileEventStoreTornTailDoesNotRepairCompleteInvalidLine(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store, err := OpenFileEventStore(path, "org-a", 8)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(ctx, fileStoreTestEvent("org-a", 1)); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	lastLine := bytes.LastIndexByte(contents[:len(contents)-1], '\n') + 1
	corrupt := append([]byte(nil), contents...)
	corrupt = append(corrupt[:lastLine], []byte("not-json\n")...)
	if err := os.WriteFile(path, corrupt, 0o600); err != nil { // #nosec G703 -- path is created in this test's private temporary directory.
		t.Fatal(err)
	}
	before, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenFileEventStore(path, "org-a", 8)
	if !errors.Is(err, ErrEventLogCorrupt) {
		if opened != nil {
			_ = opened.Close()
		}
		t.Fatalf("open complete invalid final line error = %v", err)
	}
	after, err := os.ReadFile(path) // #nosec G304 -- path is created in this test's private temporary directory.
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("complete invalid final line was modified")
	}
}
