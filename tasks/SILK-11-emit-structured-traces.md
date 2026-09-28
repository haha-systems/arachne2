# SILK-11 — Emit structured execution traces

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-09  
**Source:** PRD §7, Silk Phase 6

## Outcome

Execution produces structured events that explain what the evaluator did.

## Work

Define and emit events for procedure entry and exit, branches, host calls and results, errors, authority decisions, and effect execution. Include correlation and provenance fields.

## Acceptance criteria

- A trace can reconstruct the major steps of a run without parsing free-form log text.
- Trace events distinguish attempted, authorized, denied, and completed host calls.

## Completion notes

- Added serializable structured trace event types with stable event labels, monotonic sequence numbers, per-session trace IDs, and nested procedure call correlation IDs.
- Emits procedure entry/exit, branch, state/output effect, host attempt/authorization/denial/completion, and error events. Failed executions return their emitted trace on the structured error.
- Wired the standalone CLI JSON result to include its trace array.
- Checks: `cargo fmt --all --check`, `cargo test --workspace`, and `cargo clippy --workspace --all-targets -- -D warnings`.
- Gotchas/CES: `dispatch_host_call` combined authorization and provider invocation, preventing an accurate authorization event before host execution. Split out a pure `authorize_host_call` decision and retained `dispatch_host_call` as its safe wrapper. Traces currently include raw host arguments/results; hosts need a redaction policy before sensitive calls use them.
