// Package governance evaluates Arachne approval policy above runtime authorization.
package governance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

// ActionClass identifies which governance rule applies to a proposed change or action.
type ActionClass string

// Governance action classes identify consequential behavior subject to policy.
const (
	ActionExternalEffect     ActionClass = "external_effect"
	ActionHighImpact         ActionClass = "high_impact"
	ActionMemoryMutation     ActionClass = "memory_mutation"
	ActionSemanticPromotion  ActionClass = "semantic_promotion"
	ActionIdentityChange     ActionClass = "identity_change"
	ActionProcedureRetention ActionClass = "procedure_retention"
	ActionStructuralChange   ActionClass = "structural_change"
)

// Outcome is the Arachne governance decision, independent of runtime authorization.
type Outcome string

// Governance outcomes describe eligibility after policy evaluation.
const (
	OutcomeApproved Outcome = "approved"
	OutcomeRejected Outcome = "rejected"
	OutcomePending  Outcome = "pending"
)

// ApprovalDecision is a trusted host reviewer's explicit decision.
type ApprovalDecision string

// Approval decisions are explicit choices made by host-authenticated reviewers.
const (
	ApprovalApprove ApprovalDecision = "approve"
	ApprovalReject  ApprovalDecision = "reject"
)

// Rule applies required evidence, target scope, and independent approval count.
type Rule struct {
	MinimumApprovals int      `json:"minimum_approvals"`
	RequireEvidence  bool     `json:"require_evidence"`
	AllowedTargets   []string `json:"allowed_targets"`
}

// Policy is versioned, explicit governance configuration supplied by the host.
type Policy struct {
	ID        string               `json:"id"`
	Version   string               `json:"version"`
	Approvers []string             `json:"approvers"`
	Rules     map[ActionClass]Rule `json:"rules"`
}

// Proposal describes one consequential intent without executing it.
type Proposal struct {
	ID               string          `json:"id"`
	InteractionID    string          `json:"interaction_id"`
	ProposerID       string          `json:"proposer_id"`
	Class            ActionClass     `json:"action_class"`
	Target           string          `json:"target"`
	Action           string          `json:"action"`
	Payload          json.RawMessage `json:"payload"`
	CreatedAt        time.Time       `json:"created_at"`
	SourceEventIDs   []string        `json:"source_event_ids,omitempty"`
	EvidenceEventIDs []string        `json:"evidence_event_ids,omitempty"`
}

// Approval binds one trusted reviewer's decision to one proposal and policy digest.
type Approval struct {
	ID             string           `json:"id"`
	ApproverID     string           `json:"approver_id"`
	ProposalDigest string           `json:"proposal_digest"`
	PolicyDigest   string           `json:"policy_digest"`
	Decision       ApprovalDecision `json:"decision"`
	Reason         string           `json:"reason"`
	DecidedAt      time.Time        `json:"decided_at"`
	SourceEventID  string           `json:"source_event_id,omitempty"`
}

// ApprovalRecord preserves the review decision and its binding in the audit result.
type ApprovalRecord struct {
	ID             string           `json:"id"`
	ApproverID     string           `json:"approver_id"`
	Decision       ApprovalDecision `json:"decision"`
	Reason         string           `json:"reason"`
	ProposalDigest string           `json:"proposal_digest"`
	PolicyDigest   string           `json:"policy_digest"`
	DecidedAt      time.Time        `json:"decided_at"`
	SourceEventID  string           `json:"source_event_id,omitempty"`
}

