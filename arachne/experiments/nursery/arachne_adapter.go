package nursery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/haha-systems/arachne2/internal/agent"
	"github.com/haha-systems/arachne2/internal/organism"
)

// ArachneOrganism is the thin Nursery boundary around the persistent Arachne runtime.
type ArachneOrganism struct {
	Runtime            *organism.Runtime
	BranchID           string
	pendingInteraction string
	lastTrace          CognitiveTrace
}

// NewArachneOrganism binds one fresh or restored runtime to one explicit branch identity.
func NewArachneOrganism(runtime *organism.Runtime, branchID string) (*ArachneOrganism, error) {
	if runtime == nil || branchID == "" {
		return nil, errors.New("arachne runtime and branch ID are required")
	}
	if !runtime.DeferredExecution() {
		return nil, errors.New("nursery requires a runtime configured for deferred environment consequences")
	}
	return &ArachneOrganism{Runtime: runtime, BranchID: branchID}, nil
}

// Decide translates Nursery observations to generic Arachne features and returns the committed action.
func (o *ArachneOrganism) Decide(ctx context.Context, perception Perception) (Action, CognitiveTrace, error) {
	if o.pendingInteraction != "" {
		return Action{}, CognitiveTrace{}, errors.New("previous Nursery consequence is still pending")
	}
	input, err := json.Marshal(perception)
	if err != nil {
		return Action{}, CognitiveTrace{}, fmt.Errorf("encode Nursery perception: %w", err)
	}
	features := FeatureVector(perception)
	result, err := o.Runtime.Step(ctx, organism.Perception{
		SessionID: "nursery",
		Input:     input,
		Features:  features,
	})
	if err != nil {
		return Action{}, CognitiveTrace{}, err
	}
	if result.Eligibility != nil && !result.Eligibility.Eligible {
		return Action{}, traceFromStep(result, input, organism.DeterminismStatus{Deterministic: false, Reasons: []string{"committed action was ineligible"}}), errors.New("arachne commitment was not eligible for the nursery actuator")
	}
	action, err := nurseryAction(result.Commitment)
	if err != nil {
		completed, completionErr := o.Runtime.CompleteOutcome(ctx, result.InteractionID, organism.Outcome{
			Status: "failed", Error: err.Error(),
		})
		if completionErr != nil {
			return Action{}, CognitiveTrace{}, fmt.Errorf("record failed nursery action translation: %w", completionErr)
		}
		o.lastTrace = traceFromStep(completed, input, o.Runtime.Determinism())
		return Action{}, cloneCognition(o.lastTrace), err
	}
	if result.Outcome != nil {
		return Action{}, CognitiveTrace{}, errors.New("deferred Nursery runtime unexpectedly completed an outcome")
	}
	o.pendingInteraction = result.InteractionID
	o.lastTrace = traceFromStep(result, input, o.Runtime.Determinism())
	return action, cloneCognition(o.lastTrace), nil
}

// ReportConsequence closes the deferred step after the Nursery world has applied its action.
func (o *ArachneOrganism) ReportConsequence(ctx context.Context, consequence Consequence) (CognitiveTrace, error) {
	if o.pendingInteraction == "" {
		return CognitiveTrace{}, errors.New("no Nursery action is awaiting a consequence")
	}
	result, err := o.Runtime.CompleteOutcome(ctx, o.pendingInteraction, organism.Outcome{
		Status:           consequence.Status,
		Result:           append(json.RawMessage(nil), consequence.Result...),
		ObservedEvidence: append(json.RawMessage(nil), consequence.ObservedEvidence...),
		ConditionDeltas:  cloneDeltas(consequence.ConditionDeltas),
		Error:            consequence.Error,
	})
	if err != nil {
		return CognitiveTrace{}, err
	}
	o.pendingInteraction = ""
	o.lastTrace = traceFromStep(result, o.lastTrace.ModelInput, o.Runtime.Determinism())
	return cloneCognition(o.lastTrace), nil
}

// SnapshotState delegates to the runtime's explicit versioned state envelope.
func (o *ArachneOrganism) SnapshotState(ctx context.Context) (json.RawMessage, error) {
	if o == nil || o.Runtime == nil {
		return nil, errors.New("arachne runtime is required")
	}
	return o.Runtime.SnapshotState(ctx)
}

