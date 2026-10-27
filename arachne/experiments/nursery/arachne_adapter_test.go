package nursery

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/memory"
	"github.com/haha-systems/arachne2/internal/organism"
	"github.com/haha-systems/arachne2/internal/regulation"
	"github.com/haha-systems/arachne2/internal/workspace"
)

type nurseryRuntimeGate struct{}

func (nurseryRuntimeGate) Evaluate(context.Context, organism.Commitment) (organism.EligibilityResult, error) {
	return organism.EligibilityResult{Eligible: true, Reason: "deterministic test gate"}, nil
}

func newNurseryRuntime(t *testing.T, id string) *organism.Runtime {
	t.Helper()
	ctx := context.Background()
	eventStore, err := cognition.NewMemoryStore(4096)
	if err != nil {
		t.Fatal(err)
	}
	events, err := cognition.NewSpine(id, eventStore)
	if err != nil {
		t.Fatal(err)
	}
	memoryStore, err := memory.NewMemoryStore(id)
	if err != nil {
		t.Fatal(err)
	}
	memories, err := memory.NewService(id, memoryStore, events)
	if err != nil {
		t.Fatal(err)
	}
	regulator, err := regulation.New(regulation.DefaultPolicy(), events)
	if err != nil {
		t.Fatal(err)
	}
	specialist, err := agent.NewLearnableSpecialist(agent.LearnableSpecialistConfig{
		ID: "nursery-fixture", Seed: 29, FeatureDimension: 10,
		Actions: []agent.ActionCapability{{Kind: "wait"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := organism.New(organism.Config{
		OrganismID: id, Events: events,
		Specialists: []organism.RegisteredSpecialist{{ID: "nursery-fixture", Implementation: specialist}},
		Workspace:   workspace.DefaultConfig(),
		Gate:        nurseryRuntimeGate{},
		Memory:      memories,
		Regulator:   regulator,
		Learners:    map[string]organism.OutcomeLearner{"nursery-fixture": specialist},
		Scorer: func(context.Context, organism.StepResult) (map[string]float64, error) {
			return map[string]float64{"wait": 0.5}, nil
		},
		ComponentRestorability: map[string]organism.Restorability{
			"eligibility-gate": organism.RestorabilityStateless,
			"outcome-scorer":   organism.RestorabilityStateless,
		},
		DeferredExecution: true,
		Deterministic:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = ctx
	return runtime
}

func TestCheckpointOmitsOpaqueProviderState(t *testing.T) {
	ctx := context.Background()
	adapter, err := NewArachneOrganism(newNurseryRuntime(t, "provider-secret-test"), "root")
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(ctx, ExperimentConfig{
		Name: "provider-secret-test", World: Config{Seed: 114},
		Provider: ProviderRecord{Configuration: json.RawMessage(`{"model":"fixture","token":"ambient-secret"}`), Deterministic: true},
	}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := session.CreateCheckpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoint.Provider.Configuration) != 0 || len(checkpoint.Provider.State) != 0 ||
		len(checkpoint.Experiment.Provider.Configuration) != 0 || len(checkpoint.Experiment.Provider.State) != 0 {
		t.Fatal("checkpoint retained opaque provider data")
	}
	if checkpoint.Deterministic == nil || *checkpoint.Deterministic {
		t.Fatal("omitted provider state was not marked nondeterministic")
	}
}

func TestArachneNurseryRuntimeCheckpointRestoreSmoke(t *testing.T) {
	ctx := context.Background()
	runtime := newNurseryRuntime(t, "nursery-real-runtime")
	adapter, err := NewArachneOrganism(runtime, "root")
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSession(ctx, ExperimentConfig{
		Name:                    "pr4-runtime-smoke",
		World:                   Config{Seed: 113},
		Provider:                ProviderRecord{Deterministic: true},
		RequireExactRestoration: true,
	}, adapter)
	if err != nil {
		t.Fatal(err)
	}

	record, err := session.Step(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if record.Action.Kind != "wait" || record.Tick != 1 {
		t.Fatalf("Nursery did not apply the committed specialist action: tick=%d action=%+v", record.Tick, record.Action)
	}
	if !record.Cognition.Deterministic || len(record.Cognition.ActionSourceEventIDs) != 1 {
		t.Fatalf("runtime provenance or deterministic status missing: %+v", record.Cognition)
	}
	if len(record.Cognition.Events) != 1 {
		t.Fatalf("expected an event-ID trace for the completed runtime step, got %d", len(record.Cognition.Events))
	}
	var eventIDs organism.EventIDs
	if err := json.Unmarshal(record.Cognition.Events[0], &eventIDs); err != nil {
		t.Fatal(err)
	}
	if eventIDs.Perception == "" || eventIDs.Selection == "" || eventIDs.Commitment == "" || eventIDs.Eligibility == "" || eventIDs.Action == "" || eventIDs.Outcome == "" || eventIDs.Memory == "" {
		t.Fatalf("runtime event lineage is incomplete after the environment consequence: %+v", eventIDs)
	}
	if len(eventIDs.Learning) == 0 {
		t.Fatal("the explicit consequence score did not reach the learnable specialist")
	}
	if len(record.Cognition.Proposals) != 1 || len(record.Cognition.MemoryWrites) != 1 {
		t.Fatalf("real specialist proposal or attributed memory was not surfaced: proposals=%d memory=%v", len(record.Cognition.Proposals), record.Cognition.MemoryWrites)
	}

	for tick := 2; tick <= 8; tick++ {
		if _, err := session.Step(ctx); err != nil {
			t.Fatalf("Nursery smoke failed at tick %d: %v", tick, err)
		}
	}
	checkpoint, err := session.CreateCheckpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Mode != "exact" || !checkpoint.ExactRestoration || !checkpoint.Restorability.Exact {
		t.Fatalf("Nursery did not create an exact checkpoint: mode=%q exact=%v report=%+v", checkpoint.Mode, checkpoint.ExactRestoration, checkpoint.Restorability)
	}
	if checkpoint.Deterministic == nil || !*checkpoint.Deterministic {
		t.Fatalf("checkpoint did not preserve deterministic continuation status: %+v", checkpoint.Deterministic)
	}
	if err := ValidateCheckpoint(checkpoint); err != nil {
		t.Fatal(err)
	}

	baselineNext, err := session.Step(ctx)
	if err != nil {
		t.Fatal(err)
	}
	branchAAdapter, err := NewArachneOrganism(newNurseryRuntime(t, "nursery-real-runtime"), "branch-a")
	if err != nil {
		t.Fatal(err)
	}
	branchA, err := Fork(ctx, checkpoint, branchAAdapter, "branch-a")
	if err != nil {
		t.Fatal(err)
	}
	branchBAdapter, err := NewArachneOrganism(newNurseryRuntime(t, "nursery-real-runtime"), "branch-b")
	if err != nil {
		t.Fatal(err)
	}
	branchB, err := Fork(ctx, checkpoint, branchBAdapter, "branch-b")
	if err != nil {
		t.Fatal(err)
	}
	nextA, err := branchA.Step(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nextB, err := branchB.Step(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if baselineNext.Tick != 9 || nextA.Tick != 9 || nextB.Tick != 9 ||
		!reflect.DeepEqual(baselineNext.Action, nextA.Action) || !reflect.DeepEqual(nextA.Action, nextB.Action) ||
		!sameProposals(baselineNext.Cognition.Proposals, nextA.Cognition.Proposals) ||
		!sameProposals(nextA.Cognition.Proposals, nextB.Cognition.Proposals) ||
		!sameWorldOutcome(session.World, branchA.World) || !sameWorldOutcome(branchA.World, branchB.World) {
		t.Fatal("identical exact branches diverged without an intervention")
	}
	if branchA.Organism.(*ArachneOrganism).Runtime == branchB.Organism.(*ArachneOrganism).Runtime ||
		branchA.BranchID == branchB.BranchID || branchA.ParentCheckpointID != branchB.ParentCheckpointID {
		t.Fatal("restored branches did not retain independent runtime and lineage identities")
	}
	if nextA.Cognition.Events[0] == nil || nextB.Cognition.Events[0] == nil || reflect.DeepEqual(nextA.Cognition.Events, nextB.Cognition.Events) {
		t.Fatal("continuation event identities were not isolated between branches")
	}
	before := append([]Cell(nil), branchA.World.Cells...)
	branchB.World.Cells[0] = Cell{}
	if !reflect.DeepEqual(branchA.World.Cells, before) {
		t.Fatal("forked worlds share mutable cell storage")
	}
	if len(branchA.History) != 9 || branchA.ParentCheckpointID != checkpoint.ID {
		t.Fatalf("restored runtime lost checkpoint lineage: history=%d parent=%q", len(branchA.History), branchA.ParentCheckpointID)
	}
}

func sameProposals(left, right []json.RawMessage) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		var first, second agent.Proposal
		if json.Unmarshal(left[index], &first) != nil || json.Unmarshal(right[index], &second) != nil {
			return false
		}
		if first.ID != second.ID || first.Status != second.Status || first.Summary != second.Summary ||
			first.Confidence != second.Confidence || !reflect.DeepEqual(first.RequestedActions, second.RequestedActions) {
			return false
		}
	}
	return true
}

func sameWorldOutcome(left, right *World) bool {
	return left != nil && right != nil && left.Config == right.Config && left.Tick == right.Tick &&
		left.RNGState == right.RNGState && left.Position == right.Position &&
		left.Viability == right.Viability && reflect.DeepEqual(left.Cells, right.Cells) &&
		reflect.DeepEqual(left.Entities, right.Entities) && reflect.DeepEqual(left.Inventory, right.Inventory)
}
