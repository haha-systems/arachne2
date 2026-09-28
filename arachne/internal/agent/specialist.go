package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

const (
	// SpecialistActivateMessage carries one bounded activation request to a specialist.
	SpecialistActivateMessage = "specialist.activate"
	// SpecialistProposalMessage returns a candidate proposal for later coordination.
	SpecialistProposalMessage = "specialist.proposal"
	maxProposalActions        = 32
	maxProposalBytes          = 1 << 20
)

// Activation is the explicit input and evidence context supplied to one specialist.
type Activation struct {
	InteractionID  string          `json:"interaction_id"`
	Input          json.RawMessage `json:"input"`
	SourceEventIDs []string        `json:"source_event_ids,omitempty"`
	SessionID      string          `json:"session_id,omitempty"`
}

// RequestedAction is a proposed intent. It has no execution capability.
type RequestedAction struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// Proposal is an attributed candidate interpretation or action request.
type Proposal struct {
	ID               string            `json:"id"`
	EventID          string            `json:"event_id,omitempty"`
	InteractionID    string            `json:"interaction_id"`
	SpecialistID     string            `json:"specialist_id"`
	CorrelationID    string            `json:"correlation_id"`
	Status           string            `json:"status"`
	Summary          string            `json:"summary,omitempty"`
	Confidence       float64           `json:"confidence,omitempty"`
	EvidenceEventIDs []string          `json:"evidence_event_ids,omitempty"`
	RequestedActions []RequestedAction `json:"requested_actions,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	Error            string            `json:"error,omitempty"`
}

// Specialist produces a proposal from one activation without performing actions.
type Specialist interface {
	Propose(context.Context, Activation) (Proposal, error)
}

// SpecialistFunc adapts a function to the Specialist interface.
type SpecialistFunc func(context.Context, Activation) (Proposal, error)

// Propose implements Specialist.
func (f SpecialistFunc) Propose(ctx context.Context, activation Activation) (Proposal, error) {
	return f(ctx, activation)
}

// SpecialistAgent runs one specialist and returns attributed proposals through the router.
type SpecialistAgent struct {
	id             string
	implementation Specialist
	events         *cognition.Spine
	now            func() time.Time
}

// NewSpecialistAgent binds a specialist implementation to its identity and event spine.
func NewSpecialistAgent(id string, implementation Specialist, events *cognition.Spine) (*SpecialistAgent, error) {
	if strings.TrimSpace(id) == "" || implementation == nil || events == nil {
		return nil, errors.New("specialist ID, implementation, and event spine are required")
	}
	return &SpecialistAgent{id: id, implementation: implementation, events: events, now: time.Now}, nil
}

// Run handles activation messages until cancellation or inbox closure.
func (a *SpecialistAgent) Run(ctx context.Context, inbox <-chan Message, sender Sender) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, open := <-inbox:
			if !open {
				return nil
			}
			if message.Kind != SpecialistActivateMessage {
				continue
			}
			if err := a.handle(ctx, message, sender); err != nil {
				return err
			}
		}
	}
}

func (a *SpecialistAgent) handle(ctx context.Context, message Message, sender Sender) error {
	perception, err := a.events.Emit(ctx, cognition.Draft{
		AgentID: a.id, SessionID: sessionFromActivation(message.Payload), CorrelationID: message.CorrelationID,
		Kind: cognition.KindPerception, Payload: message.Payload,
	})
	if err != nil {
		return fmt.Errorf("record specialist perception: %w", err)
	}
	var activation Activation
	if err := json.Unmarshal(message.Payload, &activation); err != nil {
		return a.sendFailure(ctx, message, sender, perception.EventID, "invalid activation: "+err.Error())
	}
	if err := validateActivation(activation); err != nil {
		return a.recordAndSend(ctx, message, sender, activation, perception.EventID, Proposal{
			Status: "failed", Error: err.Error(),
		})
	}
	activationEvent, err := a.events.Emit(ctx, cognition.Draft{
		AgentID: a.id, SessionID: activation.SessionID, CorrelationID: message.CorrelationID,
		ParentEventIDs: []string{perception.EventID}, Kind: cognition.KindActivation,
		Payload: message.Payload,
	})
	if err != nil {
		return fmt.Errorf("record specialist activation: %w", err)
	}
	proposal, proposeErr := a.implementation.Propose(ctx, activation)
	if proposeErr != nil {
		return a.recordAndSend(ctx, message, sender, activation, activationEvent.EventID, Proposal{
			Status: "failed", Error: proposeErr.Error(),
		})
	}
	if err := validateProposal(proposal); err != nil {
		return a.recordAndSend(ctx, message, sender, activation, activationEvent.EventID, Proposal{
			Status: "failed", Error: err.Error(),
		})
	}
	proposal.Status = "candidate"
	return a.recordAndSend(ctx, message, sender, activation, activationEvent.EventID, proposal)
}

func (a *SpecialistAgent) sendFailure(ctx context.Context, message Message, sender Sender, parentID, detail string) error {
	return a.recordAndSend(ctx, message, sender, Activation{}, parentID, Proposal{
		Status: "failed", Error: detail,
	})
}

func (a *SpecialistAgent) recordAndSend(ctx context.Context, message Message, sender Sender, activation Activation, parentID string, proposal Proposal) error {
	id, err := proposalID()
	if err != nil {
		return err
	}
	proposal.ID = id
	proposal.InteractionID = activation.InteractionID
	proposal.SpecialistID = a.id
	proposal.CorrelationID = message.CorrelationID
	proposal.CreatedAt = a.now().UTC()
	proposal.EvidenceEventIDs = mergeIDs(activation.SourceEventIDs, proposal.EvidenceEventIDs)
	if len(proposal.RequestedActions) > maxProposalActions {
		return fmt.Errorf("specialist %q returned more than %d requested actions", a.id, maxProposalActions)
	}
	payload, err := json.Marshal(proposal)
	if err != nil {
		return fmt.Errorf("encode proposal: %w", err)
	}
	if len(payload) > maxProposalBytes {
		return fmt.Errorf("proposal exceeds %d bytes", maxProposalBytes)
	}
	proposalEvent, err := a.events.Emit(ctx, cognition.Draft{
		AgentID: a.id, SessionID: activation.SessionID, CorrelationID: message.CorrelationID,
		ParentEventIDs: []string{parentID}, Kind: cognition.KindProposal, Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("record specialist proposal: %w", err)
	}
	proposal.EventID = proposalEvent.EventID
	payload, err = json.Marshal(proposal)
	if err != nil {
		return fmt.Errorf("encode attributed proposal: %w", err)
	}
	if err := sender.Send(ctx, message.From, SpecialistProposalMessage, payload); err != nil {
		return fmt.Errorf("send proposal to %q: %w", message.From, err)
	}
	return nil
}

func validateActivation(activation Activation) error {
	if strings.TrimSpace(activation.InteractionID) == "" {
		return errors.New("activation interaction ID must not be empty")
	}
	if len(activation.Input) == 0 || len(activation.Input) > maxProposalBytes || !json.Valid(activation.Input) {
		return errors.New("activation input must be valid JSON no larger than one MiB")
	}
	return nil
}

func validateProposal(proposal Proposal) error {
	if strings.TrimSpace(proposal.Summary) == "" {
		return errors.New("specialist proposal summary must not be empty")
	}
	if proposal.Confidence < 0 || proposal.Confidence > 1 {
		return errors.New("specialist proposal confidence must be between zero and one")
	}
	if len(proposal.RequestedActions) > maxProposalActions {
		return fmt.Errorf("specialist proposal exceeds %d requested actions", maxProposalActions)
	}
	for index, action := range proposal.RequestedActions {
		if strings.TrimSpace(action.Kind) == "" || len(action.Payload) == 0 || len(action.Payload) > maxProposalBytes || !json.Valid(action.Payload) {
			return fmt.Errorf("requested action %d requires a kind and valid JSON payload no larger than one MiB", index)
		}
	}
	return nil
}

func sessionFromActivation(payload json.RawMessage) string {
	var activation Activation
	if json.Unmarshal(payload, &activation) == nil {
		return activation.SessionID
	}
	return ""
}

func proposalID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate proposal ID: %w", err)
	}
	return hex.EncodeToString(random[:]), nil
}

func mergeIDs(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	merged := make([]string, 0, len(left)+len(right))
	for _, id := range append(append([]string(nil), left...), right...) {
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	return merged
}
