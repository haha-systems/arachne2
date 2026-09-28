// Package workspace collects and selects bounded specialist proposals.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
)

// BroadcastMessage delivers the admitted proposal view to an explicit recipient.
const BroadcastMessage = "workspace.broadcast"

var (
	// ErrFull reports a workspace run or per-run proposal capacity limit.
	ErrFull = errors.New("workspace run capacity exceeded")
	// ErrRunNotFound reports a workspace ID that is not open.
	ErrRunNotFound = errors.New("workspace run not found")
	// ErrAlreadySelected reports an attempt to change a finalized selection.
	ErrAlreadySelected = errors.New("workspace run has already been selected")
)

// Config caps retained runs, proposals, selected entries, and broadcast recipients.
type Config struct {
	MaxRuns           int
	MaxProposals      int
	Capacity          int
	MaxRecipients     int
	MaxBroadcastBytes int
}

// DefaultConfig returns bounded settings for a local organism workspace.
func DefaultConfig() Config {
	return Config{MaxRuns: 128, MaxProposals: 16, Capacity: 4, MaxRecipients: 16, MaxBroadcastBytes: 4 << 20}
}

// Request starts one interaction's proposal collection run.
type Request struct {
	ID             string   `json:"id"`
	InteractionID  string   `json:"interaction_id"`
	SessionID      string   `json:"session_id,omitempty"`
	SourceEventIDs []string `json:"source_event_ids,omitempty"`
}

// SelectionEntry explains why one proposal was admitted or rejected.
type SelectionEntry struct {
	ProposalID string  `json:"proposal_id"`
	Specialist string  `json:"specialist_id"`
	Confidence float64 `json:"confidence"`
	Selected   bool    `json:"selected"`
	Reason     string  `json:"reason"`
}

// Selection records the bounded competition result and its event identity.
type Selection struct {
	EventID     string           `json:"event_id"`
	Capacity    int              `json:"capacity"`
	Policy      string           `json:"policy"`
	Entries     []SelectionEntry `json:"entries"`
	SelectedIDs []string         `json:"selected_ids"`
}

// Delivery captures the result of broadcasting a selection to one agent.
type Delivery struct {
	Recipient string `json:"recipient"`
	Sent      bool   `json:"sent"`
	Error     string `json:"error,omitempty"`
	EventID   string `json:"event_id,omitempty"`
}

// Report is the inspectable state of one interaction workspace.
type Report struct {
	Request              Request          `json:"request"`
	Proposals            []agent.Proposal `json:"proposals"`
	Selection            *Selection       `json:"selection,omitempty"`
	BroadcastPlanEventID string           `json:"broadcast_plan_event_id,omitempty"`
	BroadcastReason      string           `json:"broadcast_reason,omitempty"`
	Recipients           []string         `json:"recipients,omitempty"`
	Deliveries           []Delivery       `json:"deliveries,omitempty"`
}

type run struct {
	report       Report
	bySpecialist map[string]struct{}
	broadcasting bool
}

// Workspace is a bounded, inspectable coordinator over candidate proposals.
type Workspace struct {
	mu     sync.RWMutex
	config Config
	events *cognition.Spine
	runs   map[string]*run
}

// New creates a workspace with explicit resource bounds and event attribution.
func New(config Config, events *cognition.Spine) (*Workspace, error) {
	if config.MaxRuns < 1 || config.MaxProposals < 1 || config.Capacity < 1 ||
		config.Capacity > config.MaxProposals || config.MaxRecipients < 1 || config.MaxBroadcastBytes < 1 {
		return nil, errors.New("workspace bounds must be positive and capacity cannot exceed proposal limit")
	}
	if events == nil {
		return nil, errors.New("cognitive event spine is required")
	}
	return &Workspace{config: config, events: events, runs: make(map[string]*run)}, nil
}

// Open starts proposal collection for a unique interaction ID.
func (w *Workspace) Open(ctx context.Context, request Request) error {
	if strings.TrimSpace(request.ID) == "" || strings.TrimSpace(request.InteractionID) == "" {
		return errors.New("workspace run and interaction IDs are required")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.runs[request.ID]; exists {
		return fmt.Errorf("workspace run %q already exists", request.ID)
	}
	if len(w.runs) >= w.config.MaxRuns {
		return ErrFull
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "workspace_opened", "run_id": request.ID,
		"interaction_id": request.InteractionID, "source_event_ids": request.SourceEventIDs,
	})
	if err != nil {
		return fmt.Errorf("encode workspace open event: %w", err)
	}
	if _, err := w.events.Emit(ctx, cognition.Draft{
		SessionID: request.SessionID, CorrelationID: request.InteractionID,
		ParentEventIDs: request.SourceEventIDs, Kind: cognition.KindActivation, Payload: payload,
	}); err != nil {
		return fmt.Errorf("record workspace opening: %w", err)
	}
	request.SourceEventIDs = append([]string(nil), request.SourceEventIDs...)
	w.runs[request.ID] = &run{report: Report{Request: request}, bySpecialist: make(map[string]struct{})}
	return nil
}

