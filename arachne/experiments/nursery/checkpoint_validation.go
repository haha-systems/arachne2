package nursery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

func validateCheckpointHistory(checkpoint Checkpoint) error {
	historyBytes, err := json.Marshal(checkpoint.History)
	if err != nil {
		return fmt.Errorf("encode checkpoint history: %w", err)
	}
	historyHash := sha256.Sum256(historyBytes)
	if checkpoint.EventHistoryRef != hex.EncodeToString(historyHash[:]) {
		return errors.New("checkpoint event history reference does not match its history")
	}
	previousDigest := ""
	for index, record := range checkpoint.History {
		if record.Tick != uint64(index+1) {
			return errors.New("checkpoint history has a missing or out-of-order tick")
		}
		wantDigest, err := historyDigest(previousDigest, record)
		if err != nil {
			return fmt.Errorf("validate checkpoint tick %d digest: %w", record.Tick, err)
		}
		if record.HistoryDigest != wantDigest {
			return fmt.Errorf("checkpoint tick %d history digest is inconsistent", record.Tick)
		}
		previousDigest = record.HistoryDigest
	}
	if len(checkpoint.History) > 0 && len(checkpoint.Interventions) == 0 {
		var lastWorld World
		if err := json.Unmarshal(checkpoint.History[len(checkpoint.History)-1].WorldAfter, &lastWorld); err != nil {
			return fmt.Errorf("decode checkpoint final world: %w", err)
		}
		lastWorldJSON, _ := json.Marshal(&lastWorld)
		checkpointWorldJSON, _ := json.Marshal(&checkpoint.World)
		if !bytes.Equal(lastWorldJSON, checkpointWorldJSON) {
			return errors.New("checkpoint world does not match the final event-history state")
		}
	}
	return nil
}
