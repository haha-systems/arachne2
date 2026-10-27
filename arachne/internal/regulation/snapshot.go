package regulation

import (
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
)

// ConditionValue is one named numeric condition in deterministic order.
type ConditionValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// StateSnapshot preserves mutable regulator state with its immutable policy identity.
type StateSnapshot struct {
	SchemaVersion string           `json:"schema_version"`
	Policy        Policy           `json:"policy"`
	Conditions    []ConditionValue `json:"conditions"`
}

// SnapshotState exports the complete mutable regulator state as plain data.
func (r *Regulator) SnapshotState() StateSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := StateSnapshot{SchemaVersion: "arachne.regulation_state.v1", Policy: r.policy}
	for name, value := range r.conditions {
		snapshot.Conditions = append(snapshot.Conditions, ConditionValue{Name: name, Value: value})
	}
	sort.Slice(snapshot.Conditions, func(i, j int) bool { return snapshot.Conditions[i].Name < snapshot.Conditions[j].Name })
	return snapshot
}

// RestoreState imports state only into a fresh regulator with the same policy.
func (r *Regulator) RestoreState(snapshot StateSnapshot) error {
	if snapshot.SchemaVersion != "arachne.regulation_state.v1" || !reflect.DeepEqual(snapshot.Policy, r.policy) {
		return errors.New("regulation snapshot schema or policy is incompatible")
	}
	conditions := make(map[string]float64, len(snapshot.Conditions))
	for _, condition := range snapshot.Conditions {
		if strings.TrimSpace(condition.Name) == "" || math.IsNaN(condition.Value) || math.IsInf(condition.Value, 0) {
			return errors.New("regulation snapshot contains an invalid condition")
		}
		if _, exists := conditions[condition.Name]; exists {
			return errors.New("regulation snapshot contains duplicate conditions")
		}
		conditions[condition.Name] = condition.Value
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.conditions) != 0 {
		return errors.New("regulator must be empty before restore")
	}
	r.conditions = conditions
	return nil
}
