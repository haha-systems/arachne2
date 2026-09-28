// Package main runs a traceable, bounded engineering task across Arachne and Silk.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/development"
	"github.com/haha-systems/arachne2/internal/governance"
	"github.com/haha-systems/arachne2/internal/inspection"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/silk"
	"github.com/haha-systems/arachne2/internal/workspace"
)

const organismID = "integrated-engineering-organism"

type experimentReport struct {
	OrganismID        string                  `json:"organism_id"`
	Task              string                  `json:"task"`
	TaskOutcome       string                  `json:"task_outcome"`
	InitialRouting    map[string]float64      `json:"initial_routing"`
	FinalRouting      map[string]float64      `json:"final_routing"`
	SelectedProposal  agent.Proposal          `json:"selected_proposal"`
	Workspace         workspace.Report        `json:"workspace"`
	Regulation        regulation.Snapshot     `json:"regulation"`
	Consolidation     memory.ConsolidationRun `json:"consolidation"`
	Replay            memory.ReplayRun        `json:"replay"`
	FailedPath        failedPath              `json:"failed_path"`
	TaskFailure       *failedPath             `json:"task_failure,omitempty"`
	AcquiredProcedure *procedureResult        `json:"acquired_procedure,omitempty"`
	Development       *development.Change     `json:"development,omitempty"`
	Evolution         inspection.Evolution    `json:"evolution"`
	InspectedHistory  inspection.Report       `json:"inspected_history"`
	Events            []cognition.Event       `json:"events"`
}

type failedPath struct {
	Stage  string `json:"stage"`
	Reason string `json:"reason"`
	Event  string `json:"event_id"`
}

type procedureResult struct {
	ProcedureID    string          `json:"procedure_id"`
	RevisionDigest string          `json:"revision_digest"`
	EntryProcedure string          `json:"entry_procedure"`
	Output         json.RawMessage `json:"output"`
	ActionEventID  string          `json:"action_event_id"`
	RetentionEvent string          `json:"retention_event_id"`
}

type procedureIdentity struct {
	ProcedureID    string `json:"procedure_id"`
	RevisionDigest string `json:"revision_digest"`
}

type retainedProcedure struct {
	candidate silk.PreparedCandidate
	identity  procedureIdentity
	eventID   string
}

type runState struct {
	events           *cognition.Spine
	memory           *memory.Service
	governor         *governance.Service
	policyDigest     string
	engine           *development.Engine
	client           *silk.Client
	sessionID        string
	forceTaskFailure bool
}

type coordinationResult struct {
	proposals  []agent.Proposal
	workspace  workspace.Report
	regulation regulation.Snapshot
	err        error
}

type coordinationInput struct {
	workspace  *workspace.Workspace
	regulation *regulation.Regulator
	sourceID   string
}

type experimentSetup struct {
	PatternRun memory.ConsolidationRun
	Pattern    memory.ConsolidatedPattern
	Inspector  *inspection.Inspector
	Initial    inspection.Report
	Replay     memory.ReplayRun
	Selected   agent.Proposal
	Workspace  workspace.Report
	Regulation regulation.Snapshot
}

type experimentOutcome struct {
	InitialRouting map[string]float64
	FailedPath     failedPath
	TaskFailure    *failedPath
	Procedure      *procedureResult
	Development    *development.Change
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 || len(os.Args) == 3 && os.Args[2] != "--fail-task" {
		fmt.Fprintln(os.Stderr, "usage: integrated-engineering <path-to-silk-binary> [--fail-task]")
		os.Exit(2)
	}
	ctx := context.Background()
	state := newRunState(ctx, os.Args[1], len(os.Args) == 3)
	defer func() { fatalIf(state.client.Close()) }()
	report, err := runExperiment(ctx, state)
	fatalIf(err)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	fatalIf(encoder.Encode(report))
}

