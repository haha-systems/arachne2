// Package inspection builds read-only developmental reports from cognitive event history.
package inspection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/haha-systems/arachne2/internal/cognition"
)

const readBatchSize = 256

// Category groups related changes for a developmental report.
type Category string

// Category values describe the developmental event families exposed in reports.
const (
	CategoryProcedure   Category = "procedure"
	CategoryMemory      Category = "memory"
	CategorySpecialist  Category = "specialist"
	CategoryRegulation  Category = "regulation"
	CategoryStructure   Category = "structure"
	CategoryGovernance  Category = "governance"
	CategoryDevelopment Category = "development"
)

// Reader is the read side of the cognitive event spine.
type Reader interface {
	Read(context.Context, uint64, int) ([]cognition.Event, error)
}

// Inspector creates developmental views without mutating organism state.
type Inspector struct {
	events Reader
}

// Activity is one meaningful change or decision, with links back to causal events.
type Activity struct {
	EventID            string                  `json:"event_id"`
	Sequence           uint64                  `json:"sequence"`
	OccurredAt         time.Time               `json:"occurred_at"`
	OrganismID         string                  `json:"organism_id"`
	Category           Category                `json:"category"`
	Operation          string                  `json:"operation,omitempty"`
	Subject            string                  `json:"subject,omitempty"`
	Summary            string                  `json:"summary"`
	ParentEventIDs     []string                `json:"parent_event_ids,omitempty"`
	ProvenanceEventIDs []string                `json:"provenance_event_ids,omitempty"`
	GovernanceEventIDs []string                `json:"governance_event_ids,omitempty"`
	Payload            json.RawMessage         `json:"payload"`
	Silk               *cognition.SilkTraceRef `json:"silk,omitempty"`
}

// Report is an ordered, provenance-linked view of one organism's developmental history.
type Report struct {
	OrganismID string     `json:"organism_id"`
	Through    uint64     `json:"through_sequence"`
	Activities []Activity `json:"activities"`
}

// Difference explains which developmental activities occur in only one history.
type Difference struct {
	LeftOnly  []Activity `json:"left_only"`
	RightOnly []Activity `json:"right_only"`
	Shared    int        `json:"shared_activities"`
}

// Comparison contains both source reports and the activities that distinguish them.
type Comparison struct {
	Left  Report     `json:"left"`
	Right Report     `json:"right"`
	Diff  Difference `json:"difference"`
}

// Evolution links a baseline report to the activities recorded after that point.
type Evolution struct {
	Initial Report     `json:"initial"`
	Current Report     `json:"current"`
	Changes []Activity `json:"changes_since_initial"`
}

// New creates an inspector over an event reader.
func New(events Reader) (*Inspector, error) {
	if events == nil {
		return nil, errors.New("cognitive event reader is required")
	}
	return &Inspector{events: events}, nil
}

// Inspect returns the categorized event history for one organism.
func (i *Inspector) Inspect(ctx context.Context, organismID string) (Report, error) {
	if strings.TrimSpace(organismID) == "" {
		return Report{}, errors.New("organism ID is required")
	}
	events, err := i.readAll(ctx)
	if err != nil {
		return Report{}, err
	}
	byID := make(map[string]cognition.Event, len(events))
	byEntity := make(map[string][]string)
	for _, event := range events {
		if event.OrganismID == organismID {
			byID[event.EventID] = event
			indexEntities(event, byEntity)
		}
	}
	report := Report{OrganismID: organismID, Activities: make([]Activity, 0)}
	for _, event := range events {
		if event.OrganismID != organismID {
			continue
		}
		report.Through = event.Sequence
		category, operation, subject, include := classify(event)
		if !include {
			continue
		}
		provenance, governance := resolveLineage(event, byID, byEntity)
		report.Activities = append(report.Activities, Activity{
			EventID: event.EventID, Sequence: event.Sequence, OccurredAt: event.OccurredAt,
			OrganismID: event.OrganismID, Category: category, Operation: operation,
			Subject: subject, Summary: summarize(event, category, operation, subject),
			ParentEventIDs:     append([]string(nil), event.ParentEventIDs...),
			ProvenanceEventIDs: provenance, GovernanceEventIDs: governance,
			Payload: append(json.RawMessage(nil), event.Payload...), Silk: cognition.CloneSilkTraceRef(event.Silk),
		})
	}
	return report, nil
}

