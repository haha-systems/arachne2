# AR-05 — Implement the cognitive event spine

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-03, AR-04  
**Source:** PRD §8, Arachne Phase 4

## Outcome

Major cognitive events use a common, observable event model.

## Work

Define event identity, time/order, agent and session attribution, and links to Silk traces. Emit perception, activation, proposal, selection, decision, action, prediction error, replay, regulation, governance, and development events as those features arrive.

## Acceptance criteria

- A complete early agent interaction can be reconstructed from structured events.
- Later subsystems can extend the event model without private tracing paths.

## Completion notes

- Added a versioned event envelope with stable organism-scoped IDs, authoritative sequence ordering, UTC timestamps, agent/session/correlation attribution, parent links, extensible kinds, and Silk trace references.
- Added a bounded append/read memory store that reports capacity exhaustion without silent eviction. `Daemon.Events()` exposes a shared spine with a 10,000-event default.
- Router messages receive correlation IDs. When configured with the shared spine, `SilkProcedureAgent` records perception, activation, each Silk trace, and the resulting action or error as a causally linked chain.
- Documented payload privacy, ordering, storage, and persistence boundaries in [the event model](../docs/ARACHNE_COGNITIVE_EVENTS.md).

### Gotchas and CES record

- Silk traces can contain host arguments and results, so the event model preserves raw trace payloads and documents the need for host-side redaction around sensitive values.
- The bounded store returns a hard error at capacity; the agent surfaces event persistence failures rather than proceeding with an incomplete record. The in-memory daemon store does not recover sequence state across restarts.
- Proposal, selection, decision, prediction-error, replay, regulation, and governance kinds are available for later work; this task wires the complete current Silk procedure interaction and daemon lifecycle, not future subsystems that do not yet exist.