func newRunState(ctx context.Context, silkPath string, forceTaskFailure bool) runState {
	eventStore, err := cognition.NewMemoryStore(2048)
	fatalIf(err)
	events, err := cognition.NewSpine(organismID, eventStore)
	fatalIf(err)
	memoryStore, err := memory.NewMemoryStore(organismID)
	fatalIf(err)
	memories, err := memory.NewService(organismID, memoryStore, events)
	fatalIf(err)
	policy := governance.Policy{
		ID: "integrated-engineering-policy", Version: "1", Approvers: []string{"operator"},
		Rules: map[governance.ActionClass]governance.Rule{
			governance.ActionProcedureRetention: {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"}},
			governance.ActionHighImpact:         {MinimumApprovals: 1, RequireEvidence: true, AllowedTargets: []string{"*"}},
		},
	}
	governor, err := governance.New(policy, events)
	fatalIf(err)
	policyDigest, err := governance.PolicyDigest(policy)
	fatalIf(err)
	engine, err := development.NewEngine(ctx, organismID, map[string]float64{"engineering:estimate": 0.5}, governor, events)
	fatalIf(err)
	client, err := silk.Start(ctx, silk.Command{Path: silkPath, Args: []string{"serve"}, Err: os.Stderr})
	fatalIf(err)
	session, err := client.CreateSession(ctx, silk.SessionConfig{
		ID: "integrated-engineering-session", Grants: []silk.Grant{},
		Limits: silk.Limits{Fuel: 10000, CallDepth: 16, TimeoutMS: 5000},
	})
	fatalIf(err)
	return runState{
		events: events, memory: memories, governor: governor,
		policyDigest: policyDigest, engine: engine, client: client, sessionID: session.ID,
		forceTaskFailure: forceTaskFailure,
	}
}

func runExperiment(ctx context.Context, state runState) (experimentReport, error) {
	initialRouting := state.engine.Snapshot().Routing
	setup, err := prepareExperiment(ctx, state)
	if err != nil {
		return experimentReport{}, err
	}
	failed, err := exerciseRejectedCandidate(ctx, state, setup.Pattern)
	if err != nil {
		return experimentReport{}, err
	}
	var procedure *procedureResult
	var change *development.Change
	var taskFailure *failedPath
	acquired, acquisitionErr := acquireAndRun(ctx, state, setup.Pattern, setup.Replay, setup.Selected)
	if acquisitionErr != nil {
		if acquired.ActionEventID != "" {
			procedure = &acquired
		}
		failure, recordErr := recordTaskFailure(ctx, state.events, "retained procedure execution", acquisitionErr,
			append(append([]string(nil), setup.Pattern.SourceEventIDs...), setup.Replay.EventID, setup.Selected.EventID, acquired.RetentionEvent, acquired.ActionEventID))
		if recordErr != nil {
			return experimentReport{}, recordErr
		}
		taskFailure = &failure
	} else {
		procedure = &acquired
		developed, developmentErr := developRouting(ctx, state, setup.Selected, acquired.ActionEventID, setup.Replay.EventID)
		if developmentErr != nil {
			failure, recordErr := recordTaskFailure(ctx, state.events, "governed developmental update", developmentErr, []string{acquired.ActionEventID})
			if recordErr != nil {
				return experimentReport{}, recordErr
			}
			taskFailure = &failure
		} else {
			change = &developed
		}
	}
	return assembleReport(ctx, state, setup, experimentOutcome{
		InitialRouting: initialRouting, FailedPath: failed, TaskFailure: taskFailure,
		Procedure: procedure, Development: change,
	})
}

func prepareExperiment(ctx context.Context, state runState) (experimentSetup, error) {
	perception, err := state.events.Emit(ctx, cognition.Draft{
		CorrelationID: "estimate-current", Kind: cognition.KindPerception,
		Payload: json.RawMessage(`{"task":"estimate total replacement cost","parts":[12.5,4.5],"unit":"credits"}`),
	})
	if err != nil {
		return experimentSetup{}, err
	}
	episodes, err := recordPastExamples(ctx, state.memory, state.events)
	if err != nil {
		return experimentSetup{}, err
	}
	patternRun, patterns, err := state.memory.Consolidate(ctx, memory.ConsolidationPolicy{
		MinimumSupport: 2, EpisodeQuery: memory.EpisodeQuery{Limit: 8},
	})
	if err != nil {
		return experimentSetup{}, err
	}
	if len(patterns) != 1 {
		return experimentSetup{}, fmt.Errorf("expected one repeated procedural pattern, received %d", len(patterns))
	}
	pattern := patterns[0]
	inspector, err := inspection.New(state.events)
	if err != nil {
		return experimentSetup{}, err
	}
	initial, err := inspector.Inspect(ctx, organismID)
	if err != nil {
		return experimentSetup{}, err
	}
	replay, err := state.memory.ScheduleReplay(ctx, memory.ReplayPlan{
		InteractionID: "estimate-current", RequestedBy: "coordinator",
		ScheduledAt: time.Now().UTC(), EpisodeIDs: episodes,
	})
	if err != nil {
		return experimentSetup{}, err
	}
	replayRun, err := state.memory.RunReplay(ctx, replay)
	if err != nil {
		return experimentSetup{}, err
	}
	proposals, workspaceReport, regulationSnapshot, err := coordinateSpecialists(ctx, state, perception.EventID)
	if err != nil {
		return experimentSetup{}, err
	}
	selected, err := selectedProposal(workspaceReport, proposals)
	if err != nil {
		return experimentSetup{}, err
	}
	return experimentSetup{
		PatternRun: patternRun, Pattern: pattern,
		Inspector: inspector, Initial: initial, Replay: replayRun, Selected: selected,
		Workspace: workspaceReport, Regulation: regulationSnapshot,
	}, nil
}

func assembleReport(ctx context.Context, state runState, setup experimentSetup, outcome experimentOutcome) (experimentReport, error) {
	history, err := setup.Inspector.Inspect(ctx, organismID)
	if err != nil {
		return experimentReport{}, err
	}
	evolution, err := inspection.ChangesSince(setup.Initial, history)
	if err != nil {
		return experimentReport{}, err
	}
	events, err := readEvents(ctx, state.events)
	if err != nil {
		return experimentReport{}, err
	}
	return experimentReport{
		OrganismID: organismID, Task: "estimate total replacement cost in credits",
		TaskOutcome: taskOutcome(outcome.TaskFailure), TaskFailure: outcome.TaskFailure,
		InitialRouting: outcome.InitialRouting, FinalRouting: state.engine.Snapshot().Routing,
		SelectedProposal: setup.Selected, Workspace: setup.Workspace, Regulation: setup.Regulation,
		Consolidation: setup.PatternRun, Replay: setup.Replay, FailedPath: outcome.FailedPath, AcquiredProcedure: outcome.Procedure,
		Development: outcome.Development, Evolution: evolution, InspectedHistory: history, Events: events,
	}, nil
}

func taskOutcome(failure *failedPath) string {
	if failure != nil {
		return "failed_with_captured_history"
	}
	return "completed_with_captured_failure_path"
}

func recordPastExamples(ctx context.Context, memories *memory.Service, events *cognition.Spine) ([]string, error) {
	episodeIDs := []string{"prior-estimate-a", "prior-estimate-b"}
	for _, id := range episodeIDs {
		source, err := events.Emit(ctx, cognition.Draft{
			CorrelationID: id, Kind: cognition.KindPerception,
			Payload: json.RawMessage(`{"task":"estimate replacement cost","unit":"credits"}`),
		})
		if err != nil {
			return nil, err
		}
		_, err = memories.RecordEpisode(ctx, memory.Episode{
			ID: id, Source: memory.SourceRef{Kind: "completed-estimate", ID: id},
			SourceEventIDs: []string{source.EventID}, OccurredAt: time.Now().UTC(),
			Kind: "successful-sum", Content: json.RawMessage(`{"operation":"sum two measured costs","inputs":[2,3],"output":5,"unit":"credits"}`),
		})
		if err != nil {
			return nil, err
		}
	}
	return episodeIDs, nil
}

func coordinateSpecialists(ctx context.Context, state runState, sourceID string) ([]agent.Proposal, workspace.Report, regulation.Snapshot, error) {
	coordination := make(chan coordinationResult, 1)
	supervisor, err := agent.NewSupervisor(8)
	if err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, err
	}
	registerSpecialist(supervisor, state.events, "planner", 0.85, "sum the measured costs after checking units", sourceID)
	registerSpecialist(supervisor, state.events, "critic", 0.65, "verify both measurements use credits", sourceID)
	config := workspace.DefaultConfig()
	config.Capacity = 2
	proposalWorkspace, err := workspace.New(config, state.events)
	if err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, err
	}
	regulator, err := regulation.New(regulation.DefaultPolicy(), state.events)
	if err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, err
	}
	coordinatorAgent := agent.Func(func(ctx context.Context, inbox <-chan agent.Message, sender agent.Sender) error {
		result := coordinateInteraction(ctx, inbox, sender, coordinationInput{
			workspace: proposalWorkspace, regulation: regulator, sourceID: sourceID,
		})
		coordination <- result
		return result.err
	})
	if err := supervisor.Register("coordinator", coordinatorAgent); err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, err
	}
	if err := supervisor.Start(ctx); err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, err
	}
	var result coordinationResult
	select {
	case result = <-coordination:
	case <-time.After(10 * time.Second):
		stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = supervisor.Stop(stopCtx)
		return nil, workspace.Report{}, regulation.Snapshot{}, fmt.Errorf("specialist coordination timed out")
	}
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := supervisor.Stop(stopCtx); err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, err
	}
	if result.err != nil {
		return nil, workspace.Report{}, regulation.Snapshot{}, result.err
	}
	return result.proposals, result.workspace, result.regulation, nil
}

