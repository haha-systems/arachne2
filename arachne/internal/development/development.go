// Package development applies governance-approved, provenance-bearing changes.
package development

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
)

const (
	replayBatchSize = 256
	schemaVersion   = "arachne.development.v1"
)

type routeChangePayload struct {
	OrganismID string  `json:"organism_id"`
	Revision   uint64  `json:"revision"`
	Key        string  `json:"key"`
	Previous   float64 `json:"previous"`
	Value      float64 `json:"value"`
}

// RoutingRequest proposes a bounded routing-weight change backed by event evidence.
type RoutingRequest struct {
	ID               string
	InteractionID    string
	ProposerID       string
	Key              string
	Value            float64
	CreatedAt        time.Time
	SourceEventIDs   []string
	EvidenceEventIDs []string
}

// Change records an applied value, the governance result, and its evidence lineage.
type Change struct {
	ID                 string    `json:"id"`
	OrganismID         string    `json:"organism_id"`
	ProposalID         string    `json:"proposal_id"`
	Revision           uint64    `json:"revision"`
	Key                string    `json:"key"`
	Previous           float64   `json:"previous"`
	Value              float64   `json:"value"`
	GovernanceDecision string    `json:"governance_decision_id"`
	GovernanceEventID  string    `json:"governance_event_id"`
	SourceEventIDs     []string  `json:"source_event_ids,omitempty"`
	EvidenceEventIDs   []string  `json:"evidence_event_ids,omitempty"`
	AppliedAt          time.Time `json:"applied_at"`
	EventID            string    `json:"event_id,omitempty"`
}

// Snapshot is one organism instance's current route weights and change history.
type Snapshot struct {
	OrganismID string             `json:"organism_id"`
	Revision   uint64             `json:"revision"`
	Routing    map[string]float64 `json:"routing"`
	Changes    []Change           `json:"changes"`
}

// Engine serializes policy-approved routing changes for one organism instance.
type Engine struct {
	mu         sync.Mutex
	organismID string
	routing    map[string]float64
	revision   uint64
	changes    []Change
	governor   *governance.Service
	events     *cognition.Spine
	now        func() time.Time
}

// NewEngine starts from an explicit initial profile and recovers changes from its event history.
func NewEngine(ctx context.Context, organismID string, initial map[string]float64, governor *governance.Service, events *cognition.Spine) (*Engine, error) {
	if strings.TrimSpace(organismID) == "" || governor == nil || events == nil {
		return nil, errors.New("organism ID, governance service, and cognitive event spine are required")
	}
	engine := &Engine{
		organismID: organismID, routing: cloneRouting(initial), changes: make([]Change, 0),
		governor: governor, events: events, now: time.Now,
	}
	if err := validateRouting(engine.routing); err != nil {
		return nil, err
	}
	if err := engine.restore(ctx); err != nil {
		return nil, err
	}
	return engine, nil
}

// PrepareRoutingProposal binds the current value and revision into governance input.
func (e *Engine) PrepareRoutingProposal(request RoutingRequest) (governance.Proposal, error) {
	if err := validateRequest(request); err != nil {
		return governance.Proposal{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	previous := e.routing[request.Key]
	if request.Value == previous {
		return governance.Proposal{}, errors.New("routing request does not change the current value")
	}
	payload, err := json.Marshal(routeChangePayload{
		OrganismID: e.organismID, Revision: e.revision + 1,
		Key: request.Key, Previous: previous, Value: request.Value,
	})
	if err != nil {
		return governance.Proposal{}, fmt.Errorf("encode routing change: %w", err)
	}
	return governance.Proposal{
		ID: request.ID, InteractionID: request.InteractionID, ProposerID: request.ProposerID,
		Class: governance.ActionHighImpact, Target: routeTarget(e.organismID, request.Key),
		Action: "change routing weight", Payload: payload, CreatedAt: request.CreatedAt.UTC(),
		SourceEventIDs: sortedIDs(request.SourceEventIDs), EvidenceEventIDs: sortedIDs(request.EvidenceEventIDs),
	}, nil
}

// ApplyRoutingChange evaluates approval, records the change, and updates this instance's profile.
func (e *Engine) ApplyRoutingChange(ctx context.Context, proposal governance.Proposal, approvals []governance.Approval) (Change, governance.Decision, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	changeInput, err := validatePreparedChange(e.organismID, e.revision, e.routing, proposal)
	if err != nil {
		return Change{}, governance.Decision{}, err
	}
	decision, err := e.governor.Evaluate(ctx, proposal, approvals)
	if err != nil {
		return Change{}, governance.Decision{}, err
	}
	if decision.Outcome != governance.OutcomeApproved {
		return Change{}, decision, nil
	}
	change := Change{
		ID:         stableChangeID(e.organismID, proposal.ID, changeInput.Revision),
		OrganismID: e.organismID, ProposalID: proposal.ID, Revision: changeInput.Revision,
		Key: changeInput.Key, Previous: changeInput.Previous, Value: changeInput.Value,
		GovernanceDecision: decision.ID, GovernanceEventID: decision.EventID,
		SourceEventIDs: sortedIDs(proposal.SourceEventIDs), EvidenceEventIDs: sortedIDs(proposal.EvidenceEventIDs),
		AppliedAt: e.now().UTC(),
	}
	payload, err := json.Marshal(map[string]any{
		"schema_version": schemaVersion, "operation": "routing_change_applied", "change": change,
	})
	if err != nil {
		return Change{}, decision, fmt.Errorf("encode development event: %w", err)
	}
	parents := sortedIDs(append([]string{decision.EventID}, append(change.SourceEventIDs, change.EvidenceEventIDs...)...))
	event, err := e.events.Emit(ctx, cognition.Draft{
		AgentID: proposal.ProposerID, CorrelationID: proposal.InteractionID,
		ParentEventIDs: parents, Kind: cognition.KindDevelopment, Payload: payload,
	})
	if err != nil {
		return Change{}, decision, fmt.Errorf("governance approved change but development event failed; profile unchanged: %w", err)
	}
	change.EventID = event.EventID
	e.routing[change.Key] = change.Value
	e.revision = change.Revision
	e.changes = append(e.changes, cloneChange(change))
	return cloneChange(change), decision, nil
}

// Snapshot returns a defensive copy of the current profile and its causal history.
func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	snapshot := Snapshot{
		OrganismID: e.organismID, Revision: e.revision,
		Routing: cloneRouting(e.routing), Changes: make([]Change, len(e.changes)),
	}
	for i, change := range e.changes {
		snapshot.Changes[i] = cloneChange(change)
	}
	return snapshot
}

func (e *Engine) restore(ctx context.Context) error {
	var after uint64
	for {
		events, err := e.events.Read(ctx, after, replayBatchSize)
		if err != nil {
			return fmt.Errorf("read development history: %w", err)
		}
		for _, event := range events {
			after = event.Sequence
			if err := e.restoreEvent(event); err != nil {
				return err
			}
		}
		if len(events) < replayBatchSize {
			return nil
		}
	}
}

func (e *Engine) restoreEvent(event cognition.Event) error {
	if event.Kind != cognition.KindDevelopment {
		return nil
	}
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		Operation     string `json:"operation"`
		Change        Change `json:"change"`
	}
	if err := json.Unmarshal(event.Payload, &envelope); err != nil {
		return fmt.Errorf("decode development event %q: %w", event.EventID, err)
	}
	if envelope.Operation != "routing_change_applied" || envelope.Change.OrganismID != e.organismID {
		return nil
	}
	if envelope.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported development schema %q", envelope.SchemaVersion)
	}
	if err := e.restoreChange(envelope.Change, event.EventID); err != nil {
		return fmt.Errorf("restore development event %q: %w", event.EventID, err)
	}
	return nil
}

