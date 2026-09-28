# Arachne governed replay

Replay schedules an explicit set of immutable episodic records for renewed
attention. `ScheduleReplay` validates the organism's episode IDs, normalizes
the evidence set, assigns a stable plan ID and digest, and emits a schedule
event. `RunReplay` accepts only the unchanged plan after its scheduled time,
loads those same episodes, and emits a completion event whose parents include
the schedule and source events.

Replay cues are ordered by occurrence time and episode ID. Each carries the
episode ID, kind, occurrence time, canonical content digest, and source event
IDs. The completion event records the plan digest, exact input IDs, and all cue
outputs. The digests let an inspector check the immutable source records
without copying their raw content into replay events. Re-running a plan yields
the same cue order and content digests; completion timestamps and event IDs
will differ.

Replay only offers past experiences as attention cues. It does not write
episodic or semantic memory, promote a belief, alter identity or procedure
retention, execute an external action, or grant runtime authority. An embedding
application that turns a cue into a persistent memory change or another
consequential operation must submit that operation separately to the
[governance evaluator](ARACHNE_GOVERNANCE.md) and then meet its ordinary Silk
authorization requirements. The [`governed-replay` example](../arachne/examples/governed-replay/main.go)
shows the replay output followed by a separate approved proposal that remains
unexecuted.

## Scheduling and evidence limits

The service emits schedule and completion events but does not own a durable
queue or enforce exactly-once execution. The embedding host retains the
returned plan and runs it when due. Recovery after process restart depends on a
durable event store and host logic that resolves recorded plan IDs back to
plans. The memory `FileStore` keeps source episodes immutable, which supports
repeatable results for the same plan.

This is a deterministic provenance and authority-boundary demonstration, not
evidence that replay improves learning or task performance. Existing SWR
research includes a null/noisy natural-replay versus recency-only result; this
implementation does not overturn it. See the [reference inventory](REFERENCE_INVENTORY.md)
and [compatibility decisions](COMPATIBILITY_DECISIONS.md) for the evidence
limits and required authority separation.