func coordinateInteraction(ctx context.Context, inbox <-chan agent.Message, sender agent.Sender, input coordinationInput) coordinationResult {
	if err := input.workspace.Open(ctx, workspace.Request{
		ID: "estimate-workspace", InteractionID: "estimate-current", SessionID: "integrated-engineering-session",
		SourceEventIDs: []string{input.sourceID},
	}); err != nil {
		return coordinationResult{err: err}
	}
	activation, err := json.Marshal(agent.Activation{
		InteractionID: "estimate-current", SessionID: "integrated-engineering-session",
		Input: json.RawMessage(`{"parts":[12.5,4.5],"unit":"credits"}`), SourceEventIDs: []string{input.sourceID},
	})
	if err != nil {
		return coordinationResult{err: err}
	}
	for _, specialist := range []string{"planner", "critic"} {
		if err := sender.Send(ctx, specialist, agent.SpecialistActivateMessage, activation); err != nil {
			return coordinationResult{err: err}
		}
	}
	proposals, err := collectProposals(ctx, inbox, input.workspace)
	if err != nil {
		return coordinationResult{err: err}
	}
	snapshot, err := input.regulation.Evaluate(ctx, regulation.Input{
		InteractionID: "estimate-current", SessionID: "integrated-engineering-session",
		SourceEventIDs: []string{input.sourceID},
		Expected:       json.RawMessage(`{"measurements":"certain"}`), Observed: json.RawMessage(`{"measurements":"not_yet_verified"}`),
		DeclaredSalience: 0.9, ActiveSpecialists: 2, SpecialistCapacity: 2,
		PendingActions: 2, ActionCapacity: 2, WorkspaceCapacity: 2,
	})
	if err != nil {
		return coordinationResult{err: err}
	}
	selection, err := input.workspace.SelectWithPolicy(ctx, "estimate-workspace", snapshot.WorkspacePolicy)
	if err != nil {
		return coordinationResult{err: err}
	}
	return coordinationResult{proposals: proposals, workspace: selection, regulation: snapshot}
}

