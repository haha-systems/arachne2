// Package main demonstrates evidence-backed Silk procedure acquisition by Arachne.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/governance"
	"github.com/haha-systems/arachne2/internal/inspection"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/silk"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: silk-acquisition <path-to-silk-binary>")
		os.Exit(2)
	}
	ctx := context.Background()
	events := newSpine()
	client, err := silk.Start(ctx, silk.Command{Path: os.Args[1], Args: []string{"serve"}, Err: os.Stderr})
	fatalIf(err)
	defer func() { fatalIf(client.Close()) }()
	session, err := client.CreateSession(ctx, silk.SessionConfig{
		ID: "acquisition-session", Grants: []silk.Grant{},
		Limits: silk.Limits{Fuel: 10000, CallDepth: 16, TimeoutMS: 5000},
	})
	fatalIf(err)
	pattern := createRepeatedPattern(ctx, events)
	candidate := prepareCandidate(ctx, client, session.ID, pattern)
	artifact := artifactIdentity(candidate.Artifact)
	decision, policy := governRetention(ctx, events, pattern, artifact, candidate.EntryProcedure)
	runtime := acquisitionRuntime{client: client, events: events, sessionID: session.ID}
	retentionEventID := runtime.retain(ctx, retentionRequest{candidate: candidate, artifact: artifact, decision: decision, policy: policy})
	runtime.runAndRecord(ctx, candidate, artifact, retentionEventID)
	inspector, err := inspection.New(events)
	fatalIf(err)
	report, err := inspector.Inspect(ctx, "silk-acquisition-example")
	fatalIf(err)
	for _, activity := range report.Activities {
		if activity.Category == inspection.CategoryProcedure {
			fmt.Printf("inspection: %s event=%s governance=%v provenance=%v\n",
				activity.Summary, activity.EventID, activity.GovernanceEventIDs, activity.ProvenanceEventIDs)
		}
	}
}

func newSpine() *cognition.Spine {
	store, err := cognition.NewMemoryStore(256)
	fatalIf(err)
	events, err := cognition.NewSpine("silk-acquisition-example", store)
	fatalIf(err)
	return events
}

func createRepeatedPattern(ctx context.Context, events *cognition.Spine) memory.ConsolidatedPattern {
	store, err := memory.NewMemoryStore("organism-acquisition")
	fatalIf(err)
	memories, err := memory.NewService("organism-acquisition", store, events)
	fatalIf(err)
	for _, id := range []string{"experience-1", "experience-2"} {
		source, emitErr := events.Emit(ctx, cognition.Draft{
			CorrelationID: id, Kind: cognition.KindPerception,
			Payload: json.RawMessage(`{"task":"add two small numbers"}`),
		})
		fatalIf(emitErr)
		_, recordErr := memories.RecordEpisode(ctx, memory.Episode{
			ID: id, Source: memory.SourceRef{Kind: "worked-example", ID: id},
			SourceEventIDs: []string{source.EventID}, OccurredAt: time.Now().UTC(),
			Kind:    "successful-operation",
			Content: json.RawMessage(`{"operation":"sum","inputs":[2,3],"output":5}`),
			Tags:    []string{"arithmetic", "repeatable"},
		})
		fatalIf(recordErr)
	}
	run, patterns, err := memories.Consolidate(ctx, memory.ConsolidationPolicy{
		MinimumSupport: 2, EpisodeQuery: memory.EpisodeQuery{Limit: 8},
	})
	fatalIf(err)
	if len(patterns) != 1 || patterns[0].Support != 2 {
		panic(fmt.Sprintf("expected one two-example pattern, got run %s with %d patterns", run.ID, len(patterns)))
	}
	fmt.Printf("recurrence: pattern=%s support=%d evidence=%v\n", patterns[0].ID, patterns[0].Support, patterns[0].SourceEventIDs)
	return patterns[0]
}

