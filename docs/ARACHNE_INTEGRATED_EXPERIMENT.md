# Integrated Arachne and Silk engineering experiment

This finite experiment exercises one engineering interaction across Arachne's
perception, memory, specialist, workspace, regulation, replay, governance,
development, and Silk interfaces. It estimates the combined replacement cost of
two measured parts. The report includes every cognitive event and the
inspection timeline, so the run can be followed from the initial observations
through action and developmental change.

## Run

Build the Silk CLI, then run both experiment outcomes from the Arachne module:

```sh
cd silk2
cargo build --workspace
cd ../arachne
go run ./examples/integrated-engineering ../silk2/target/debug/silk > success.json
go run ./examples/integrated-engineering ../silk2/target/debug/silk --fail-task > failure.json
```

The normal run deliberately tries one malformed candidate first. Silk rejects
that source during preparation without registry admission; Arachne records the
failure and continues with a valid fixed-source candidate. The `--fail-task`
run prepares and retains a syntactically valid subtraction procedure, invokes
it on the same inputs, and records that the result does not satisfy the task.
Both paths return complete JSON reports instead of losing the event history at
the failure point.

## Captured results

The checked-in [successful run](evidence/AR-17-integrated-success.json) and
[failed run](evidence/AR-17-integrated-failure.json) each contain the raw event
sequence, specialist/workspace/regulation/replay results, baseline and final
routing profiles, task outcome, and provenance-linked inspection report.

- Both runs record two prior episodes and consolidate one recurring sum
  pattern. Replay returns two cues. Planner and critic contribute independently;
  regulation marks the interaction `strained` and reduces the workspace to one
  proposal. The planner is selected by the declared confidence rule.
- The malformed candidate is rejected at `candidate.prepare` and is recorded as
  a failed path. No registry state is created for that attempt.
- On success, Silk admits and retains `estimate.sum`, returns `17.0`, and emits
  trace events. Governance approves the exact procedure revision and a later
  routing change. The route weight moves from `0.5` to `0.9`.
- With `--fail-task`, the retained subtraction procedure returns `8.0` where
  the task expects `17`. The report records the wrong result and failure event;
  the routing weight remains at its initial `0.5` because no developmental
  proposal is applied after the failed action.

The successful capture contains 31 cognitive events and 19 inspection
activities. The failure capture contains 29 events and 18 inspection activities and preserves the failure
path, Silk traces, and unchanged routing profile. The inspector links procedure
reuse through the trace and retention events to the governing decision, and
links the applied routing change to the selected proposal, replay, and action
evidence.

## Evidence limits

This experiment checks integration and observability, not general reasoning or
learning quality. The specialists are deterministic functions, the candidate
source is selected by the example rather than generated from episodes, the
operator approval is synthetic, and the arithmetic task is deliberately small.
The `--fail-task` mode is a controlled wrong-answer case; it does not measure
reliability under arbitrary faults. The Silk registry and cognitive event store
are process-local, so the captures preserve the run but do not demonstrate
restart recovery. See the [inspection contract](ARACHNE_INSPECTION.md) and the
[acquisition contract](ARACHNE_SILK_ACQUISITION.md) for their respective
durability and trust boundaries.