func collectProposals(ctx context.Context, inbox <-chan agent.Message, proposalWorkspace *workspace.Workspace) ([]agent.Proposal, error) {
	proposals := make([]agent.Proposal, 0, 2)
	for len(proposals) < 2 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case message := <-inbox:
			if message.Kind != agent.SpecialistProposalMessage {
				continue
			}
			var proposal agent.Proposal
			if err := json.Unmarshal(message.Payload, &proposal); err != nil {
				return nil, err
			}
			if err := proposalWorkspace.Submit(ctx, "estimate-workspace", proposal); err != nil {
				return nil, err
			}
			proposals = append(proposals, proposal)
		}
	}
	return proposals, nil
}

func registerSpecialist(supervisor *agent.Supervisor, events *cognition.Spine, id string, confidence float64, summary, sourceID string) {
	implementation := agent.SpecialistFunc(func(_ context.Context, _ agent.Activation) (agent.Proposal, error) {
		return agent.Proposal{
			Summary: summary, Confidence: confidence, EvidenceEventIDs: []string{sourceID},
			RequestedActions: []agent.RequestedAction{{Kind: "estimate_review", Payload: json.RawMessage(`{"executable":false}`)}},
		}, nil
	})
	specialist, err := agent.NewSpecialistAgent(id, implementation, events)
	fatalIf(err)
	fatalIf(supervisor.Register(id, specialist))
}

