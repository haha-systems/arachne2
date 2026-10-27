package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/experiment"
	"github.com/haha-systems/arachne2/internal/workspace"
)

const (
	taskName = "xor-evidence-reconstruction"
	solverID = "solver"
	runID    = "workspace-run"
)

type config struct {
	Experiment experiment.Spec `json:"experiment"`
	Controls   controls        `json:"controls"`
}

type controls struct {
	ModelProvider     string   `json:"model_provider"`
	Model             string   `json:"model"`
	Tools             []string `json:"tools"`
	StepBudget        int      `json:"step_budget"`
	TimeLimitMS       int      `json:"time_limit_ms"`
	Memory            string   `json:"memory"`
	Replay            string   `json:"replay"`
	Learning          string   `json:"learning"`
	ActiveInference   string   `json:"active_inference"`
	Governance        string   `json:"governance"`
	InitialState      string   `json:"initial_state"`
	WorkspaceCapacity int      `json:"workspace_capacity"`
	Agents            []string `json:"agents"`
}

type generatedTask struct {
	prompt     string
	pieces     [4]uint64
	answer     uint64
	confidence [5]float64
}

type trialOutput struct {
	TaskInput         string  `json:"task_input"`
	ExpectedAnswer    uint64  `json:"expected_answer"`
	FinalAnswer       *uint64 `json:"final_answer"`
	Success           bool    `json:"success"`
	TerminationReason string  `json:"termination_reason"`
}

func generateTask(seed int64) generatedTask {
	// #nosec G404 -- seeded reproducibility is required for this synthetic task.
	rng := rand.New(rand.NewSource(seed))
	task := generatedTask{prompt: "XOR the four private 64-bit evidence values; ignore the decoy proposal."}
	for index := range task.pieces {
		task.pieces[index] = rng.Uint64()
		task.answer ^= task.pieces[index]
		task.confidence[index] = float64(rng.Intn(1000)) / 1000
	}
	task.confidence[4] = float64(rng.Intn(1000)) / 1000
	return task
}

func loadConfig(path string) (config, error) {
	// #nosec G304 -- the experiment operator selects a local JSON config path.
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var cfg config
	err = json.NewDecoder(bytes.NewReader(data)).Decode(&cfg)
	return cfg, err
}

func validateConfig(cfg config) error {
	if cfg.Experiment.Name != experimentName || cfg.Experiment.Task != taskName {
		return errors.New("experiment/task name mismatch")
	}
	if err := experiment.Validate(cfg.Experiment); err != nil {
		return err
	}
	if cfg.Controls.WorkspaceCapacity != 4 || cfg.Controls.StepBudget != 1 || len(cfg.Controls.Agents) != 5 {
		return errors.New("task requires five agents, one step, and workspace capacity four")
	}
	if cfg.Controls.TimeLimitMS < 1 {
		return errors.New("time_limit_ms must be positive")
	}
	if len(cfg.Experiment.Profiles) != 2 {
		return errors.New("expected paired workspace profiles")
	}
	enabled, disabled := 0, 0
	for _, profile := range cfg.Experiment.Profiles {
		if len(profile.Subsystems) != 1 {
			return errors.New("profiles may vary only shared_workspace")
		}
		value, ok := profile.Subsystems["shared_workspace"]
		if !ok {
			return errors.New("shared_workspace control missing")
		}
		if value {
			if profile.Name != "workspace_enabled" {
				return errors.New("enabled profile must be named workspace_enabled")
			}
			enabled++
		} else {
			if profile.Name != "workspace_disabled" {
				return errors.New("disabled profile must be named workspace_disabled")
			}
			disabled++
		}
	}
	if enabled != 1 || disabled != 1 {
		return errors.New("expected one enabled and one disabled profile")
	}
	return nil
}

func trialExecutor(c controls) experiment.Execute {
	return func(ctx context.Context, profile experiment.Profile, seed int64) (experiment.Result, func() error, error) {
		return runTrial(ctx, c, profile, seed)
	}
}

