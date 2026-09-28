package memory

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStore persists one organism's memory as an atomically replaced JSON snapshot.
type FileStore struct {
	*InMemory
	path string
}

// OpenFileStore opens or creates the private JSON snapshot at path.
func OpenFileStore(path, organismID string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("memory file path must not be empty")
	}
	if strings.TrimSpace(organismID) == "" {
		return nil, errors.New("organism ID must not be empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve memory file path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create memory directory: %w", err)
	}
	db, err := loadDatabase(path, organismID)
	if err != nil {
		return nil, err
	}
	store := newMemoryStore(db)
	store.persist = func(candidate database) (bool, error) {
		return writeDatabase(path, candidate)
	}
	return &FileStore{InMemory: store, path: path}, nil
}

// Path returns the absolute path used for the persisted memory snapshot.
func (s *FileStore) Path() string { return s.path }

func loadDatabase(path, organismID string) (database, error) {
	// #nosec G304 -- the embedding application explicitly configures the memory file path.
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		store, createErr := NewMemoryStore(organismID)
		if createErr != nil {
			return database{}, createErr
		}
		return store.database, nil
	}
	if err != nil {
		return database{}, fmt.Errorf("open memory file: %w", err)
	}
	db, decodeErr := decodeDatabase(file)
	closeErr := file.Close()
	if decodeErr != nil {
		return database{}, decodeErr
	}
	if closeErr != nil {
		return database{}, fmt.Errorf("close memory file: %w", closeErr)
	}
	if db.Runs == nil {
		db.Runs = make(map[string]ConsolidationRun)
	}
	if db.Patterns == nil {
		db.Patterns = make(map[string]ConsolidatedPattern)
	}
	if err := validateDatabase(db, organismID); err != nil {
		return database{}, err
	}
	return db, nil
}

func decodeDatabase(file io.Reader) (database, error) {
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var db database
	if err := decoder.Decode(&db); err != nil {
		return database{}, fmt.Errorf("decode memory file: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return database{}, errors.New("memory file contains multiple JSON values")
		}
		return database{}, fmt.Errorf("read memory file trailer: %w", err)
	}
	return db, nil
}

func validateDatabase(db database, organismID string) error {
	if db.SchemaVersion != "arachne.memory.v1" {
		return fmt.Errorf("unsupported memory schema %q", db.SchemaVersion)
	}
	if db.OrganismID != organismID {
		return fmt.Errorf("memory file belongs to organism %q, not %q", db.OrganismID, organismID)
	}
	if db.Episodes == nil || db.Semantics == nil {
		return errors.New("memory file is missing record maps")
	}
	for id, episode := range db.Episodes {
		if id != episode.ID {
			return fmt.Errorf("persisted episode key %q does not match record ID %q", id, episode.ID)
		}
		if err := validateEpisode(organismID, episode); err != nil {
			return fmt.Errorf("invalid persisted episode: %w", err)
		}
	}
	for id, record := range db.Semantics {
		if id != record.ID {
			return fmt.Errorf("persisted semantic key %q does not match record ID %q", id, record.ID)
		}
		if err := validateSemantic(organismID, record); err != nil {
			return fmt.Errorf("invalid persisted semantic record: %w", err)
		}
	}
	for id, run := range db.Runs {
		if id != run.ID {
			return fmt.Errorf("persisted consolidation key %q does not match run ID %q", id, run.ID)
		}
		for _, episodeID := range run.SourceEpisodeIDs {
			if _, exists := db.Episodes[episodeID]; !exists {
				return fmt.Errorf("consolidation run %q references missing episode %q", id, episodeID)
			}
		}
		patterns := make([]ConsolidatedPattern, 0, len(run.PatternIDs))
		for _, patternID := range run.PatternIDs {
			pattern, exists := db.Patterns[patternID]
			if !exists {
				return fmt.Errorf("consolidation run %q references missing pattern %q", id, patternID)
			}
			patterns = append(patterns, pattern)
		}
		if err := validateConsolidation(organismID, run, patterns); err != nil {
			return fmt.Errorf("invalid persisted consolidation %q: %w", id, err)
		}
	}
	for id, pattern := range db.Patterns {
		if id != pattern.ID {
			return fmt.Errorf("persisted pattern key %q does not match pattern ID %q", id, pattern.ID)
		}
		if _, exists := db.Runs[pattern.RunID]; !exists {
			return fmt.Errorf("persisted pattern %q has missing run %q", id, pattern.RunID)
		}
		for _, episodeID := range pattern.SourceEpisodeIDs {
			if _, exists := db.Episodes[episodeID]; !exists {
				return fmt.Errorf("consolidated pattern %q references missing episode %q", id, episodeID)
			}
		}
	}
	return nil
}

func writeDatabase(path string, db database) (bool, error) {
	contents, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode memory snapshot: %w", err)
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".arachne-memory-*.tmp")
	if err != nil {
		return false, fmt.Errorf("create temporary memory snapshot: %w", err)
	}
	temporaryPath := temporary.Name()
	renamed := false
	defer func() {
		_ = temporary.Close()
		if !renamed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return false, fmt.Errorf("secure temporary memory snapshot: %w", err)
	}
	if _, err := io.Copy(temporary, bytes.NewReader(contents)); err != nil {
		return false, fmt.Errorf("write memory snapshot: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return false, fmt.Errorf("sync memory snapshot: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return false, fmt.Errorf("close memory snapshot: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return false, fmt.Errorf("replace memory snapshot: %w", err)
	}
	renamed = true
	// #nosec G304 -- directory comes from the application's configured memory path.
	directoryFile, err := os.Open(directory)
	if err != nil {
		return true, fmt.Errorf("open memory directory for sync: %w", err)
	}
	defer func() { _ = directoryFile.Close() }()
	if err := directoryFile.Sync(); err != nil {
		return true, fmt.Errorf("sync memory directory: %w", err)
	}
	return true, nil
}