// Decision is the inspectable policy result; approved means eligible, not executed.
type Decision struct {
	ID               string           `json:"id"`
	ProposalID       string           `json:"proposal_id"`
	ProposalDigest   string           `json:"proposal_digest"`
	PolicyID         string           `json:"policy_id"`
	PolicyVersion    string           `json:"policy_version"`
	PolicyDigest     string           `json:"policy_digest"`
	Class            ActionClass      `json:"action_class"`
	Target           string           `json:"target"`
	Action           string           `json:"action"`
	PayloadDigest    string           `json:"payload_digest"`
	MinimumApprovals int              `json:"minimum_approvals,omitempty"`
	RequireEvidence  bool             `json:"require_evidence,omitempty"`
	AllowedTargets   []string         `json:"allowed_targets,omitempty"`
	Outcome          Outcome          `json:"outcome"`
	Reasons          []string         `json:"reasons"`
	Approvals        []ApprovalRecord `json:"approvals,omitempty"`
	ApprovalIDs      []string         `json:"approval_ids,omitempty"`
	ApproverIDs      []string         `json:"approver_ids,omitempty"`
	SourceEventIDs   []string         `json:"source_event_ids,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	EventID          string           `json:"event_id"`
}

// Service evaluates proposals and records every result through the shared event spine.
type Service struct {
	policy Policy
	digest string
	events *cognition.Spine
	now    func() time.Time
}

// New creates a governance service with a validated, versioned policy.
func New(policy Policy, events *cognition.Spine) (*Service, error) {
	if events == nil {
		return nil, errors.New("cognitive event spine is required")
	}
	if err := validatePolicy(policy); err != nil {
		return nil, err
	}
	digest, err := PolicyDigest(policy)
	if err != nil {
		return nil, err
	}
	return &Service{policy: clonePolicy(policy), digest: digest, events: events, now: time.Now}, nil
}

// PolicyDigest returns a stable digest over the explicit policy, including target scopes.
func PolicyDigest(policy Policy) (string, error) {
	policy = clonePolicy(policy)
	sort.Strings(policy.Approvers)
	for class, rule := range policy.Rules {
		sort.Strings(rule.AllowedTargets)
		policy.Rules[class] = rule
	}
	canonical, err := canonicalJSON(policy)
	if err != nil {
		return "", fmt.Errorf("canonicalize governance policy: %w", err)
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

// ProposalDigest binds approvals to every material proposal field and canonical payload.
func ProposalDigest(proposal Proposal) (string, error) {
	proposal.SourceEventIDs = uniqueSorted(proposal.SourceEventIDs)
	proposal.EvidenceEventIDs = uniqueSorted(proposal.EvidenceEventIDs)
	canonical, err := canonicalJSON(proposal)
	if err != nil {
		return "", fmt.Errorf("canonicalize governance proposal: %w", err)
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

// Evaluate records an approved, rejected, or pending decision without executing the proposal.
func (s *Service) Evaluate(ctx context.Context, proposal Proposal, approvals []Approval) (Decision, error) {
	if err := validateProposal(proposal); err != nil {
		return Decision{}, err
	}
	rule, ruleExists := s.policy.Rules[proposal.Class]
	proposalDigest, err := ProposalDigest(proposal)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{
		ProposalID: proposal.ID, ProposalDigest: proposalDigest,
		PolicyID: s.policy.ID, PolicyVersion: s.policy.Version, PolicyDigest: s.digest,
		Class: proposal.Class, Target: proposal.Target, Action: proposal.Action, Outcome: OutcomeRejected,
		Reasons: make([]string, 0), CreatedAt: s.now().UTC(),
		SourceEventIDs: uniqueSorted(append(append([]string(nil), proposal.SourceEventIDs...), proposal.EvidenceEventIDs...)),
	}
	payloadHash := sha256.Sum256(proposal.Payload)
	decision.PayloadDigest = hex.EncodeToString(payloadHash[:])
	if !ruleExists {
		decision.Reasons = append(decision.Reasons, "policy has no rule for this action class")
		return s.record(ctx, proposal, decision)
	}
	decision.MinimumApprovals = rule.MinimumApprovals
	decision.RequireEvidence = rule.RequireEvidence
	decision.AllowedTargets = append([]string(nil), rule.AllowedTargets...)
	if !targetAllowed(rule.AllowedTargets, proposal.Target) {
		decision.Reasons = append(decision.Reasons, "target is outside the policy scope")
		return s.record(ctx, proposal, decision)
	}
	if rule.RequireEvidence && len(proposal.EvidenceEventIDs) == 0 {
		decision.Reasons = append(decision.Reasons, "policy requires source evidence")
		return s.record(ctx, proposal, decision)
	}
	approvalIDs, approverIDs, records, approvalSources, hasRejection, invalidReasons := s.validateApprovals(proposal, proposalDigest, approvals)
	decision.ApprovalIDs = approvalIDs
	decision.ApproverIDs = approverIDs
	decision.Approvals = records
	decision.SourceEventIDs = uniqueSorted(append(decision.SourceEventIDs, approvalSources...))
	if len(invalidReasons) > 0 {
		decision.Reasons = append(decision.Reasons, invalidReasons...)
		return s.record(ctx, proposal, decision)
	}
	if hasRejection {
		decision.Reasons = append(decision.Reasons, "an authorized reviewer rejected the proposal")
		return s.record(ctx, proposal, decision)
	}
	if len(approverIDs) < rule.MinimumApprovals {
		decision.Outcome = OutcomePending
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("requires %d distinct approvals; received %d", rule.MinimumApprovals, len(approverIDs)))
		return s.record(ctx, proposal, decision)
	}
	decision.Outcome = OutcomeApproved
	decision.Reasons = append(decision.Reasons, "target, evidence, and approval requirements satisfied")
	return s.record(ctx, proposal, decision)
}

func (s *Service) record(ctx context.Context, proposal Proposal, decision Decision) (Decision, error) {
	decision.ID = stableID(decision.ProposalDigest, decision.PolicyDigest, string(decision.Outcome), strings.Join(decision.ApprovalIDs, ","))
	payload, err := json.Marshal(decision)
	if err != nil {
		return Decision{}, fmt.Errorf("encode governance decision: %w", err)
	}
	event, err := s.events.Emit(ctx, cognition.Draft{
		AgentID: proposal.ProposerID, CorrelationID: proposal.InteractionID,
		ParentEventIDs: decision.SourceEventIDs, Kind: cognition.KindGovernance, Payload: payload,
	})
	if err != nil {
		return Decision{}, fmt.Errorf("record governance decision: %w", err)
	}
	decision.EventID = event.EventID
	return decision, nil
}

func (s *Service) validateApprovals(proposal Proposal, proposalDigest string, approvals []Approval) ([]string, []string, []ApprovalRecord, []string, bool, []string) {
	allowed := make(map[string]struct{}, len(s.policy.Approvers))
	for _, approver := range s.policy.Approvers {
		allowed[approver] = struct{}{}
	}
	seenApprovers := make(map[string]struct{}, len(approvals))
	seenIDs := make(map[string]struct{}, len(approvals))
	approvalIDs := make([]string, 0, len(approvals))
	approverIDs := make([]string, 0, len(approvals))
	records := make([]ApprovalRecord, 0, len(approvals))
	sources := make([]string, 0, len(approvals))
	reasons := make([]string, 0)
	hasRejection := false
	for _, approval := range approvals {
		if strings.TrimSpace(approval.ID) == "" || strings.TrimSpace(approval.ApproverID) == "" || strings.TrimSpace(approval.Reason) == "" || approval.DecidedAt.IsZero() {
			reasons = append(reasons, "approval is missing identity, reason, or time")
			continue
		}
		if _, authorized := allowed[approval.ApproverID]; !authorized {
			reasons = append(reasons, fmt.Sprintf("approver %q is not authorized by policy", approval.ApproverID))
			continue
		}
		if _, duplicate := seenIDs[approval.ID]; duplicate {
			reasons = append(reasons, fmt.Sprintf("approval ID %q was submitted more than once", approval.ID))
			continue
		}
		if approval.ProposalDigest != proposalDigest || approval.PolicyDigest != s.digest {
			reasons = append(reasons, fmt.Sprintf("approval %q is bound to a different proposal or policy", approval.ID))
			continue
		}
		if approval.DecidedAt.Before(proposal.CreatedAt) {
			reasons = append(reasons, fmt.Sprintf("approval %q predates its proposal", approval.ID))
			continue
		}
		if approval.Decision != ApprovalApprove && approval.Decision != ApprovalReject {
			reasons = append(reasons, fmt.Sprintf("approval %q has an unsupported decision", approval.ID))
			continue
		}
		if _, duplicate := seenApprovers[approval.ApproverID]; duplicate {
			reasons = append(reasons, fmt.Sprintf("approver %q was counted more than once", approval.ApproverID))
			continue
		}
		seenApprovers[approval.ApproverID] = struct{}{}
		seenIDs[approval.ID] = struct{}{}
		approvalIDs = append(approvalIDs, approval.ID)
		approverIDs = append(approverIDs, approval.ApproverID)
		records = append(records, ApprovalRecord{
			ID: approval.ID, ApproverID: approval.ApproverID, Decision: approval.Decision,
			Reason: approval.Reason, ProposalDigest: approval.ProposalDigest,
			PolicyDigest: approval.PolicyDigest, DecidedAt: approval.DecidedAt.UTC(), SourceEventID: approval.SourceEventID,
		})
		sources = append(sources, approval.SourceEventID)
		if approval.Decision == ApprovalReject {
			hasRejection = true
		}
	}
	sort.Strings(approvalIDs)
	sort.Strings(approverIDs)
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return approvalIDs, approverIDs, records, sources, hasRejection, reasons
}

func validatePolicy(policy Policy) error {
	if strings.TrimSpace(policy.ID) == "" || strings.TrimSpace(policy.Version) == "" || len(policy.Rules) == 0 {
		return errors.New("governance policy ID, version, and rules are required")
	}
	if len(policy.Approvers) == 0 {
		return errors.New("governance policy must name at least one trusted approver")
	}
	approvers := make(map[string]struct{}, len(policy.Approvers))
	for _, approver := range policy.Approvers {
		if strings.TrimSpace(approver) == "" {
			return errors.New("governance approver IDs must not be empty")
		}
		if _, exists := approvers[approver]; exists {
			return fmt.Errorf("duplicate governance approver %q", approver)
		}
		approvers[approver] = struct{}{}
	}
	for class, rule := range policy.Rules {
		if !validActionClass(class) || rule.MinimumApprovals < 1 || rule.MinimumApprovals > len(approvers) || len(rule.AllowedTargets) == 0 {
			return fmt.Errorf("governance rule %q requires a known class, approval threshold, and explicit target scope", class)
		}
		for _, target := range rule.AllowedTargets {
			if strings.TrimSpace(target) == "" {
				return fmt.Errorf("governance rule %q contains an empty target", class)
			}
		}
	}
	return nil
}

func validateProposal(proposal Proposal) error {
	if strings.TrimSpace(proposal.ID) == "" || strings.TrimSpace(proposal.InteractionID) == "" ||
		strings.TrimSpace(proposal.ProposerID) == "" || strings.TrimSpace(proposal.Target) == "" ||
		strings.TrimSpace(proposal.Action) == "" || proposal.CreatedAt.IsZero() {
		return errors.New("governance proposal requires identity, target, action, and creation time")
	}
	if len(proposal.Payload) == 0 || !json.Valid(proposal.Payload) {
		return errors.New("governance proposal payload must be valid JSON")
	}
	if !validActionClass(proposal.Class) {
		return fmt.Errorf("unsupported governance action class %q", proposal.Class)
	}
	return nil
}

func validActionClass(class ActionClass) bool {
	switch class {
	case ActionExternalEffect, ActionHighImpact, ActionMemoryMutation, ActionSemanticPromotion,
		ActionIdentityChange, ActionProcedureRetention, ActionStructuralChange:
		return true
	default:
		return false
	}
}

func targetAllowed(allowed []string, target string) bool {
	for _, candidate := range allowed {
		if candidate == "*" || candidate == target {
			return true
		}
	}
	return false
}

func canonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func clonePolicy(policy Policy) Policy {
	clone := Policy{ID: policy.ID, Version: policy.Version, Approvers: append([]string(nil), policy.Approvers...), Rules: make(map[ActionClass]Rule, len(policy.Rules))}
	for class, rule := range policy.Rules {
		rule.AllowedTargets = append([]string(nil), rule.AllowedTargets...)
		clone.Rules[class] = rule
	}
	return clone
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func stableID(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}