// CompareReports compares reports collected from separate organism event stores.
func CompareReports(left, right Report) (Comparison, error) {
	if left.OrganismID == "" || right.OrganismID == "" || left.OrganismID == right.OrganismID {
		return Comparison{}, errors.New("comparison requires reports from two distinct organisms")
	}
	leftActivities := sortedActivities(left.Activities)
	rightActivities := sortedActivities(right.Activities)
	difference := Difference{
		LeftOnly: make([]Activity, 0), RightOnly: make([]Activity, 0),
	}
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(leftActivities) && rightIndex < len(rightActivities) {
		leftKey := activityKey(leftActivities[leftIndex])
		rightKey := activityKey(rightActivities[rightIndex])
		switch {
		case leftKey == rightKey:
			difference.Shared++
			leftIndex++
			rightIndex++
		case leftKey < rightKey:
			difference.LeftOnly = append(difference.LeftOnly, leftActivities[leftIndex])
			leftIndex++
		default:
			difference.RightOnly = append(difference.RightOnly, rightActivities[rightIndex])
			rightIndex++
		}
	}
	for ; leftIndex < len(leftActivities); leftIndex++ {
		difference.LeftOnly = append(difference.LeftOnly, leftActivities[leftIndex])
	}
	for ; rightIndex < len(rightActivities); rightIndex++ {
		difference.RightOnly = append(difference.RightOnly, rightActivities[rightIndex])
	}
	return Comparison{Left: left, Right: right, Diff: difference}, nil
}

// ChangesSince selects activities emitted after an earlier report of the same organism.
func ChangesSince(initial, current Report) (Evolution, error) {
	if initial.OrganismID == "" || initial.OrganismID != current.OrganismID {
		return Evolution{}, errors.New("baseline and current reports must belong to the same organism")
	}
	if current.Through < initial.Through {
		return Evolution{}, errors.New("current report predates the initial report")
	}
	changes := make([]Activity, 0)
	for _, activity := range current.Activities {
		if activity.Sequence > initial.Through {
			changes = append(changes, activity)
		}
	}
	return Evolution{Initial: initial, Current: current, Changes: changes}, nil
}

func (i *Inspector) readAll(ctx context.Context) ([]cognition.Event, error) {
	all := make([]cognition.Event, 0)
	var after uint64
	for {
		batch, err := i.events.Read(ctx, after, readBatchSize)
		if err != nil {
			return nil, fmt.Errorf("read cognitive event history: %w", err)
		}
		if len(batch) == 0 {
			return all, nil
		}
		for _, event := range batch {
			if event.Sequence <= after {
				return nil, fmt.Errorf("event reader returned non-increasing sequence %d after %d", event.Sequence, after)
			}
			after = event.Sequence
			all = append(all, event)
		}
		if len(batch) < readBatchSize {
			return all, nil
		}
	}
}

func classify(event cognition.Event) (Category, string, string, bool) {
	fields := decodeFields(event.Payload)
	operation := fields.string("operation")
	switch event.Kind {
	case cognition.KindGovernance:
		class := fields.string("action_class")
		if class == "structural_change" {
			return CategoryStructure, operationOr(operation, "governance_decision"), firstValue(fields, "target", "action"), true
		}
		return CategoryGovernance, operationOr(operation, "governance_decision"), firstValue(fields, "target", "action"), true
	case cognition.KindRegulation:
		return CategoryRegulation, operationOr(operation, "regulation_evaluated"), firstValue(fields, "cognitive_state", "interaction_id"), true
	case cognition.KindMemory, cognition.KindReplay:
		return CategoryMemory, operationOr(operation, event.Kind), firstValue(fields,
			"episode_id", "pattern_id", "run_id", "plan_id", "semantic_record_id"), true
	case cognition.KindProposal:
		specialist := fields.string("specialist_id")
		if specialist == "" {
			return "", "", "", false
		}
		return CategorySpecialist, operationOr(operation, "specialist_proposal"), specialist, true
	case cognition.KindDevelopment:
		if strings.Contains(operation, "structural") || strings.Contains(operation, "topology") {
			return CategoryStructure, operation, firstValue(fields, "target", "subject"), true
		}
		if strings.Contains(operation, "silk_registry") || strings.Contains(operation, "silk_candidate") ||
			fields.has("procedure_id") || fields.nestedHas("result", "procedure_id") {
			return CategoryProcedure, operationOr(operation, "procedure_development"), firstValue(fields,
				"procedure_id", "revision_digest", "entry_procedure"), true
		}
		return CategoryDevelopment, operationOr(operation, "development_change"), firstValue(fields, "key", "subject"), true
	case cognition.KindAction:
		if fields.has("procedure_id") {
			return CategoryProcedure, "procedure_reused", firstValue(fields, "procedure_id", "revision_digest"), true
		}
	case cognition.KindSilkTrace:
		if event.Silk != nil && event.Silk.Procedure != "" {
			return CategoryProcedure, "procedure_trace", event.Silk.Procedure, true
		}
	}
	return "", "", "", false
}

