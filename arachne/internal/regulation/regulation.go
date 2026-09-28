// Package regulation computes explicit organism-level signals for Arachne behavior.
package regulation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/workspace"
)

// Policy defines thresholds and the visible effect of high load or action pressure.
type Policy struct {
	HighLoadThreshold     float64
	HighPressureThreshold float64
	HighSurpriseThreshold float64
	HighSalienceThreshold float64
	CapacityReduction     int
}

// DefaultPolicy returns an inspectable starting policy, not a biological constant.
func DefaultPolicy() Policy {
	return Policy{
		HighLoadThreshold: 0.75, HighPressureThreshold: 0.75,
		HighSurpriseThreshold: 0.5, HighSalienceThreshold: 0.75,
		CapacityReduction: 1,
	}
}

// Input contains observed or explicitly declared measurements for one interaction.
type Input struct {
	InteractionID      string
	SessionID          string
	SourceEventIDs     []string
	Expected           json.RawMessage
	Observed           json.RawMessage
	DeclaredSalience   float64
	ActiveSpecialists  int
	SpecialistCapacity int
	PendingActions     int
	ActionCapacity     int
	WorkspaceCapacity  int
}

// PredictionError records a bounded mismatch and content digests, not raw values.
type PredictionError struct {
	Metric         string   `json:"metric"`
	Magnitude      float64  `json:"magnitude"`
	ExpectedDigest string   `json:"expected_digest"`
	ObservedDigest string   `json:"observed_digest"`
	SourceEventIDs []string `json:"source_event_ids,omitempty"`
	EventID        string   `json:"event_id"`
}

// Snapshot contains computed signals and their event provenance.
type Snapshot struct {
	InteractionID     string                    `json:"interaction_id"`
	SessionID         string                    `json:"session_id,omitempty"`
	CreatedAt         time.Time                 `json:"created_at"`
	PredictionError   float64                   `json:"prediction_error"`
	Surprise          float64                   `json:"surprise"`
	Salience          float64                   `json:"salience"`
	Load              float64                   `json:"load"`
	ActionPressure    float64                   `json:"action_pressure"`
	CognitiveState    string                    `json:"cognitive_state"`
	Prediction        PredictionError           `json:"prediction"`
	WorkspacePolicy   workspace.SelectionPolicy `json:"workspace_policy"`
	RegulationEventID string                    `json:"regulation_event_id"`
	SourceEventIDs    []string                  `json:"source_event_ids,omitempty"`
}

// Regulator computes signals, policy effects, and linked cognitive events.
type Regulator struct {
	policy Policy
	events *cognition.Spine
	now    func() time.Time
}

// New validates a regulator policy and event sink.
func New(policy Policy, events *cognition.Spine) (*Regulator, error) {
	if events == nil {
		return nil, errors.New("cognitive event spine is required")
	}
	for name, threshold := range map[string]float64{
		"high load":            policy.HighLoadThreshold,
		"high action pressure": policy.HighPressureThreshold,
		"high surprise":        policy.HighSurpriseThreshold,
		"high salience":        policy.HighSalienceThreshold,
	} {
		if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1 {
			return nil, fmt.Errorf("%s threshold must be between zero and one", name)
		}
	}
	if policy.CapacityReduction < 1 {
		return nil, errors.New("capacity reduction must be positive")
	}
	return &Regulator{policy: policy, events: events, now: time.Now}, nil
}

