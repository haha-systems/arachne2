package agent

// Restorability names the explicit continuation contract for a behaviourally relevant component.
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

// ComponentRestorability identifies one registered component's declared state contract.
type ComponentRestorability struct {
	ID            string        `json:"id"`
	Kind          string        `json:"kind"`
	Restorability Restorability `json:"restorability"`
	Reason        string        `json:"reason,omitempty"`
}

// Restorability reports the explicit state contract of this specialist.
func (a *SpecialistAgent) Restorability() ComponentRestorability {
	component := ComponentRestorability{ID: a.id, Kind: "specialist"}
	if participant, ok := a.implementation.(SnapshotParticipant); ok && participant.RuntimeStateKind() != "" {
		component.Restorability = RestorabilityExact
		return component
	}
	if stateless, ok := a.implementation.(StatelessSpecialist); ok && stateless.Stateless() {
		component.Restorability = RestorabilityStateless
		return component
	}
	component.Restorability = RestorabilityOpaque
	component.Reason = "specialist does not declare statelessness or implement SnapshotParticipant"
	return component
}
