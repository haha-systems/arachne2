package nursery

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// Summary contains descriptive measurements only; it has no aggregate capability or score.
type Summary struct {
	Ticks                   int                       `json:"ticks"`
	ActionCounts            map[string]int            `json:"action_counts"`
	ActionEntropy           float64                   `json:"action_entropy_bits"`
	ActionMotifs            map[string]int            `json:"action_motifs"`
	LocationVisits          map[string]int            `json:"location_visits"`
	EntityInteractions      map[string]int            `json:"entity_interactions"`
	Viability               []Viability               `json:"viability_history"`
	MemoryReplayAction      []TemporalRelation        `json:"memory_replay_action_relations,omitempty"`
	AgentActionCooccurrence map[string]map[string]int `json:"agent_action_cooccurrence,omitempty"`
}

// TemporalRelation records event IDs and relative ticks without inferring causal influence.
type TemporalRelation struct {
	SourceEventID string `json:"source_event_id"`
	SourceKind    string `json:"source_kind"`
	Action        string `json:"action"`
	Tick          uint64 `json:"tick"`
	Lag           uint64 `json:"lag"`
}

// Analyze produces offline action, location, viability, and event-timing measurements.
func Analyze(history []TickRecord) Summary {
	summary := Summary{
		Ticks: len(history), ActionCounts: make(map[string]int),
		ActionMotifs: make(map[string]int), LocationVisits: make(map[string]int),
		EntityInteractions: make(map[string]int), AgentActionCooccurrence: make(map[string]map[string]int),
	}
	previousActions := make([]string, 0, 3)
	seenSignals := make([]eventObservation, 0)
	for _, record := range history {
		kind := record.Action.Kind
		summary.ActionCounts[kind]++
		previousActions = append(previousActions, kind)
		if len(previousActions) > 3 {
			previousActions = previousActions[1:]
		}
		for length := 2; length <= len(previousActions); length++ {
			motif := strings.Join(previousActions[len(previousActions)-length:], ">")
			summary.ActionMotifs[motif]++
		}
		location := pointKey(record.Perception.Position)
		summary.LocationVisits[location]++
		for _, change := range record.Changes {
			if change.Kind == "entities.interacted" || change.Kind == "entity.changed" {
				summary.EntityInteractions[change.EntityID]++
			}
		}
		summary.Viability = append(summary.Viability, record.ViabilityAfter)
		for _, raw := range record.Cognition.Events {
			var event struct {
				EventID string `json:"event_id"`
				Kind    string `json:"kind"`
				AgentID string `json:"agent_id"`
			}
			if json.Unmarshal(raw, &event) != nil {
				continue
			}
			if event.AgentID != "" {
				if summary.AgentActionCooccurrence[event.AgentID] == nil {
					summary.AgentActionCooccurrence[event.AgentID] = make(map[string]int)
				}
				summary.AgentActionCooccurrence[event.AgentID][kind]++
			}
			if event.EventID != "" && (event.Kind == "memory" || event.Kind == "replay") {
				seenSignals = append(seenSignals, eventObservation{ID: event.EventID, Kind: event.Kind, Tick: record.Tick})
			}
		}
		for _, signal := range seenSignals {
			if signal.Tick > record.Tick {
				continue
			}
			if record.Tick-signal.Tick > 16 {
				continue
			}
			summary.MemoryReplayAction = append(summary.MemoryReplayAction, TemporalRelation{
				SourceEventID: signal.ID, SourceKind: signal.Kind, Action: kind,
				Tick: record.Tick, Lag: record.Tick - signal.Tick,
			})
		}
		if len(seenSignals) > 0 {
			retained := seenSignals[:0]
			for _, signal := range seenSignals {
				if record.Tick-signal.Tick <= 16 {
					retained = append(retained, signal)
				}
			}
			seenSignals = retained
		}
	}
	summary.ActionEntropy = actionEntropy(summary.ActionCounts, len(history))
	return summary
}

// WriteJSONL appends one reconstructable tick record per line.
func WriteJSONL(writer io.Writer, records []TickRecord) error {
	encoder := json.NewEncoder(writer)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("encode nursery tick %d: %w", record.Tick, err)
		}
	}
	return nil
}

// ReadJSONL reads a tick history while enforcing a per-record size bound.
func ReadJSONL(reader io.Reader) ([]TickRecord, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	var records []TickRecord
	line := 0
	for scanner.Scan() {
		line++
		var record TickRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode nursery JSONL line %d: %w", line, err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

type eventObservation struct {
	ID   string
	Kind string
	Tick uint64
}

func actionEntropy(counts map[string]int, total int) float64 {
	if total == 0 {
		return 0
	}
	entropy := 0.0
	for _, count := range counts {
		probability := float64(count) / float64(total)
		entropy -= probability * math.Log2(probability)
	}
	return entropy
}

func pointKey(point Point) string { return fmt.Sprintf("%d,%d", point.X, point.Y) }

// SortedCounts provides stable tabular output for CLI summaries.
func SortedCounts(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