func prepareCandidate(ctx context.Context, client *silk.Client, sessionID string, pattern memory.ConsolidatedPattern) silk.PreparedCandidate {
	request := map[string]any{
		"procedure_id": "repeated.sum",
		"name":         "repeated-sum",
		"semantic_description": map[string]any{
			"summary": "Add two numeric inputs, as observed in repeated successful examples.",
			"terms":   []string{"addition", "arithmetic", "sum"},
		},
		"contract": map[string]any{
			"purpose":        "Return the sum of two numeric inputs.",
			"input_schema":   map[string]any{"type": "array", "items": map[string]string{"type": "number"}, "minItems": 2, "maxItems": 2},
			"output_schema":  map[string]string{"type": "number"},
			"effect_ceiling": []string{}, "required_authorities": []string{}, "dependencies": []string{},
		},
		"provenance": map[string]any{
			"origin": "generated", "source_reference": "memory-pattern:" + pattern.ID,
			"generator": "arachne.recurrence-example.v1", "claims": map[string]string{"pattern_id": pattern.ID},
		},
		"entry_procedure":              "main",
		"required_validation_profiles": []string{"silk.syntax_lowering.v1", "silk.effects_authority.v1"},
	}
	requestJSON, err := json.Marshal(request)
	fatalIf(err)
	candidate, err := client.PrepareCandidate(ctx, sessionID, `fn main(left, right) { return left + right; }`, requestJSON)
	fatalIf(err)
	fmt.Printf("Silk prepared: entry=%s retention=%s\n", candidate.EntryProcedure, candidate.RetentionState)
	return candidate
}

type procedureIdentity struct {
	ProcedureID    string `json:"procedure_id"`
	RevisionDigest string `json:"revision_digest"`
}

type acquisitionRuntime struct {
	client    *silk.Client
	events    *cognition.Spine
	sessionID string
}

type retentionRequest struct {
	candidate silk.PreparedCandidate
	artifact  procedureIdentity
	decision  governance.Decision
	policy    governance.Policy
}

func artifactIdentity(raw json.RawMessage) procedureIdentity {
	var identity procedureIdentity
	fatalIf(json.Unmarshal(raw, &identity))
	return identity
}

func governRetention(ctx context.Context, events *cognition.Spine, pattern memory.ConsolidatedPattern, artifact procedureIdentity, entryProcedure string) (governance.Decision, governance.Policy) {
	target := "silk:procedure:" + artifact.ProcedureID + "@" + artifact.RevisionDigest
	policy := governance.Policy{
		ID: "procedure-retention", Version: "1", Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionProcedureRetention: {
				MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{target},
			},
		},
	}
	governor, err := governance.New(policy, events)
	fatalIf(err)
	proposal := governance.Proposal{
		ID: "retain-" + artifact.ProcedureID, InteractionID: "interaction-acquisition",
		ProposerID: "specialist/recurrence", Class: governance.ActionProcedureRetention,
		Target: target, Action: "retain Silk procedure revision",
		Payload:   json.RawMessage(fmt.Sprintf(`{"pattern_id":%q,"revision_digest":%q,"entry_procedure":%q}`, pattern.ID, artifact.RevisionDigest, entryProcedure)),
		CreatedAt: time.Now().UTC(), SourceEventIDs: pattern.SourceEventIDs,
		EvidenceEventIDs: pattern.SourceEventIDs,
	}
	proposalDigest, err := governance.ProposalDigest(proposal)
	fatalIf(err)
	policyDigest, err := governance.PolicyDigest(policy)
	fatalIf(err)
	review, err := events.Emit(ctx, cognition.Draft{
		AgentID: "operator", CorrelationID: proposal.InteractionID,
		ParentEventIDs: pattern.SourceEventIDs, Kind: cognition.KindDecision,
		Payload: json.RawMessage(`{"decision":"approve exact procedure revision"}`),
	})
	fatalIf(err)
	decision, err := governor.Evaluate(ctx, proposal, []governance.Approval{{
		ID: "review-acquisition", ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: policyDigest, Decision: governance.ApprovalApprove,
		Reason: "reviewed repeated evidence and validated candidate", DecidedAt: time.Now().UTC(),
		SourceEventID: review.EventID,
	}})
	fatalIf(err)
	fmt.Printf("Arachne governance: outcome=%s event=%s\n", decision.Outcome, decision.EventID)
	if decision.Outcome != governance.OutcomeApproved {
		panic("procedure retention was not approved")
	}
	return decision, policy
}