func selectedProposal(report workspace.Report, proposals []agent.Proposal) (agent.Proposal, error) {
	selectedID := ""
	for _, entry := range report.Selection.Entries {
		if entry.Selected {
			selectedID = entry.ProposalID
			break
		}
	}
	for _, proposal := range proposals {
		if proposal.ID == selectedID {
			return proposal, nil
		}
	}
	return agent.Proposal{}, fmt.Errorf("workspace selected no known specialist proposal")
}

func exerciseRejectedCandidate(ctx context.Context, state runState, pattern memory.ConsolidatedPattern) (failedPath, error) {
	request, err := candidateMetadata(pattern)
	if err != nil {
		return failedPath{}, err
	}
	_, rejected := state.client.PrepareCandidate(ctx, state.sessionID, "fn main( {", request)
	if rejected == nil {
		return failedPath{}, fmt.Errorf("malformed source unexpectedly passed candidate preparation")
	}
	payload, err := json.Marshal(map[string]string{
		"operation": "silk_candidate_rejected", "reason": rejected.Error(), "pattern_id": pattern.ID,
	})
	if err != nil {
		return failedPath{}, err
	}
	event, err := state.events.Emit(ctx, cognition.Draft{
		CorrelationID: "estimate-current", ParentEventIDs: pattern.SourceEventIDs,
		Kind: cognition.KindDevelopment, Payload: payload,
	})
	if err != nil {
		return failedPath{}, err
	}
	return failedPath{Stage: "Silk candidate preparation", Reason: rejected.Error(), Event: event.EventID}, nil
}

func acquireAndRun(ctx context.Context, state runState, pattern memory.ConsolidatedPattern, replay memory.ReplayRun, selected agent.Proposal) (procedureResult, error) {
	retained, err := prepareAndRetain(ctx, state, pattern, replay, selected)
	if err != nil {
		return procedureResult{}, err
	}
	return executeRetained(ctx, state, retained, selected)
}

func prepareAndRetain(ctx context.Context, state runState, pattern memory.ConsolidatedPattern, replay memory.ReplayRun, selected agent.Proposal) (retainedProcedure, error) {
	request, err := candidateMetadata(pattern)
	if err != nil {
		return retainedProcedure{}, err
	}
	source := "fn main(left, right) { return left + right; }"
	if state.forceTaskFailure {
		source = "fn main(left, right) { return left - right; }"
	}
	prepared, err := state.client.PrepareCandidate(ctx, state.sessionID, source, request)
	if err != nil {
		return retainedProcedure{}, err
	}
	var identity procedureIdentity
	if err := json.Unmarshal(prepared.Artifact, &identity); err != nil {
		return retainedProcedure{}, err
	}
	proposal := governance.Proposal{
		ID: "retain-estimate-sum", InteractionID: "estimate-current", ProposerID: selected.SpecialistID,
		Class: governance.ActionProcedureRetention, Target: "silk:procedure:" + identity.ProcedureID + "@" + identity.RevisionDigest,
		Action:    "retain validated estimate summation procedure",
		Payload:   json.RawMessage(fmt.Sprintf(`{"pattern_id":%q,"revision_digest":%q,"entry_procedure":%q}`, pattern.ID, identity.RevisionDigest, prepared.EntryProcedure)),
		CreatedAt: time.Now().UTC(), SourceEventIDs: pattern.SourceEventIDs,
		EvidenceEventIDs: append(append([]string(nil), pattern.SourceEventIDs...), replay.EventID, selected.EventID),
	}
	decision, err := approve(ctx, state, proposal, "approve retained sum procedure")
	if err != nil {
		return retainedProcedure{}, err
	}
	admission, err := state.client.AdmitCandidate(ctx, prepared)
	if err != nil {
		return retainedProcedure{}, err
	}
	if admission.EntryProcedure != prepared.EntryProcedure {
		return retainedProcedure{}, fmt.Errorf("admitted entry %q differs from approved entry %q", admission.EntryProcedure, prepared.EntryProcedure)
	}
	admissionEvent, err := emitBoundaryEvent(ctx, state.events, decision.EventID, "silk_registry_admitted", admission)
	if err != nil {
		return retainedProcedure{}, err
	}
	retention := retentionEvidence(decision, state.policyDigest, identity.RevisionDigest)
	retentionJSON, err := json.Marshal(retention)
	if err != nil {
		return retainedProcedure{}, err
	}
	if err := state.client.RetainCandidate(ctx, identity.ProcedureID, identity.RevisionDigest, retentionJSON); err != nil {
		return retainedProcedure{}, err
	}
	retained, err := emitBoundaryEvent(ctx, state.events, admissionEvent.EventID, "silk_registry_retained", map[string]string{
		"procedure_id": identity.ProcedureID, "revision_digest": identity.RevisionDigest,
		"governance_decision_id": decision.ID,
	})
	if err != nil {
		return retainedProcedure{}, err
	}
	return retainedProcedure{candidate: prepared, identity: identity, eventID: retained.EventID}, nil
}

