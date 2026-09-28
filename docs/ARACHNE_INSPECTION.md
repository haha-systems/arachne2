# Arachne developmental inspection

The `internal/inspection` package builds read-only reports from the cognitive
event spine. `Inspector.Inspect` returns ordered activities for one organism,
grouped as procedure, memory, specialist, regulation, structural, governance, or
development events. Each activity keeps its source event and payload, direct
parents, transitive provenance event IDs, and any governing decision event IDs.
Procedure activities also preserve Silk trace references when the source event
contains one.

The report covers current event producers: memory recording and consolidation,
governance decisions, approved routing changes, specialist proposals,
regulation, Silk registry admission and retention boundary events, and retained
procedure calls. Structural governance decisions and explicitly marked
structural development events are surfaced under the structural category. The
inspector describes emitted history; it does not infer a state transition from
an approval alone.

`CompareReports` compares reports gathered from independent organism event
stores. It pairs equivalent activities after removing event, organism,
interaction, provenance, and timestamp identifiers, then returns the shared
activities and the records found on only one side. The original reports remain
attached so each difference can be followed back to its evidence and governance
events. Payload fields that describe substantive values remain part of the
comparison.

`ChangesSince` compares an initial report with a later report for the same
organism and returns the activities emitted after the initial report's event
sequence. This answers what changed during an organism's own development while
keeping earlier evidence available through each change's provenance links.

Run the [`developmental-inspection` example](../arachne/examples/developmental-inspection/main.go)
to see two instances with the same initial routing profile diverge after
different evidence-backed experiences. It prints each side's routing state,
change summaries, and provenance links. The
[`silk-acquisition` example](../arachne/examples/silk-acquisition/main.go) also
uses the inspector to display retained procedure admission, reuse, and trace
activities.

This is a Go API that returns JSON-serializable report data; it does not ship a
web UI. Current event stores are bounded and process-local, so reports and
transitive links are only as durable as the host's configured event store.
Unresolved references remain visible by ID but cannot be expanded. Arachne
currently implements governed routing development, not specialist learning or
structural mutation, so those categories describe proposal or governance events
until corresponding state-changing features emit their own development events.
