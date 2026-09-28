package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

const (
	learnableWeightScale = 0.1
	maxLearnableFeatures = 4096
)

// NumericParameter declares one bounded numeric field for an action payload.
type NumericParameter struct {
	Name string  `json:"name"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// ActionCapability declares an action kind and the numeric fields it may propose.
type ActionCapability struct {
	Kind              string             `json:"kind"`
	NumericParameters []NumericParameter `json:"numeric_parameters,omitempty"`
}

// LearnableSpecialistConfig fixes one specialist's identity, feature space, and capabilities.
type LearnableSpecialistConfig struct {
	ID               string             `json:"id"`
	Seed             uint64             `json:"seed"`
	FeatureDimension int                `json:"feature_dimension"`
	Actions          []ActionCapability `json:"actions"`
	LearningRate     float64            `json:"learning_rate,omitempty"`
	MaxActions       int                `json:"max_actions,omitempty"`
}

type linearHead struct {
	Bias    float64
	Weights []float64
}

type actionHeads struct {
	Kind       string
	Score      linearHead
	Parameters map[string]linearHead
}

// LearnableUpdate records the externally supplied target used for one update.
type LearnableUpdate struct {
	Index          uint64    `json:"index"`
	ActionKind     string    `json:"action_kind"`
	TargetScore    float64   `json:"target_score"`
	PredictedScore float64   `json:"predicted_score"`
	Features       []float64 `json:"features"`
}

// LearnableActionParameters is a plain data representation of one action's heads.
type LearnableActionParameters struct {
	Kind       string                  `json:"kind"`
	Score      LinearParameters        `json:"score"`
	Parameters []NamedLinearParameters `json:"parameters,omitempty"`
}

// LinearParameters stores a linear head's bias and feature weights.
type LinearParameters struct {
	Bias    float64   `json:"bias"`
	Weights []float64 `json:"weights"`
}

// NamedLinearParameters stores the head for one declared numeric payload field.
type NamedLinearParameters struct {
	Name   string           `json:"name"`
	Linear LinearParameters `json:"linear"`
}

// LearnableSpecialistSnapshot is a serializable copy of all specialist state.
type LearnableSpecialistSnapshot struct {
	Config      LearnableSpecialistConfig   `json:"config"`
	Parameters  []LearnableActionParameters `json:"parameters"`
	RNGState    uint64                      `json:"rng_state"`
	UpdateCount uint64                      `json:"update_count"`
	LastUpdate  *LearnableUpdate            `json:"last_update,omitempty"`
}

// LearnableSpecialist proposes only actions declared by its immutable configuration.
type LearnableSpecialist struct {
	mu          sync.RWMutex
	config      LearnableSpecialistConfig
	actions     []actionHeads
	actionIndex map[string]int
	rngState    uint64
	updateCount uint64
	lastUpdate  *LearnableUpdate
}

// NewLearnableSpecialist validates a capability set and initializes independent seeded heads.
func NewLearnableSpecialist(config LearnableSpecialistConfig) (*LearnableSpecialist, error) {
	config, err := normalizeLearnableConfig(config)
	if err != nil {
		return nil, err
	}
	seed := config.Seed
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	specialist := &LearnableSpecialist{
		config: config, actionIndex: make(map[string]int), rngState: seed,
		actions: make([]actionHeads, 0, len(config.Actions)),
	}
	specialist.initializeActions()
	return specialist, nil
}

func normalizeLearnableConfig(config LearnableSpecialistConfig) (LearnableSpecialistConfig, error) {
	if strings.TrimSpace(config.ID) == "" {
		return config, errors.New("learnable specialist ID is required")
	}
	if config.FeatureDimension <= 0 || config.FeatureDimension > maxLearnableFeatures {
		return config, fmt.Errorf("feature dimension must be between 1 and %d", maxLearnableFeatures)
	}
	if len(config.Actions) == 0 || len(config.Actions) > maxProposalActions {
		return config, fmt.Errorf("action capabilities must contain between 1 and %d entries", maxProposalActions)
	}
	if config.LearningRate == 0 {
		config.LearningRate = 0.1
	}
	if !isFinite(config.LearningRate) || config.LearningRate <= 0 {
		return config, errors.New("learning rate must be finite and greater than zero")
	}
	if config.MaxActions == 0 {
		config.MaxActions = maxProposalActions
	}
	if config.MaxActions < 1 || config.MaxActions > maxProposalActions {
		return config, fmt.Errorf("maximum actions must be between 1 and %d", maxProposalActions)
	}
	config.Actions = cloneCapabilities(config.Actions)
	for i := range config.Actions {
		config.Actions[i].Kind = strings.TrimSpace(config.Actions[i].Kind)
		for j := range config.Actions[i].NumericParameters {
			config.Actions[i].NumericParameters[j].Name = strings.TrimSpace(config.Actions[i].NumericParameters[j].Name)
		}
	}
	if err := validateCapabilities(config.Actions); err != nil {
		return config, err
	}
	return config, nil
}

func validateCapabilities(actions []ActionCapability) error {
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		if action.Kind == "" {
			return errors.New("action capability kind must not be empty")
		}
		if _, exists := seen[action.Kind]; exists {
			return fmt.Errorf("duplicate action capability %q", action.Kind)
		}
		seen[action.Kind] = struct{}{}
		if err := validateNumericParameters(action); err != nil {
			return err
		}
	}
	return nil
}

func validateNumericParameters(action ActionCapability) error {
	seen := make(map[string]struct{}, len(action.NumericParameters))
	for _, parameter := range action.NumericParameters {
		if parameter.Name == "" {
			return fmt.Errorf("action %q has a numeric parameter with an empty name", action.Kind)
		}
		if !isFinite(parameter.Min) || !isFinite(parameter.Max) || parameter.Min > parameter.Max {
			return fmt.Errorf("action %q parameter %q requires finite ordered bounds", action.Kind, parameter.Name)
		}
		if _, exists := seen[parameter.Name]; exists {
			return fmt.Errorf("action %q has duplicate numeric parameter %q", action.Kind, parameter.Name)
		}
		seen[parameter.Name] = struct{}{}
	}
	return nil
}

func (s *LearnableSpecialist) initializeActions() {
	actions := append([]ActionCapability(nil), s.config.Actions...)
	sort.Slice(actions, func(i, j int) bool { return actions[i].Kind < actions[j].Kind })
	for _, capability := range actions {
		head := actionHeads{Kind: capability.Kind, Parameters: make(map[string]linearHead)}
		head.Score = s.newHead()
		for _, parameter := range capability.NumericParameters {
			head.Parameters[parameter.Name] = s.newHead()
		}
		s.actionIndex[head.Kind] = len(s.actions)
		s.actions = append(s.actions, head)
	}
}

type scoredLearnableAction struct {
	head       actionHeads
	score      float64
	confidence float64
}

// Propose computes deterministic scores and bounded numeric payloads without mutating state.
func (s *LearnableSpecialist) Propose(ctx context.Context, activation Activation) (Proposal, error) {
	if err := ctx.Err(); err != nil {
		return Proposal{}, err
	}
	features := append([]float64(nil), activation.Features...)
	if err := s.validateFeatures(features); err != nil {
		return Proposal{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	scored, err := s.scoreActions(features)
	if err != nil {
		return Proposal{}, err
	}
	actions, err := s.proposeActions(scored, features)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{
		Summary:          "learnable specialist candidate",
		Confidence:       scored[0].confidence,
		RequestedActions: actions,
	}, nil
}

func (s *LearnableSpecialist) scoreActions(features []float64) ([]scoredLearnableAction, error) {
	scored := make([]scoredLearnableAction, 0, len(s.actions))
	maxScore := math.Inf(-1)
	for _, head := range s.actions {
		score := applyHead(head.Score, features)
		if !isFinite(score) {
			return nil, fmt.Errorf("action %q score is not finite", head.Kind)
		}
		maxScore = math.Max(maxScore, score)
		scored = append(scored, scoredLearnableAction{head: head, score: score})
	}
	total := 0.0
	for i := range scored {
		scored[i].confidence = math.Exp(scored[i].score - maxScore)
		total += scored[i].confidence
	}
	for i := range scored {
		scored[i].confidence /= total
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].head.Kind < scored[j].head.Kind
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > s.config.MaxActions {
		scored = scored[:s.config.MaxActions]
	}
	return scored, nil
}

func (s *LearnableSpecialist) proposeActions(scored []scoredLearnableAction, features []float64) ([]RequestedAction, error) {
	actions := make([]RequestedAction, 0, len(scored))
	for _, candidate := range scored {
		payload, err := s.actionPayload(candidate.head, features)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode action %q payload: %w", candidate.head.Kind, err)
		}
		actions = append(actions, RequestedAction{Kind: candidate.head.Kind, Payload: encoded})
	}
	return actions, nil
}

func (s *LearnableSpecialist) actionPayload(head actionHeads, features []float64) (map[string]float64, error) {
	payload := make(map[string]float64, len(head.Parameters))
	for _, capability := range s.config.Actions {
		if capability.Kind != head.Kind {
			continue
		}
		for _, parameter := range capability.NumericParameters {
			value := applyHead(head.Parameters[parameter.Name], features)
			if !isFinite(value) {
				return nil, fmt.Errorf("action %q parameter %q output is not finite", head.Kind, parameter.Name)
			}
			normalized := (math.Tanh(value) + 1) / 2
			mapped := parameter.Min + normalized*(parameter.Max-parameter.Min)
			payload[parameter.Name] = clamp(mapped, parameter.Min, parameter.Max)
		}
		break
	}
	return payload, nil
}

// Update applies one squared-error gradient step using a caller-provided bounded target score.
func (s *LearnableSpecialist) Update(features []float64, actionKind string, targetScore float64) (LearnableUpdate, error) {
	if err := s.validateFeatures(features); err != nil {
		return LearnableUpdate{}, err
	}
	if !isFinite(targetScore) || targetScore < 0 || targetScore > 1 {
		return LearnableUpdate{}, errors.New("target score must be finite and between zero and one")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	index, exists := s.actionIndex[actionKind]
	if !exists {
		return LearnableUpdate{}, fmt.Errorf("action %q is not declared by specialist %q", actionKind, s.config.ID)
	}
	head := &s.actions[index].Score
	logit := applyHead(*head, features)
	if !isFinite(logit) {
		return LearnableUpdate{}, errors.New("action score is not finite for the observed features")
	}
	prediction := sigmoid(logit)
	gradient := (targetScore - prediction) * prediction * (1 - prediction)
	updated := linearHead{Bias: head.Bias + s.config.LearningRate*gradient, Weights: append([]float64(nil), head.Weights...)}
	if !isFinite(updated.Bias) {
		return LearnableUpdate{}, errors.New("update would produce a non-finite action bias")
	}
	for i, feature := range features {
		updated.Weights[i] += s.config.LearningRate * gradient * feature
		if !isFinite(updated.Weights[i]) {
			return LearnableUpdate{}, fmt.Errorf("update would produce a non-finite weight for feature %d", i)
		}
	}
	*head = updated
	s.updateCount++
	update := LearnableUpdate{
		Index: s.updateCount, ActionKind: actionKind, TargetScore: targetScore,
		PredictedScore: prediction, Features: append([]float64(nil), features...),
	}
	s.lastUpdate = &update
	return cloneUpdate(update), nil
}

// UpdateOutcome adapts a bounded outcome target to the organism learner boundary.
func (s *LearnableSpecialist) UpdateOutcome(_ context.Context, features []float64, actionKind string, targetScore float64) (json.RawMessage, error) {
	update, err := s.Update(features, actionKind, targetScore)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// Snapshot returns a deep plain-data copy suitable for inspection or JSON serialization.
func (s *LearnableSpecialist) Snapshot() LearnableSpecialistSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := LearnableSpecialistSnapshot{
		Config: cloneConfig(s.config), RNGState: s.rngState, UpdateCount: s.updateCount,
		Parameters: make([]LearnableActionParameters, 0, len(s.actions)),
	}
	for _, head := range s.actions {
		parameters := LearnableActionParameters{Kind: head.Kind, Score: linearParameters(head.Score)}
		names := make([]string, 0, len(head.Parameters))
		for name := range head.Parameters {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			parameters.Parameters = append(parameters.Parameters, NamedLinearParameters{
				Name: name, Linear: linearParameters(head.Parameters[name]),
			})
		}
		snapshot.Parameters = append(snapshot.Parameters, parameters)
	}
	if s.lastUpdate != nil {
		last := cloneUpdate(*s.lastUpdate)
		snapshot.LastUpdate = &last
	}
	return snapshot
}

func (s *LearnableSpecialist) validateFeatures(features []float64) error {
	if len(features) != s.config.FeatureDimension {
		return fmt.Errorf("specialist %q requires %d features, got %d", s.config.ID, s.config.FeatureDimension, len(features))
	}
	for i, feature := range features {
		if !isFinite(feature) {
			return fmt.Errorf("feature %d must be finite", i)
		}
	}
	return nil
}

func (s *LearnableSpecialist) newHead() linearHead {
	head := linearHead{Bias: s.nextRandom()*learnableWeightScale - learnableWeightScale/2, Weights: make([]float64, s.config.FeatureDimension)}
	for i := range head.Weights {
		head.Weights[i] = s.nextRandom()*learnableWeightScale - learnableWeightScale/2
	}
	return head
}

func (s *LearnableSpecialist) nextRandom() float64 {
	x := s.rngState
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	s.rngState = x
	value := x * 0x2545f4914f6cdd1d
	return float64(value>>11) / (1 << 53)
}

func applyHead(head linearHead, features []float64) float64 {
	value := head.Bias
	for i, feature := range features {
		value += head.Weights[i] * feature
	}
	return value
}

func sigmoid(value float64) float64 {
	if value >= 0 {
		return 1 / (1 + math.Exp(-value))
	}
	exp := math.Exp(value)
	return exp / (1 + exp)
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func cloneCapabilities(actions []ActionCapability) []ActionCapability {
	cloned := make([]ActionCapability, len(actions))
	for i, action := range actions {
		cloned[i] = action
		cloned[i].NumericParameters = append([]NumericParameter(nil), action.NumericParameters...)
	}
	return cloned
}

func cloneConfig(config LearnableSpecialistConfig) LearnableSpecialistConfig {
	config.Actions = cloneCapabilities(config.Actions)
	return config
}

func linearParameters(head linearHead) LinearParameters {
	return LinearParameters{Bias: head.Bias, Weights: append([]float64(nil), head.Weights...)}
}

func cloneUpdate(update LearnableUpdate) LearnableUpdate {
	update.Features = append([]float64(nil), update.Features...)
	return update
}
