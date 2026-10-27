package organism

import "testing"

func TestRuntimeRestorabilityReportsConfiguredExactComponents(t *testing.T) {
	runtime, _, _ := newSnapshotRuntime(t, "inventory-organism")
	report := runtime.RuntimeRestorability()
	if !report.Exact {
		t.Fatalf("configured deterministic runtime is not exact: %+v", report)
	}
	for _, expected := range []string{"event-spine", "memory", "regulation", "decision", "actuator"} {
		found := false
		for _, component := range report.Components {
			if component.ID == expected {
				found = true
				if component.Restorability == RestorabilityOpaque {
					t.Errorf("component %q was marked opaque: %s", expected, component.Reason)
				}
			}
		}
		if !found {
			t.Errorf("restorability report omitted component %q", expected)
		}
	}
}
