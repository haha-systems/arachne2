// Package memory stores source-attributed episodic and candidate semantic memories.
package memory

import (
	"encoding/json"
	"time"
)

// SourceRef identifies the external or cognitive record from which a memory came.
type SourceRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Episode is an observed experience with source, time, and organism context.
type Episode struct {
	ID             string          `json:"id"`
	OrganismID     string          `json:"organism_id"`
	Source         SourceRef       `json:"source"`
	SourceEventIDs []string        `json:"source_event_ids,omitempty"`
	OccurredAt     time.Time       `json:"occurred_at"`
	RecordedAt     time.Time       `json:"recorded_at"`
	AgentID        string          `json:"agent_id,omitempty"`
	SessionID      string          `json:"session_id,omitempty"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	Kind           string          `json:"kind"`
	Content        json.RawMessage `json:"content"`
	Tags           []string        `json:"tags,omitempty"`
}

// SemanticStatus tracks epistemic standing without granting promotion authority.
type SemanticStatus string

const (
	// SemanticCandidate is a sourced interpretation awaiting review or governance.
	SemanticCandidate SemanticStatus = "candidate"
	// SemanticRejected is a candidate retained as rejected evidence.
	SemanticRejected SemanticStatus = "rejected"
	// SemanticSuperseded marks a candidate displaced by a later record.
	SemanticSuperseded SemanticStatus = "superseded"
)

// SemanticRecord is a sourced interpretation; this package cannot promote it.
type SemanticRecord struct {
	ID               string          `json:"id"`
	OrganismID       string          `json:"organism_id"`
	Assertion        string          `json:"assertion"`
	Source           SourceRef       `json:"source"`
	SourceEpisodeIDs []string        `json:"source_episode_ids,omitempty"`
	SourceEventIDs   []string        `json:"source_event_ids,omitempty"`
	Contradicts      []string        `json:"contradicts,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	AgentID          string          `json:"agent_id,omitempty"`
	SessionID        string          `json:"session_id,omitempty"`
	Confidence       float64         `json:"confidence"`
	Status           SemanticStatus  `json:"status"`
	Context          json.RawMessage `json:"context,omitempty"`
}

// EpisodeQuery filters explicit memory retrieval without mutating stored records.
type EpisodeQuery struct {
	OrganismID    string
	AgentID       string
	SessionID     string
	CorrelationID string
	SourceKind    string
	SourceID      string
	Tags          []string
	Terms         []string
	From          time.Time
	To            time.Time
	Limit         int
}

// SemanticQuery filters candidate interpretations by attribution and evidence.
type SemanticQuery struct {
	OrganismID string
	AgentID    string
	SessionID  string
	Status     SemanticStatus
	SourceID   string
	Terms      []string
	Limit      int
}