func executeRetained(ctx context.Context, state runState, retained retainedProcedure, selected agent.Proposal) (procedureResult, error) {
	identity := retained.identity
	result := procedureResult{
		ProcedureID: identity.ProcedureID, RevisionDigest: identity.RevisionDigest,
		EntryProcedure: retained.candidate.EntryProcedure, RetentionEvent: retained.eventID,
	}
	call, err := state.client.Run(ctx, silk.ProcedureCall{
		SessionID: state.sessionID, Procedure: retained.candidate.EntryProcedure,
		Arguments: []json.RawMessage{json.RawMessage("12.5"), json.RawMessage("4.5")},
		Retained:  &silk.RegisteredRevision{ProcedureID: identity.ProcedureID, RevisionDigest: identity.RevisionDigest},
	})
	if err != nil {
		return result, err
	}
	parents := []string{retained.eventID}
	for _, trace := range call.Trace {
		traceEvent, err := state.events.RecordSilkTrace(ctx, selected.SpecialistID, state.sessionID,
			"estimate-current", parents, trace)
		if err != nil {
			return result, err
		}
		parents = append(parents, traceEvent.EventID)
	}
	action, err := state.events.Emit(ctx, cognition.Draft{
		AgentID: selected.SpecialistID, SessionID: state.sessionID, CorrelationID: "estimate-current",
		ParentEventIDs: parents, Kind: cognition.KindAction,
		Payload: json.RawMessage(fmt.Sprintf(`{"procedure_id":%q,"revision_digest":%q,"output":%s}`, identity.ProcedureID, identity.RevisionDigest, call.Value)),
	})
	if err != nil {
		return result, err
	}
	result.Output = call.Value
	result.ActionEventID = action.EventID
	var total float64
	if err := json.Unmarshal(call.Value, &total); err != nil {
		return result, fmt.Errorf("decode procedure output %s: %w", call.Value, err)
	}
	if total != 17 {
		return result, fmt.Errorf("procedure returned %s; expected 17", call.Value)
	}
	return result, nil
}

func developRouting(ctx context.Context, state runState, selected agent.Proposal, actionEventID, replayEventID string) (development.Change, error) {
	proposal, err := state.engine.PrepareRoutingProposal(development.RoutingRequest{
		ID: "learn-estimate-route", InteractionID: "estimate-current", ProposerID: selected.SpecialistID,
		Key: "engineering:estimate", Value: 0.9, CreatedAt: time.Now().UTC(),
		SourceEventIDs: []string{selected.EventID, replayEventID}, EvidenceEventIDs: []string{actionEventID},
	})
	if err != nil {
		return development.Change{}, err
	}
	approval, err := newApproval(ctx, state, proposal, "approve experience-backed route update")
	if err != nil {
		return development.Change{}, err
	}
	change, decision, err := state.engine.ApplyRoutingChange(ctx, proposal, []governance.Approval{approval})
	if err != nil {
		return development.Change{}, err
	}
	if decision.Outcome != governance.OutcomeApproved {
		return development.Change{}, fmt.Errorf("routing update decision was %s", decision.Outcome)
	}
	return change, nil
}

func approve(ctx context.Context, state runState, proposal governance.Proposal, reason string) (governance.Decision, error) {
	approval, err := newApproval(ctx, state, proposal, reason)
	if err != nil {
		return governance.Decision{}, err
	}
	return state.governor.Evaluate(ctx, proposal, []governance.Approval{approval})
}