func runTrial(parent context.Context, c controls, profile experiment.Profile, seed int64) (experiment.Result, func() error, error) {
	startedAt := time.Now()
	task := generateTask(seed)
	enabled := profile.Subsystems["shared_workspace"]
	ctx, cancel := context.WithTimeout(parent, time.Duration(c.TimeLimitMS)*time.Millisecond)
	defer cancel()

	store, err := cognition.NewMemoryStore(512)
	if err != nil {
		return experiment.Result{}, nil, err
	}
	events, err := cognition.NewSpine(fmt.Sprintf("exp001-%d-%s", seed, profile.Name), store)
	if err != nil {
		return experiment.Result{}, nil, err
	}
	inputPayload, _ := json.Marshal(map[string]any{"task": taskName, "prompt": task.prompt, "seed": seed})
	stimulus, err := events.Emit(ctx, cognition.Draft{Kind: cognition.KindPerception, Payload: inputPayload})
	if err != nil {
		return experiment.Result{}, nil, err
	}

	var shared *workspace.Workspace
	if enabled {
		workspaceConfig := workspace.DefaultConfig()
		workspaceConfig.Capacity = c.WorkspaceCapacity
		shared, err = workspace.New(workspaceConfig, events)
		if err != nil {
			return experiment.Result{}, nil, err
		}
		interactionID := fmt.Sprintf("seed-%d", seed)
		err = shared.Open(ctx, workspace.Request{
			ID: runID, InteractionID: interactionID, SessionID: interactionID,
			SourceEventIDs: []string{stimulus.EventID},
		})
		if err != nil {
			return experiment.Result{}, nil, err
		}
	}

	supervisor, err := agent.NewSupervisor(8)
	if err != nil {
		return experiment.Result{}, nil, err
	}
	turns := make([]chan struct{}, len(c.Agents))
	completed := make([]chan struct{}, len(c.Agents))
	for index, agentID := range c.Agents {
		turns[index] = make(chan struct{})
		completed[index] = make(chan struct{})
		index, agentID := index, agentID
		err := supervisor.Register(agentID, agent.Func(func(agentCtx context.Context, _ <-chan agent.Message, _ agent.Sender) error {
			select {
			case <-agentCtx.Done():
				return nil
			case <-turns[index]:
			}
			defer close(completed[index])
			if shared == nil {
				return nil
			}
			return publishEvidence(agentCtx, events, shared, task, stimulus.EventID, seed, index, agentID)
		}))
		if err != nil {
			return experiment.Result{}, nil, err
		}
	}

	solverResult := make(chan *uint64, 1)
	var solverSteps int
	var solverMu sync.Mutex
	err = supervisor.Register(solverID, agent.Func(func(agentCtx context.Context, inbox <-chan agent.Message, _ agent.Sender) error {
		select {
		case <-agentCtx.Done():
			solverResult <- nil
			return nil
		case message := <-inbox:
			if message.Kind != workspace.BroadcastMessage {
				return fmt.Errorf("unexpected solver message %q", message.Kind)
			}
			solverMu.Lock()
			solverSteps++
			solverMu.Unlock()
			answer, valid, err := solveBroadcast(message.Payload)
			if err != nil {
				return err
			}
			if !valid {
				solverResult <- nil
			} else {
				solverResult <- &answer
			}
			return nil
		}
	}))
	if err != nil {
		return experiment.Result{}, nil, err
	}

	coordinationDone := make(chan error, 1)
	err = supervisor.Register("workspace-coordinator", agent.Func(func(agentCtx context.Context, _ <-chan agent.Message, sender agent.Sender) error {
		for _, done := range completed {
			select {
			case <-agentCtx.Done():
				coordinationDone <- agentCtx.Err()
				return nil
			case <-done:
			}
		}
		if shared == nil {
			coordinationDone <- nil
			return nil
		}
		_, coordinationErr := shared.SelectWithPolicy(agentCtx, runID, workspace.SelectionPolicy{
			Capacity: c.WorkspaceCapacity, RequireEvidence: true,
		})
		if coordinationErr == nil {
			_, coordinationErr = shared.Broadcast(agentCtx, runID, "deliver selected evidence candidates to solver", []string{solverID}, sender)
		}
		coordinationDone <- coordinationErr
		return nil
	}))
	if err != nil {
		return experiment.Result{}, nil, err
	}
	if err := supervisor.Start(ctx); err != nil {
		return experiment.Result{}, nil, err
	}
publishLoop:
	for index, turn := range turns {
		select {
		case turn <- struct{}{}:
		case <-ctx.Done():
			break publishLoop
		}
		select {
		case <-completed[index]:
		case <-ctx.Done():
			break publishLoop
		}
	}
	var coordinationErr error
	select {
	case coordinationErr = <-coordinationDone:
	case <-ctx.Done():
		coordinationErr = ctx.Err()
	}
	if coordinationErr != nil && !errors.Is(coordinationErr, context.DeadlineExceeded) {
		_ = stopSupervisor(supervisor)
		return experiment.Result{}, nil, coordinationErr
	}

	var answer *uint64
	if enabled {
		select {
		case answer = <-solverResult:
		case <-ctx.Done():
		}
	}
	if err := stopSupervisor(supervisor); err != nil {
		return experiment.Result{}, nil, err
	}

	allEvents, err := events.Read(context.Background(), 0, 512)
	if err != nil {
		return experiment.Result{}, nil, err
	}
	publications, broadcasts, messages := countWorkspaceEvents(allEvents)
	workspaceReads := 0
	var workspaceReport workspace.Report
	if shared != nil {
		workspaceReport, err = shared.Inspect(context.Background(), runID)
		if err != nil {
			return experiment.Result{}, nil, err
		}
		workspaceReads = 2
	}
	success := answer != nil && *answer == task.answer
	reason := "completed"
	if answer == nil && !enabled {
		reason = "no_shared_evidence"
	} else if answer == nil {
		reason = "solver_received_incomplete_evidence"
	}
	out := trialOutput{
		TaskInput: task.prompt, ExpectedAnswer: task.answer, FinalAnswer: answer,
		Success: success, TerminationReason: reason,
	}
	measurements := map[string]any{
		"agent_steps": len(c.Agents) + solverSteps, "messages": messages,
		"event_count": len(allEvents), "runtime_ms_precise": float64(time.Since(startedAt)) / float64(time.Millisecond), "workspace_publications": publications,
		"workspace_broadcasts": broadcasts, "workspace_reads": workspaceReads,
		"tokens_used": 0, "token_usage_available": false,
		"termination_reason": reason, "errors": []string{},
		"model_provider": c.ModelProvider, "model": c.Model, "agent_definitions": c.Agents,
		"prompts": map[string]string{"task": task.prompt, "workspace_broadcast": "deliver selected evidence candidates to solver"}, "tool_access": c.Tools,
		"step_budget": c.StepBudget, "time_limit_ms": c.TimeLimitMS,
		"memory": c.Memory, "replay": c.Replay, "learning": c.Learning,
		"active_inference": c.ActiveInference, "governance": c.Governance,
		"initial_state": c.InitialState,
	}
	if shared != nil {
		measurements["workspace_report"] = workspaceReport
	}
	score := 0.0
	if success {
		score = 1
	}
	return experiment.Result{
		Output: out, Events: allEvents, Scores: map[string]float64{"success": score},
		Measurements: measurements,
	}, nil, nil
}

