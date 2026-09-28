package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/haha-systems/arachne2/internal/cognition"
)

// Activate runs one specialist synchronously and records its attributed activation
// and proposal. The parent events preserve the perception lineage.
func (a *SpecialistAgent) Activate(ctx context.Context, activation Activation, parentEventIDs []string) (Proposal, cognition.Event, error) {
	if err := validateActivation(activation); err != nil {
		return Proposal{}, cognition.Event{}, fmt.Errorf("validate specialist activation: %w", err)
	}
	activationPayload, err := json.Marshal(activation)
	if err != nil {
		return Proposal{}, cognition.Event{}, fmt.Errorf("encode specialist activation: %w", err)
	}
	activationEvent, err := a.events.Emit(ctx, cognition.Draft{
		AgentID: a.id, SessionID: activation.SessionID, CorrelationID: activation.InteractionID,
		ParentEventIDs: append([]string(nil), parentEventIDs...), Kind: cognition.KindActivation,
		Payload: activationPayload,
	})
	if err != nil {
		return Proposal{}, cognition.Event{}, fmt.Errorf("record specialist activation: %w", err)
	}

	proposal, proposeErr := a.implementation.Propose(ctx, activation)
	if proposeErr != nil {
		proposal = Proposal{Status: "failed", Error: proposeErr.Error()}
	} else if validationErr := validateProposal(proposal); validationErr != nil {
		proposal = Proposal{Status: "failed", Error: validationErr.Error()}
	} else {
		proposal.Status = "candidate"
	}
	proposal.ID, err = proposalID()
	if err != nil {
		return Proposal{}, activationEvent, err
	}
	proposal.InteractionID = activation.InteractionID
	proposal.SpecialistID = a.id
	proposal.CorrelationID = activation.InteractionID
	proposal.EvidenceEventIDs = mergeIDs(activation.SourceEventIDs, proposal.EvidenceEventIDs)
	proposal.CreatedAt = a.now().UTC()
	payload, err := json.Marshal(proposal)
	if err != nil {
		return Proposal{}, activationEvent, fmt.Errorf("encode specialist proposal: %w", err)
	}
	if len(payload) > maxProposalBytes {
		return Proposal{}, activationEvent, fmt.Errorf("proposal exceeds %d bytes", maxProposalBytes)
	}
	proposalEvent, err := a.events.Emit(ctx, cognition.Draft{
		AgentID: a.id, SessionID: activation.SessionID, CorrelationID: activation.InteractionID,
		ParentEventIDs: []string{activationEvent.EventID}, Kind: cognition.KindProposal, Payload: payload,
	})
	if err != nil {
		return Proposal{}, activationEvent, fmt.Errorf("record specialist proposal: %w", err)
	}
	proposal.EventID = proposalEvent.EventID
	return proposal, activationEvent, nil
}

// ValidateRequestedAction reports whether an action satisfies the bounded wire contract.
func ValidateRequestedAction(action RequestedAction) error {
	return validateProposal(Proposal{
		Summary: "requested action", Confidence: 0,
		RequestedActions: []RequestedAction{action},
	})
}