func (e *Engine) restoreChange(change Change, eventID string) error {
	if strings.TrimSpace(change.Key) == "" || !validWeight(change.Previous) || !validWeight(change.Value) ||
		change.Revision != e.revision+1 || e.routing[change.Key] != change.Previous {
		return errors.New("change history does not match the initial routing profile")
	}
	change.EventID = eventID
	e.routing[change.Key] = change.Value
	e.revision = change.Revision
	e.changes = append(e.changes, cloneChange(change))
	return nil
}

func validatePreparedChange(organismID string, revision uint64, routing map[string]float64, proposal governance.Proposal) (routeChangePayload, error) {
	var payload routeChangePayload
	if err := json.Unmarshal(proposal.Payload, &payload); err != nil {
		return routeChangePayload{}, errors.New("proposal does not contain a valid routing change")
	}
	if err := validateRoutingIdentity(proposal.ID, proposal.InteractionID, proposal.ProposerID, payload.Key, proposal.CreatedAt); err != nil {
		return routeChangePayload{}, err
	}
	if proposal.Class != governance.ActionHighImpact || proposal.Action != "change routing weight" {
		return routeChangePayload{}, errors.New("proposal is not a complete routing governance request")
	}
	if payload.OrganismID != organismID || payload.Revision != revision+1 || routing[payload.Key] != payload.Previous ||
		proposal.Target != routeTarget(organismID, payload.Key) || !validWeight(payload.Value) || payload.Value == payload.Previous {
		return routeChangePayload{}, errors.New("routing proposal is stale or targets another profile")
	}
	return payload, nil
}

func validateRequest(request RoutingRequest) error {
	if err := validateRoutingIdentity(request.ID, request.InteractionID, request.ProposerID, request.Key, request.CreatedAt); err != nil {
		return err
	}
	if !validWeight(request.Value) {
		return errors.New("routing weight must be finite and between -1 and 1")
	}
	return nil
}

func validateRoutingIdentity(id, interactionID, proposerID, key string, createdAt time.Time) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(interactionID) == "" || strings.TrimSpace(proposerID) == "" {
		return errors.New("routing change requires proposal, interaction, and proposer IDs")
	}
	if strings.TrimSpace(key) == "" || createdAt.IsZero() {
		return errors.New("routing change requires a key and creation time")
	}
	return nil
}

func validateRouting(routing map[string]float64) error {
	for key, value := range routing {
		if strings.TrimSpace(key) == "" || !validWeight(value) {
			return errors.New("initial routing profile contains an invalid key or weight")
		}
	}
	return nil
}

func validWeight(value float64) bool {
	return math.Abs(value) <= 1
}

func routeTarget(organismID, key string) string { return "organism:" + organismID + ":routing:" + key }

func stableChangeID(organismID, proposalID string, revision uint64) string {
	return fmt.Sprintf("%s:%s:%d", organismID, proposalID, revision)
}

func cloneRouting(routing map[string]float64) map[string]float64 {
	clone := make(map[string]float64, len(routing))
	for key, value := range routing {
		clone[key] = value
	}
	return clone
}

func cloneChange(change Change) Change {
	change.SourceEventIDs = append([]string(nil), change.SourceEventIDs...)
	change.EvidenceEventIDs = append([]string(nil), change.EvidenceEventIDs...)
	return change
}

func sortedIDs(ids []string) []string {
	result := append([]string(nil), ids...)
	sort.Strings(result)
	unique := result[:0]
	for _, id := range result {
		if id != "" && (len(unique) == 0 || unique[len(unique)-1] != id) {
			unique = append(unique, id)
		}
	}
	return unique
}