func publishEvidence(ctx context.Context, events *cognition.Spine, shared *workspace.Workspace, task generatedTask, sourceEventID string, seed int64, index int, agentID string) error {
	evidence := map[string]any{"shard_index": index}
	if index < len(task.pieces) {
		evidence["value"] = task.pieces[index]
	} else {
		evidence["decoy"] = true
	}
	summary, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"agent_id": agentID, "summary": json.RawMessage(summary), "seed": seed,
	})
	if err != nil {
		return err
	}
	interactionID := fmt.Sprintf("seed-%d", seed)
	event, err := events.Emit(ctx, cognition.Draft{
		AgentID: agentID, SessionID: interactionID, CorrelationID: interactionID,
		ParentEventIDs: []string{sourceEventID}, Kind: cognition.KindProposal, Payload: payload,
	})
	if err != nil {
		return err
	}
	return shared.Submit(ctx, runID, agent.Proposal{
		ID: fmt.Sprintf("proposal-%d", index), EventID: event.EventID,
		InteractionID: interactionID, SpecialistID: agentID, CorrelationID: interactionID,
		Status: "candidate", Summary: string(summary), Confidence: task.confidence[index],
		EvidenceEventIDs: []string{event.EventID}, CreatedAt: event.OccurredAt,
	})
}

func solveBroadcast(payload []byte) (uint64, bool, error) {
	var broadcast struct {
		Proposals []agent.Proposal `json:"proposals"`
	}
	if err := json.Unmarshal(payload, &broadcast); err != nil {
		return 0, false, err
	}
	var answer uint64
	seen := make(map[int]bool, 4)
	for _, proposal := range broadcast.Proposals {
		var evidence struct {
			Index int    `json:"shard_index"`
			Value uint64 `json:"value"`
		}
		if err := json.Unmarshal([]byte(proposal.Summary), &evidence); err != nil {
			return 0, false, err
		}
		if evidence.Index >= 0 && evidence.Index < 4 {
			answer ^= evidence.Value
			seen[evidence.Index] = true
		}
	}
	for index := 0; index < 4; index++ {
		if !seen[index] {
			return 0, false, nil
		}
	}
	return answer, true, nil
}

func countWorkspaceEvents(events []cognition.Event) (publications, broadcasts, messages int) {
	for _, event := range events {
		var payload map[string]any
		if json.Unmarshal(event.Payload, &payload) != nil {
			continue
		}
		switch payload["operation"] {
		case "proposal_received":
			publications++
		case "broadcast_planned":
			broadcasts++
		case "broadcast_delivered":
			if payload["sent"] == true {
				messages++
			}
		}
	}
	return publications, broadcasts, messages
}

func stopSupervisor(supervisor *agent.Supervisor) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return supervisor.Stop(ctx)
}