func resolveLineage(event cognition.Event, byID map[string]cognition.Event, byEntity map[string][]string) ([]string, []string) {
	seen := make(map[string]struct{})
	governance := make(map[string]struct{})
	if event.Kind == cognition.KindGovernance {
		governance[event.EventID] = struct{}{}
	}
	queue := append([]string(nil), event.ParentEventIDs...)
	fields := decodeFields(event.Payload)
	queue = append(queue, fields.strings("source_event_ids")...)
	queue = append(queue, fields.strings("evidence_event_ids")...)
	queue = append(queue, fields.strings("governance_event_id")...)
	queue = appendEntityEvents(queue, fields, byEntity)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id == "" || id == event.EventID {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ancestor, exists := byID[id]
		if !exists {
			continue
		}
		if ancestor.Kind == cognition.KindGovernance {
			governance[id] = struct{}{}
		}
		ancestorFields := decodeFields(ancestor.Payload)
		queue = append(queue, ancestor.ParentEventIDs...)
		queue = append(queue, ancestorFields.strings("source_event_ids")...)
		queue = append(queue, ancestorFields.strings("evidence_event_ids")...)
		queue = appendEntityEvents(queue, ancestorFields, byEntity)
	}
	return sortedKeys(seen), sortedKeys(governance)
}

func indexEntities(event cognition.Event, index map[string][]string) {
	fields := decodeFields(event.Payload)
	var entities []struct{ kind, id string }
	switch fields.string("operation") {
	case "episode_recorded":
		entities = append(entities, struct{ kind, id string }{"episode", fields.string("episode_id")})
	case "consolidation_applied":
		for _, id := range fields.strings("pattern_ids") {
			entities = append(entities, struct{ kind, id string }{"pattern", id})
		}
	case "replay_scheduled":
		entities = append(entities, struct{ kind, id string }{"plan", fields.string("plan_id")})
	case "silk_registry_admitted", "silk_registry_retained":
		entities = append(entities, struct{ kind, id string }{"procedure", fields.string("procedure_id")})
	}
	for _, entity := range entities {
		if entity.id != "" {
			key := entity.kind + ":" + entity.id
			index[key] = append(index[key], event.EventID)
		}
	}
}

func appendEntityEvents(queue []string, fields fields, index map[string][]string) []string {
	for _, field := range []struct{ key, kind string }{
		{"episode_id", "episode"}, {"source_episode_ids", "episode"}, {"episode_ids", "episode"},
		{"pattern_id", "pattern"}, {"pattern_ids", "pattern"},
		{"run_id", "run"}, {"plan_id", "plan"}, {"procedure_id", "procedure"},
	} {
		values := fields.strings(field.key)
		if value := fields.string(field.key); value != "" {
			values = append(values, value)
		}
		for _, value := range values {
			queue = append(queue, index[field.kind+":"+value]...)
		}
	}
	return queue
}