// Submit admits one attributed specialist response into an open bounded run.
func (w *Workspace) Submit(ctx context.Context, runID string, proposal agent.Proposal) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	current, exists := w.runs[runID]
	if !exists {
		return ErrRunNotFound
	}
	if current.report.Selection != nil {
		return ErrAlreadySelected
	}
	if len(current.report.Proposals) >= w.config.MaxProposals {
		return ErrFull
	}
	if err := validateProposal(current.report.Request, proposal); err != nil {
		return err
	}
	if _, exists := current.bySpecialist[proposal.SpecialistID]; exists {
		return fmt.Errorf("specialist %q already contributed to this interaction", proposal.SpecialistID)
	}
	for _, existing := range current.report.Proposals {
		if existing.ID == proposal.ID {
			return fmt.Errorf("proposal ID %q already exists in this interaction", proposal.ID)
		}
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "proposal_received", "run_id": runID,
		"proposal_id": proposal.ID, "proposal_event_id": proposal.EventID,
		"specialist_id": proposal.SpecialistID, "status": proposal.Status,
	})
	if err != nil {
		return fmt.Errorf("encode proposal admission event: %w", err)
	}
	if _, err := w.events.Emit(ctx, cognition.Draft{
		AgentID: proposal.SpecialistID, SessionID: current.report.Request.SessionID,
		CorrelationID: proposal.CorrelationID, ParentEventIDs: []string{proposal.EventID},
		Kind: cognition.KindProposal, Payload: payload,
	}); err != nil {
		return fmt.Errorf("record proposal admission: %w", err)
	}
	current.report.Proposals = append(current.report.Proposals, cloneProposal(proposal))
	current.bySpecialist[proposal.SpecialistID] = struct{}{}
	return nil
}

// Select ranks candidates by declared confidence with stable identity tie-breaks.
func (w *Workspace) Select(ctx context.Context, runID string) (Report, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	current, exists := w.runs[runID]
	if !exists {
		return Report{}, ErrRunNotFound
	}
	if current.report.Selection != nil {
		return cloneReport(current.report), ErrAlreadySelected
	}
	candidates := make([]agent.Proposal, 0, len(current.report.Proposals))
	for _, proposal := range current.report.Proposals {
		if proposal.Status == "candidate" {
			candidates = append(candidates, proposal)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence > candidates[j].Confidence
		}
		if candidates[i].SpecialistID != candidates[j].SpecialistID {
			return candidates[i].SpecialistID < candidates[j].SpecialistID
		}
		return candidates[i].ID < candidates[j].ID
	})
	selection := &Selection{
		Capacity:    w.config.Capacity,
		Policy:      "highest declared confidence; ties by specialist ID then proposal ID",
		Entries:     make([]SelectionEntry, 0, len(current.report.Proposals)),
		SelectedIDs: make([]string, 0, min(w.config.Capacity, len(candidates))),
	}
	selected := make(map[string]struct{}, w.config.Capacity)
	for index, proposal := range candidates {
		entry := SelectionEntry{
			ProposalID: proposal.ID, Specialist: proposal.SpecialistID,
			Confidence: proposal.Confidence,
		}
		if index < w.config.Capacity {
			entry.Selected = true
			entry.Reason = "ranked within workspace capacity"
			selection.SelectedIDs = append(selection.SelectedIDs, proposal.ID)
			selected[proposal.ID] = struct{}{}
		} else {
			entry.Reason = "ranked below admission capacity"
		}
		selection.Entries = append(selection.Entries, entry)
	}
	for _, proposal := range current.report.Proposals {
		if proposal.Status != "candidate" {
			selection.Entries = append(selection.Entries, SelectionEntry{
				ProposalID: proposal.ID, Specialist: proposal.SpecialistID,
				Confidence: proposal.Confidence, Reason: "proposal status is " + proposal.Status,
			})
		}
	}
	parents := append([]string(nil), current.report.Request.SourceEventIDs...)
	for _, proposal := range current.report.Proposals {
		parents = append(parents, proposal.EventID)
	}
	payload, err := json.Marshal(map[string]any{
		"operation": "proposals_selected", "run_id": runID,
		"interaction_id": current.report.Request.InteractionID,
		"policy":         selection.Policy, "capacity": selection.Capacity,
		"entries": selection.Entries, "selected_ids": selection.SelectedIDs,
	})
	if err != nil {
		return Report{}, fmt.Errorf("encode workspace selection event: %w", err)
	}
	event, err := w.events.Emit(ctx, cognition.Draft{
		SessionID:      current.report.Request.SessionID,
		CorrelationID:  current.report.Request.InteractionID,
		ParentEventIDs: unique(parents), Kind: cognition.KindSelection, Payload: payload,
	})
	if err != nil {
		return Report{}, fmt.Errorf("record workspace selection: %w", err)
	}
	selection.EventID = event.EventID
	current.report.Selection = selection
	return cloneReport(current.report), nil
}