func newApproval(ctx context.Context, state runState, proposal governance.Proposal, reason string) (governance.Approval, error) {
	proposalDigest, err := governance.ProposalDigest(proposal)
	if err != nil {
		return governance.Approval{}, err
	}
	review, err := state.events.Emit(ctx, cognition.Draft{
		AgentID: "operator", CorrelationID: proposal.InteractionID,
		ParentEventIDs: append(append([]string(nil), proposal.SourceEventIDs...), proposal.EvidenceEventIDs...),
		Kind:           cognition.KindDecision, Payload: json.RawMessage(fmt.Sprintf(`{"decision":%q}`, reason)),
	})
	if err != nil {
		return governance.Approval{}, err
	}
	return governance.Approval{
		ID: "operator-" + proposal.ID, ApproverID: "operator", ProposalDigest: proposalDigest,
		PolicyDigest: state.policyDigest, Decision: governance.ApprovalApprove,
		Reason: reason, DecidedAt: time.Now().UTC(), SourceEventID: review.EventID,
	}, nil
}

func candidateMetadata(pattern memory.ConsolidatedPattern) (json.RawMessage, error) {
	metadata := map[string]any{
		"procedure_id": "estimate.sum", "name": "estimate-sum",
		"semantic_description": map[string]any{"summary": "Sum two measured costs in the same unit.", "terms": []string{"estimate", "sum", "total"}},
		"contract": map[string]any{
			"purpose":        "Return the total of two numeric costs.",
			"input_schema":   map[string]any{"type": "array", "items": map[string]string{"type": "number"}, "minItems": 2, "maxItems": 2},
			"output_schema":  map[string]string{"type": "number"},
			"effect_ceiling": []string{}, "required_authorities": []string{}, "dependencies": []string{},
		},
		"provenance": map[string]any{
			"origin": "generated", "source_reference": "memory-pattern:" + pattern.ID,
			"generator": "arachne.integrated-engineering.v1", "claims": map[string]string{"pattern_id": pattern.ID},
		},
		"entry_procedure":              "main",
		"required_validation_profiles": []string{"silk.syntax_lowering.v1", "silk.effects_authority.v1"},
	}
	encoded, err := json.Marshal(metadata)
	return encoded, err
}

func retentionEvidence(decision governance.Decision, policyDigest, revisionDigest string) map[string]any {
	decisionJSON, _ := json.Marshal(map[string]string{
		"decision_id": decision.ID, "decision_event_id": decision.EventID,
		"proposal_digest": decision.ProposalDigest, "policy_digest": policyDigest,
	})
	digest := sha256.Sum256(decisionJSON)
	return map[string]any{
		"actor": "operator", "reason": "Arachne governance decision " + decision.ID,
		"evidence": map[string]any{
			"validator": "arachne.governance", "validator_version": "1",
			"profile": "silk.retention_approval.v1", "subject_revision_digest": revisionDigest,
			"outcome": "passed", "evidence_digest": "sha256:" + hex.EncodeToString(digest[:]),
		},
	}
}

func emitBoundaryEvent(ctx context.Context, events *cognition.Spine, parent, operation string, result any) (cognition.Event, error) {
	payload, err := json.Marshal(map[string]any{"operation": operation, "result": result})
	if err != nil {
		return cognition.Event{}, err
	}
	return events.Emit(ctx, cognition.Draft{
		CorrelationID: "estimate-current", ParentEventIDs: []string{parent},
		Kind: cognition.KindDevelopment, Payload: payload,
	})
}

func recordTaskFailure(ctx context.Context, events *cognition.Spine, stage string, cause error, parents []string) (failedPath, error) {
	encoded, err := json.Marshal(map[string]string{
		"operation": "integrated_task_failed", "stage": stage, "reason": cause.Error(),
	})
	if err != nil {
		return failedPath{}, err
	}
	event, err := events.Emit(ctx, cognition.Draft{
		CorrelationID: "estimate-current", ParentEventIDs: parents,
		Kind: cognition.KindDevelopment, Payload: encoded,
	})
	if err != nil {
		return failedPath{}, err
	}
	return failedPath{Stage: stage, Reason: cause.Error(), Event: event.EventID}, nil
}

func readEvents(ctx context.Context, events *cognition.Spine) ([]cognition.Event, error) {
	result := make([]cognition.Event, 0)
	var after uint64
	for {
		batch, err := events.Read(ctx, after, 256)
		if err != nil {
			return nil, err
		}
		result = append(result, batch...)
		if len(batch) == 0 || len(batch) < 256 {
			return result, nil
		}
		after = batch[len(batch)-1].Sequence
	}
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
