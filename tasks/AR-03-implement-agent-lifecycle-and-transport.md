# AR-03 — Implement agent lifecycle and event transport

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-02  
**Source:** PRD §8, Arachne Phase 2

## Outcome

The runtime can supervise agents and deliver messages or events between them.

## Work

Implement agent start, stop, failure handling, and message/event transport. Add a deterministic harness for exercising lifecycle and ordering behavior.

## Acceptance criteria

- Agents can exchange events and be stopped without leaking runtime work.
- Tests can control scheduling or event input well enough for repeatable scenarios.

## Completion notes

- Added a bounded point-to-point router and supervisor in [`arachne/internal/agent/`](../arachne/internal/agent/), with fixed pre-start registration, context cancellation, failure propagation, joining, per-recipient FIFO delivery, and closed-router errors.
- Integrated the supervisor into the daemon so embedding code can register agents before `Run`; daemon shutdown waits for all workers up to the configured timeout.
- Documented message ordering, backpressure, lifecycle behavior, and the context-cooperation requirement in [the agent runtime contract](../docs/ARACHNE_AGENT_RUNTIME.md).
- Verified with Go 1.23.12: package tests cover ordered delivery, message attribution, first-error propagation, cancellation/join, and post-stop delivery rejection; `golangci-lint run` passes.

## Gotchas and CES record

- **Observation:** the first supervisor tests reported no agent error from `Stop` and allowed a send after stop. **Hypotheses:** shutdown could race a worker's first execution, and Go `select` could choose a ready mailbox send even when the close signal was also ready. **Actions:** added `Wait` to observe worker completion deterministically, retained non-cancellation failures, and rechecked the close signal after a selected send. **Outcome:** tests now wait on completion and post-stop sends return `ErrRouterClosed`.
- **Observation:** strict lint flagged the exported name `AgentFunc` as stuttering within package `agent`. **Action:** renamed the adapter to `agent.Func`. **Outcome:** strict lint passes.