func (runtime acquisitionRuntime) retain(ctx context.Context, request retentionRequest) string {
	admission, err := runtime.client.AdmitCandidate(ctx, request.candidate)
	fatalIf(err)
	if admission.RetentionState != "candidate" {
		panic("newly admitted procedure did not enter candidate state")
	}
	if admission.EntryProcedure != request.candidate.EntryProcedure {
		panic("Silk admitted a different entry procedure than Arachne approved")
	}
	admissionEvent := recordBoundaryEvent(ctx, runtime.events, request.decision.EventID, "silk_registry_admitted", admission)
	policyDigest, err := governance.PolicyDigest(request.policy)
	fatalIf(err)
	decisionEvidence, err := json.Marshal(map[string]string{
		"decision_id": request.decision.ID, "decision_event_id": request.decision.EventID,
		"proposal_digest": request.decision.ProposalDigest, "policy_digest": policyDigest,
	})
	fatalIf(err)
	evidenceDigest := sha256.Sum256(decisionEvidence)
	retention := map[string]any{
		"actor": "operator", "reason": "Arachne governance decision " + request.decision.ID,
		"evidence": map[string]any{
			"validator": "arachne.governance", "validator_version": "1",
			"profile": "silk.retention_approval.v1", "subject_revision_digest": request.artifact.RevisionDigest,
			"outcome": "passed", "evidence_digest": "sha256:" + hex.EncodeToString(evidenceDigest[:]),
		},
	}
	retentionJSON, err := json.Marshal(retention)
	fatalIf(err)
	fatalIf(runtime.client.RetainCandidate(ctx, request.artifact.ProcedureID, request.artifact.RevisionDigest, retentionJSON))
	stateEvent := recordBoundaryEvent(ctx, runtime.events, admissionEvent.EventID, "silk_registry_retained", map[string]string{
		"procedure_id": request.artifact.ProcedureID, "revision_digest": request.artifact.RevisionDigest,
		"governance_decision_id": request.decision.ID,
	})
	fmt.Printf("Silk retained: procedure=%s revision=%s event=%s\n", request.artifact.ProcedureID, request.artifact.RevisionDigest, stateEvent.EventID)
	return stateEvent.EventID
}

func (runtime acquisitionRuntime) runAndRecord(ctx context.Context, candidate silk.PreparedCandidate, artifact procedureIdentity, retentionEventID string) {
	run, err := runtime.client.Run(ctx, silk.ProcedureCall{
		SessionID: runtime.sessionID, Procedure: candidate.EntryProcedure,
		Arguments: []json.RawMessage{json.RawMessage("2"), json.RawMessage("3")},
		Retained:  &silk.RegisteredRevision{ProcedureID: artifact.ProcedureID, RevisionDigest: artifact.RevisionDigest},
	})
	fatalIf(err)
	if string(run.Value) != "5" {
		panic(fmt.Sprintf("retained procedure returned %s, expected 5", run.Value))
	}
	parents := []string{retentionEventID}
	for _, trace := range run.Trace {
		event, recordErr := runtime.events.RecordSilkTrace(ctx, "acquisition", runtime.sessionID, "interaction-acquisition", parents, trace)
		fatalIf(recordErr)
		parents = append(parents, event.EventID)
	}
	action, err := runtime.events.Emit(ctx, cognition.Draft{
		AgentID: "acquisition", SessionID: runtime.sessionID, CorrelationID: "interaction-acquisition",
		ParentEventIDs: parents, Kind: cognition.KindAction,
		Payload: json.RawMessage(fmt.Sprintf(`{"procedure_id":%q,"revision_digest":%q,"result":%s}`, artifact.ProcedureID, artifact.RevisionDigest, run.Value)),
	})
	fatalIf(err)
	fmt.Printf("reused retained procedure: value=%s trace_events=%d action_event=%s\n", run.Value, len(run.Trace), action.EventID)
}

func recordBoundaryEvent(ctx context.Context, events *cognition.Spine, parent, operation string, payload any) cognition.Event {
	encoded, err := json.Marshal(map[string]any{"operation": operation, "result": payload})
	fatalIf(err)
	event, err := events.Emit(ctx, cognition.Draft{
		CorrelationID: "interaction-acquisition", ParentEventIDs: []string{parent},
		Kind: cognition.KindDevelopment, Payload: encoded,
	})
	fatalIf(err)
	return event
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