// Evaluate computes observable signals and emits prediction-error and regulation events.
func (r *Regulator) Evaluate(ctx context.Context, input Input) (Snapshot, error) {
	if strings.TrimSpace(input.InteractionID) == "" || len(input.Expected) == 0 || len(input.Observed) == 0 ||
		!json.Valid(input.Expected) || !json.Valid(input.Observed) {
		return Snapshot{}, errors.New("interaction ID and valid expected/observed JSON are required")
	}
	if !validRatioInput(input.DeclaredSalience) || input.SpecialistCapacity < 1 ||
		input.ActiveSpecialists < 0 || input.ActiveSpecialists > input.SpecialistCapacity ||
		input.ActionCapacity < 1 || input.PendingActions < 0 || input.PendingActions > input.ActionCapacity ||
		input.WorkspaceCapacity < 1 {
		return Snapshot{}, errors.New("salience must be normalized and activity counts must fit positive capacities")
	}
	magnitude, metric, err := predictionMagnitude(input.Expected, input.Observed)
	if err != nil {
		return Snapshot{}, err
	}
	createdAt := r.now().UTC()
	load := float64(input.ActiveSpecialists) / float64(input.SpecialistCapacity)
	pressure := float64(input.PendingActions) / float64(input.ActionCapacity)
	policy := workspace.SelectionPolicy{Capacity: input.WorkspaceCapacity}
	if load >= r.policy.HighLoadThreshold {
		policy.Capacity = reduce(policy.Capacity, r.policy.CapacityReduction)
		policy.Reasons = append(policy.Reasons, fmt.Sprintf("load %.3f met threshold %.3f; reduced workspace capacity", load, r.policy.HighLoadThreshold))
	}
	if pressure >= r.policy.HighPressureThreshold {
		policy.Capacity = reduce(policy.Capacity, r.policy.CapacityReduction)
		policy.Reasons = append(policy.Reasons, fmt.Sprintf("action pressure %.3f met threshold %.3f; reduced workspace capacity", pressure, r.policy.HighPressureThreshold))
	}
	if magnitude >= r.policy.HighSurpriseThreshold {
		policy.RequireEvidence = true
		policy.Reasons = append(policy.Reasons, fmt.Sprintf("prediction error %.3f met threshold %.3f; required proposal evidence", magnitude, r.policy.HighSurpriseThreshold))
	}
	if input.DeclaredSalience >= r.policy.HighSalienceThreshold {
		policy.RequireEvidence = true
		policy.Reasons = append(policy.Reasons, fmt.Sprintf("declared salience %.3f met threshold %.3f; required proposal evidence", input.DeclaredSalience, r.policy.HighSalienceThreshold))
	}
	state := "steady"
	if load >= r.policy.HighLoadThreshold || pressure >= r.policy.HighPressureThreshold {
		state = "strained"
	} else if magnitude >= r.policy.HighSurpriseThreshold || input.DeclaredSalience >= r.policy.HighSalienceThreshold {
		state = "alert"
	}
	expectedDigest, err := digest(input.Expected)
	if err != nil {
		return Snapshot{}, err
	}
	observedDigest, err := digest(input.Observed)
	if err != nil {
		return Snapshot{}, err
	}
	predictionPayload, err := json.Marshal(map[string]any{
		"interaction_id": input.InteractionID, "metric": metric, "magnitude": magnitude,
		"expected_digest": expectedDigest, "observed_digest": observedDigest,
		"source_event_ids": input.SourceEventIDs,
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode prediction-error event: %w", err)
	}
	predictionEvent, err := r.events.Emit(ctx, cognition.Draft{
		SessionID: input.SessionID, CorrelationID: input.InteractionID,
		ParentEventIDs: input.SourceEventIDs, Kind: cognition.KindPredictionError, Payload: predictionPayload,
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("record prediction error: %w", err)
	}
	policy.SignalEventIDs = []string{predictionEvent.EventID}
	payload, err := json.Marshal(map[string]any{
		"interaction_id": input.InteractionID, "prediction_error": magnitude,
		"surprise": magnitude, "salience": input.DeclaredSalience,
		"load": load, "action_pressure": pressure, "cognitive_state": state,
		"workspace_policy": policy,
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode regulation event: %w", err)
	}
	parents := append(append([]string(nil), input.SourceEventIDs...), predictionEvent.EventID)
	regulationEvent, err := r.events.Emit(ctx, cognition.Draft{
		SessionID: input.SessionID, CorrelationID: input.InteractionID,
		ParentEventIDs: unique(parents), Kind: cognition.KindRegulation, Payload: payload,
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("record regulation signals: %w", err)
	}
	policy.SignalEventIDs = append(policy.SignalEventIDs, regulationEvent.EventID)
	return Snapshot{
		InteractionID: input.InteractionID, SessionID: input.SessionID, CreatedAt: createdAt,
		PredictionError: magnitude, Surprise: magnitude, Salience: input.DeclaredSalience,
		Load: load, ActionPressure: pressure, CognitiveState: state,
		Prediction: PredictionError{
			Metric: metric, Magnitude: magnitude, ExpectedDigest: expectedDigest,
			ObservedDigest: observedDigest, SourceEventIDs: append([]string(nil), input.SourceEventIDs...),
			EventID: predictionEvent.EventID,
		},
		WorkspacePolicy: policy, RegulationEventID: regulationEvent.EventID,
		SourceEventIDs: append([]string(nil), input.SourceEventIDs...),
	}, nil
}

func predictionMagnitude(expectedRaw, observedRaw json.RawMessage) (float64, string, error) {
	expected, err := canonical(expectedRaw)
	if err != nil {
		return 0, "", fmt.Errorf("canonicalize expected outcome: %w", err)
	}
	observed, err := canonical(observedRaw)
	if err != nil {
		return 0, "", fmt.Errorf("canonicalize observed outcome: %w", err)
	}
	if expectedValue, expectedIsNumber := numericValue(expected); expectedIsNumber {
		if observedValue, observedIsNumber := numericValue(observed); observedIsNumber {
			denominator := math.Max(1, math.Abs(expectedValue))
			return math.Min(1, math.Abs(observedValue-expectedValue)/denominator), "normalized_absolute_numeric_error", nil
		}
	}
	if bytes.Equal(expected, observed) {
		return 0, "canonical_json_match", nil
	}
	return 1, "canonical_json_mismatch", nil
}

func numericValue(raw []byte) (float64, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return 0, false
	}
	number, isNumber := value.(json.Number)
	if !isNumber {
		return 0, false
	}
	parsed, err := number.Float64()
	return parsed, err == nil && !math.IsInf(parsed, 0) && !math.IsNaN(parsed)
}

func canonical(raw json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func digest(raw json.RawMessage) (string, error) {
	canonicalValue, err := canonical(raw)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(canonicalValue)
	return hex.EncodeToString(hash[:]), nil
}

func validRatioInput(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func reduce(capacity, reduction int) int {
	return max(1, capacity-reduction)
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