// Broadcast sends selected proposals and the selection rationale to explicit recipients.
func (w *Workspace) Broadcast(ctx context.Context, runID, reason string, recipients []string, sender agent.Sender) (Report, error) {
	if strings.TrimSpace(reason) == "" || len(recipients) == 0 || len(recipients) > w.config.MaxRecipients || sender == nil {
		return Report{}, errors.New("broadcast requires a reason, bounded recipients, and sender")
	}
	seenRecipients := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		if strings.TrimSpace(recipient) == "" {
			return Report{}, errors.New("broadcast recipient IDs must not be empty")
		}
		if _, exists := seenRecipients[recipient]; exists {
			return Report{}, fmt.Errorf("broadcast recipient %q is duplicated", recipient)
		}
		seenRecipients[recipient] = struct{}{}
	}
	w.mu.Lock()
	current, exists := w.runs[runID]
	if !exists {
		w.mu.Unlock()
		return Report{}, ErrRunNotFound
	}
	if current.report.Selection == nil {
		w.mu.Unlock()
		return Report{}, errors.New("workspace must select proposals before broadcasting")
	}
	if current.report.BroadcastPlanEventID != "" || current.broadcasting {
		w.mu.Unlock()
		return Report{}, errors.New("workspace run has already been broadcast")
	}
	selected := make([]agent.Proposal, 0, len(current.report.Selection.SelectedIDs))
	selectedSet := make(map[string]struct{}, len(current.report.Selection.SelectedIDs))
	for _, id := range current.report.Selection.SelectedIDs {
		selectedSet[id] = struct{}{}
	}
	for _, proposal := range current.report.Proposals {
		if _, exists := selectedSet[proposal.ID]; exists {
			selected = append(selected, cloneProposal(proposal))
		}
	}
	selection := cloneSelection(*current.report.Selection)
	request := current.report.Request
	message := map[string]any{
		"run_id": runID, "interaction_id": request.InteractionID,
		"selection_event_id": selection.EventID, "selection_policy": selection.Policy,
		"selection_entries": selection.Entries, "broadcast_reason": reason,
		"proposals": selected,
	}
	payload, err := json.Marshal(message)
	if err != nil {
		w.mu.Unlock()
		return Report{}, fmt.Errorf("encode workspace broadcast: %w", err)
	}
	if len(payload) > w.config.MaxBroadcastBytes {
		w.mu.Unlock()
		return Report{}, ErrFull
	}
	planPayload, err := json.Marshal(map[string]any{
		"operation": "broadcast_planned", "run_id": runID,
		"interaction_id": request.InteractionID, "reason": reason,
		"recipients": recipients, "selected_proposal_ids": selection.SelectedIDs,
		"selection_event_id": selection.EventID,
	})
	if err != nil {
		w.mu.Unlock()
		return Report{}, fmt.Errorf("encode broadcast plan event: %w", err)
	}
	current.broadcasting = true
	parentIDs := append([]string{selection.EventID}, request.SourceEventIDs...)
	w.mu.Unlock()
	planEvent, err := w.events.Emit(ctx, cognition.Draft{
		SessionID: request.SessionID, CorrelationID: request.InteractionID,
		ParentEventIDs: unique(parentIDs), Kind: cognition.KindDecision, Payload: planPayload,
	})
	if err != nil {
		w.mu.Lock()
		if current, exists := w.runs[runID]; exists {
			current.broadcasting = false
		}
		w.mu.Unlock()
		return Report{}, fmt.Errorf("record workspace broadcast plan: %w", err)
	}
	w.mu.Lock()
	current, exists = w.runs[runID]
	if !exists || current.report.BroadcastPlanEventID != "" || !current.broadcasting {
		w.mu.Unlock()
		return Report{}, errors.New("workspace run changed while broadcast was being planned")
	}
	current.report.BroadcastPlanEventID = planEvent.EventID
	current.report.BroadcastReason = reason
	current.report.Recipients = append([]string(nil), recipients...)
	current.broadcasting = false
	w.mu.Unlock()

	var deliveryErrors []error
	for _, recipient := range recipients {
		delivery := Delivery{Recipient: recipient}
		if sendErr := sender.Send(ctx, recipient, BroadcastMessage, payload); sendErr != nil {
			delivery.Error = sendErr.Error()
			deliveryErrors = append(deliveryErrors, fmt.Errorf("broadcast to %q: %w", recipient, sendErr))
		} else {
			delivery.Sent = true
		}
		deliveryPayload, marshalErr := json.Marshal(map[string]any{
			"operation": "broadcast_delivered", "run_id": runID,
			"recipient": recipient, "sent": delivery.Sent, "error": delivery.Error,
		})
		if marshalErr != nil {
			deliveryErrors = append(deliveryErrors, marshalErr)
		} else if event, emitErr := w.events.Emit(ctx, cognition.Draft{
			SessionID: request.SessionID, CorrelationID: request.InteractionID,
			ParentEventIDs: []string{planEvent.EventID}, Kind: cognition.KindDecision, Payload: deliveryPayload,
		}); emitErr != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("record broadcast result to %q: %w", recipient, emitErr))
		} else {
			delivery.EventID = event.EventID
		}
		w.mu.Lock()
		current.report.Deliveries = append(current.report.Deliveries, delivery)
		w.mu.Unlock()
	}
	report, inspectErr := w.Inspect(ctx, runID)
	return report, errors.Join(errors.Join(deliveryErrors...), inspectErr)
}