func decodeFields(payload json.RawMessage) fields {
	var root map[string]json.RawMessage
	if json.Unmarshal(payload, &root) != nil {
		return fields{}
	}
	result := fields{root: root}
	for _, key := range []string{"change", "result", "decision"} {
		var nested map[string]json.RawMessage
		if json.Unmarshal(root[key], &nested) == nil {
			for nestedKey, value := range nested {
				if _, exists := result.root[nestedKey]; !exists {
					result.root[nestedKey] = value
				}
			}
		}
	}
	return result
}

type fields struct {
	root map[string]json.RawMessage
}

func (f fields) string(key string) string {
	var value string
	_ = json.Unmarshal(f.root[key], &value)
	return strings.TrimSpace(value)
}

func (f fields) rawString(key string) string {
	value := f.root[key]
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	return string(value)
}

func (f fields) strings(key string) []string {
	var values []string
	_ = json.Unmarshal(f.root[key], &values)
	return values
}

func (f fields) has(key string) bool { return f.string(key) != "" }

func (f fields) nestedHas(parent, key string) bool {
	var nested map[string]json.RawMessage
	if json.Unmarshal(f.root[parent], &nested) != nil {
		return false
	}
	var value string
	_ = json.Unmarshal(nested[key], &value)
	return value != ""
}

func firstValue(f fields, keys ...string) string {
	for _, key := range keys {
		if value := f.string(key); value != "" {
			return value
		}
	}
	return ""
}

func operationOr(operation, fallback string) string {
	if operation != "" {
		return operation
	}
	return fallback
}

func summarize(event cognition.Event, category Category, operation, subject string) string {
	label := strings.ReplaceAll(string(category), "_", " ")
	if operation != "" {
		label += ": " + strings.ReplaceAll(operation, "_", " ")
	}
	if subject != "" {
		label += " — " + subject
	}
	fields := decodeFields(event.Payload)
	switch category {
	case CategoryDevelopment:
		previous := fields.rawString("previous")
		value := fields.rawString("value")
		if previous != "" && value != "" {
			label += " (" + previous + " → " + value + ")"
		}
	case CategoryGovernance, CategoryStructure:
		if outcome := fields.string("outcome"); outcome != "" {
			label += " [" + outcome + "]"
		}
	case CategoryRegulation:
		if state := fields.string("cognitive_state"); state != "" && state != subject {
			label += " [" + state + "]"
		}
	}
	return label
}

func sortedActivities(activities []Activity) []Activity {
	sorted := append([]Activity(nil), activities...)
	sort.Slice(sorted, func(left, right int) bool {
		return activityKey(sorted[left]) < activityKey(sorted[right])
	})
	return sorted
}

func activityKey(activity Activity) string {
	canonical, err := canonicalPayload(activity.Payload)
	if err != nil {
		canonical = activity.Payload
	}
	return string(activity.Category) + "\x00" + activity.Operation + "\x00" + comparisonSubject(activity) + "\x00" + string(canonical)
}

func comparisonSubject(activity Activity) string {
	switch activity.Category {
	case CategoryProcedure, CategorySpecialist, CategoryStructure, CategoryDevelopment, CategoryGovernance:
		return activity.Subject
	default:
		return ""
	}
}

func canonicalPayload(payload json.RawMessage) ([]byte, error) {
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, err
	}
	return json.Marshal(stripIdentityFields(value))
}

func stripIdentityFields(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, nested := range typed {
			if comparisonIgnores(key) {
				continue
			}
			clean[key] = stripIdentityFields(nested)
		}
		return clean
	case []any:
		clean := make([]any, len(typed))
		for index, nested := range typed {
			clean[index] = stripIdentityFields(nested)
		}
		return clean
	default:
		return value
	}
}

func comparisonIgnores(key string) bool {
	switch key {
	case "organism_id", "event_id", "parent_event_ids", "source_event_ids", "evidence_event_ids",
		"governance_event_id", "governance_decision_id", "proposal_id", "decision_id", "episode_id",
		"pattern_id", "run_id", "plan_id", "created_at", "recorded_at", "occurred_at", "applied_at",
		"payload_digest", "proposal_digest", "policy_digest", "interaction_id", "session_id",
		"prediction_error_event_id", "regulation_event_id":
		return true
	default:
		return strings.HasSuffix(key, "_event_ids")
	}
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