// SnapshotBestEffortState exports available runtime state and its opaque-component report.
func (o *ArachneOrganism) SnapshotBestEffortState(ctx context.Context) (json.RawMessage, error) {
	if o == nil || o.Runtime == nil {
		return nil, errors.New("arachne runtime is required")
	}
	return o.Runtime.SnapshotBestEffortState(ctx)
}

// RestoreState delegates to the runtime and uses the branch ID supplied by the embedding.
func (o *ArachneOrganism) RestoreState(ctx context.Context, state json.RawMessage) error {
	if o == nil || o.Runtime == nil || o.BranchID == "" {
		return errors.New("arachne runtime and branch ID are required")
	}
	if o.pendingInteraction != "" {
		return errors.New("cannot restore while a Nursery consequence is pending")
	}
	return o.Runtime.RestoreState(ctx, append(json.RawMessage(nil), state...), o.BranchID)
}

// Reproducibility reports runtime limits without serializing the actuator or provider.
func (o *ArachneOrganism) Reproducibility() Reproducibility {
	status := o.Runtime.Determinism()
	return Reproducibility{Deterministic: status.Deterministic, Reasons: append([]string(nil), status.Reasons...)}
}

// RestorationReport maps the core runtime inventory to Nursery's generic checkpoint contract.
func (o *ArachneOrganism) RestorationReport() RestorationReport {
	report := o.Runtime.RuntimeRestorability()
	result := RestorationReport{Exact: report.Exact, Components: make([]RestorationComponent, len(report.Components))}
	for i, component := range report.Components {
		result.Components[i] = RestorationComponent{
			ID: component.ID, Kind: component.Kind,
			Restorability: string(component.Restorability), Reason: component.Reason,
		}
	}
	return result
}

// FeatureVector defines stable generic scalar features for the Nursery perception adapter.
func FeatureVector(perception Perception) []float64 {
	return []float64{
		perception.Viability.Energy,
		perception.Viability.Integrity,
		perception.Viability.Stability,
		perception.Ambient,
		perception.Moisture,
		float64(perception.Position.X),
		float64(perception.Position.Y),
		float64(len(perception.Inventory)),
		float64(len(perception.Cells)),
		float64(len(perception.Entities)),
	}
}

func nurseryAction(commitment organism.Commitment) (Action, error) {
	if commitment.NoOp {
		return Action{Kind: "wait"}, nil
	}
	if err := agent.ValidateRequestedAction(commitment.Action); err != nil {
		return Action{}, fmt.Errorf("invalid committed Arachne action: %w", err)
	}
	var payload struct {
		DX         int    `json:"dx,omitempty"`
		DY         int    `json:"dy,omitempty"`
		EntityID   string `json:"entity_id,omitempty"`
		WithEntity string `json:"with_entity_id,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(commitment.Action.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return Action{}, fmt.Errorf("decode committed Nursery action: %w", err)
	}
	action := Action{
		Kind: commitment.Action.Kind, DX: payload.DX, DY: payload.DY,
		EntityID: payload.EntityID, WithEntityID: payload.WithEntity,
	}
	if err := ValidateAction(action); err != nil {
		return Action{}, fmt.Errorf("validate committed Nursery action: %w", err)
	}
	return action, nil
}

func traceFromStep(result organism.StepResult, input json.RawMessage, status organism.DeterminismStatus) CognitiveTrace {
	trace := CognitiveTrace{
		ActionSourceEventIDs: []string{result.Events.Action},
		Workspace:            mustJSON(result.Workspace),
		ModelInput:           append(json.RawMessage(nil), input...),
		Deterministic:        status.Deterministic,
		Nondeterminism:       append([]string(nil), status.Reasons...),
	}
	for _, proposal := range result.Proposals {
		if raw := mustJSON(proposal); len(raw) > 0 {
			trace.Proposals = append(trace.Proposals, raw)
		}
	}
	if raw := mustJSON(result.Events); len(raw) > 0 {
		trace.Events = append(trace.Events, raw)
	}
	if raw := mustJSON(result.Commitment.Action); len(raw) > 0 {
		trace.ModelOutput = raw
	}
	if result.Events.Memory != "" {
		trace.MemoryWrites = append(trace.MemoryWrites, result.Events.Memory)
	}
	trace.ReplayEventIDs = append([]string(nil), result.Events.Replay...)
	return trace
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func cloneDeltas(values map[string]float64) map[string]float64 {
	if values == nil {
		return nil
	}
	clone := make(map[string]float64, len(values))
	for name, value := range values {
		clone[name] = value
	}
	return clone
}