// Inspect returns an immutable snapshot of collection, selection, and delivery state.
func (w *Workspace) Inspect(ctx context.Context, runID string) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	current, exists := w.runs[runID]
	if !exists {
		return Report{}, ErrRunNotFound
	}
	return cloneReport(current.report), nil
}

func validateProposal(request Request, proposal agent.Proposal) error {
	if proposal.InteractionID != request.InteractionID || strings.TrimSpace(proposal.ID) == "" ||
		strings.TrimSpace(proposal.EventID) == "" || strings.TrimSpace(proposal.SpecialistID) == "" {
		return errors.New("proposal requires matching interaction and stable proposal, event, and specialist IDs")
	}
	if proposal.Status != "candidate" && proposal.Status != "failed" {
		return fmt.Errorf("unsupported proposal status %q", proposal.Status)
	}
	if math.IsNaN(proposal.Confidence) || math.IsInf(proposal.Confidence, 0) || proposal.Confidence < 0 || proposal.Confidence > 1 {
		return errors.New("proposal confidence must be between zero and one")
	}
	return nil
}

func cloneProposal(proposal agent.Proposal) agent.Proposal {
	proposal.EvidenceEventIDs = append([]string(nil), proposal.EvidenceEventIDs...)
	proposal.RequestedActions = append([]agent.RequestedAction(nil), proposal.RequestedActions...)
	for i := range proposal.RequestedActions {
		proposal.RequestedActions[i].Payload = append(json.RawMessage(nil), proposal.RequestedActions[i].Payload...)
	}
	return proposal
}

func cloneSelection(selection Selection) *Selection {
	selection.Entries = append([]SelectionEntry(nil), selection.Entries...)
	selection.SelectedIDs = append([]string(nil), selection.SelectedIDs...)
	return &selection
}

func cloneReport(report Report) Report {
	report.Request.SourceEventIDs = append([]string(nil), report.Request.SourceEventIDs...)
	report.Proposals = append([]agent.Proposal(nil), report.Proposals...)
	for i := range report.Proposals {
		report.Proposals[i] = cloneProposal(report.Proposals[i])
	}
	if report.Selection != nil {
		report.Selection = cloneSelection(*report.Selection)
	}
	report.Recipients = append([]string(nil), report.Recipients...)
	report.Deliveries = append([]Delivery(nil), report.Deliveries...)
	return report
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
