package organism

import (
	"sort"

	"github.com/haha-systems/arachne2/internal/agent"
)

// SnapshotMode selects exact or explicitly best-effort state export.
type SnapshotMode string

const (
	// SnapshotExact permits only complete exact-restorable state.
	SnapshotExact SnapshotMode = "exact"
	// SnapshotBestEffort exports available state with blockers recorded.
	SnapshotBestEffort SnapshotMode = "best-effort"
)

// Restorability describes whether future behaviour can be recovered at a checkpoint.
type Restorability string

const (
	// RestorabilityStateless marks components with no mutable behaviour-relevant state.
	RestorabilityStateless Restorability = "stateless"
	// RestorabilityExact marks components that export all mutable behaviour-relevant state.
	RestorabilityExact Restorability = "exact"
	// RestorabilityReconstructable marks components rebuilt from canonical durable history.
	RestorabilityReconstructable Restorability = "reconstructable"
	// RestorabilityOpaque marks components whose behaviour-relevant state is unavailable.
	RestorabilityOpaque Restorability = "opaque"
)

// ComponentRestorability reports one active behaviourally relevant runtime component.
type ComponentRestorability struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind"`
	Restorability Restorability `json:"restorability"`
	Reason        string        `json:"reason,omitempty"`
}

// RestorabilityReport is exact only when every active component is covered by a contract.
type RestorabilityReport struct {
	Exact      bool                     `json:"exact"`
	Components []ComponentRestorability `json:"components"`
}

func (r *Runtime) externalComponent(id, kind, missingReason string) ComponentRestorability {
	status, declared := r.componentRestorability[id]
	if declared && (status == RestorabilityStateless || status == RestorabilityReconstructable) {
		return ComponentRestorability{ID: id, Kind: kind, Restorability: status}
	}
	reason := missingReason
	if declared {
		reason = "declared exact state is not captured by this runtime snapshot"
	}
	return ComponentRestorability{ID: id, Kind: kind, Restorability: RestorabilityOpaque, Reason: reason}
}

// RuntimeRestorability inventories state owned by the organism runtime and its active components.
func (r *Runtime) RuntimeRestorability() RestorabilityReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runtimeRestorability()
}

func (r *Runtime) runtimeRestorability() RestorabilityReport {
	pending := ComponentRestorability{ID: "pending-work", Kind: "pending", Restorability: RestorabilityStateless}
	if len(r.pending) > 0 {
		pending.Restorability = RestorabilityOpaque
		pending.Reason = "a committed action is waiting for its environment consequence"
	}
	components := []ComponentRestorability{
		{ID: "runtime-state", Kind: "runtime", Restorability: RestorabilityExact},
		{ID: "event-spine", Kind: "cognitive-history", Restorability: RestorabilityExact},
		{ID: "runtime-rng", Kind: "randomness", Restorability: RestorabilityStateless, Reason: "runtime uses deterministic interaction identifiers and owns no RNG"},
		{ID: "workspace", Kind: "workspace", Restorability: RestorabilityReconstructable, Reason: "workspace is per-step and durable decisions remain on the event spine"},
		{ID: "replay", Kind: "replay", Restorability: RestorabilityExact, Reason: "replay execution is synchronous and its cues and memory inputs are persisted"},
		{ID: "development-routing", Kind: "development", Restorability: RestorabilityReconstructable, Reason: "development routing is not runtime-owned; its approved events remain on the spine"},
		pending,
	}
	if r.memory != nil {
		components = append(components, ComponentRestorability{ID: "memory", Kind: "memory", Restorability: RestorabilityExact})
	} else {
		components = append(components, ComponentRestorability{ID: "memory", Kind: "memory", Restorability: RestorabilityStateless})
	}
	if r.regulator != nil {
		components = append(components, ComponentRestorability{ID: "regulation", Kind: "regulation", Restorability: RestorabilityExact})
	} else {
		components = append(components, ComponentRestorability{ID: "regulation", Kind: "regulation", Restorability: RestorabilityStateless})
	}
	if r.gate != nil {
		components = append(components, r.externalComponent("eligibility-gate", "gate", "active eligibility gate has no explicit stateless or snapshot declaration"))
	}
	if r.actuator != nil {
		components = append(components, r.externalComponent("actuator", "actuator", "actuator state belongs to the embedding and has no explicit declaration"))
	}
	if r.scorer != nil {
		components = append(components, r.externalComponent("outcome-scorer", "scorer", "function-backed scorer has no explicit stateless declaration"))
	}
	if r.replaySelector != nil {
		components = append(components, r.externalComponent("replay-selector", "replay", "function-backed replay selector has no explicit stateless declaration"))
	}
	for id, learner := range r.learners {
		status := r.externalComponent("learner:"+id, "learner", "learner has no explicit runtime snapshot contract")
		if _, ok := learner.(agent.SnapshotParticipant); ok {
			status = ComponentRestorability{ID: "learner:" + id, Kind: "learner", Restorability: RestorabilityExact}
		}
		components = append(components, status)
	}
	for _, specialist := range r.specialists {
		declared := specialist.Restorability()
		components = append(components, ComponentRestorability{
			ID: declared.ID, Kind: declared.Kind,
			Restorability: Restorability(declared.Restorability), Reason: declared.Reason,
		})
	}
	sort.Slice(components, func(i, j int) bool {
		if components[i].Kind == components[j].Kind {
			return components[i].ID < components[j].ID
		}
		return components[i].Kind < components[j].Kind
	})
	report := RestorabilityReport{Exact: true, Components: components}
	for _, component := range components {
		if component.Restorability == RestorabilityOpaque {
			report.Exact = false
			break
		}
	}
	return report
}
