# AR-16 — Build developmental visualization and inspection

**Status:** Done
**Track:** Arachne  
**Depends on:** AR-14, AR-15  
**Source:** PRD §8, Arachne Phase 12

## Outcome

A human can inspect how an organism changed and why.

## Work

Expose acquired procedures, memory and specialist changes, regulation, structural changes, governance history, and divergence between organisms through a coherent inspection surface.

## Acceptance criteria

- The interface can answer why the organism differs from its initial state.
- Each displayed change links to its provenance and governing events.

## Completion

- Added a read-only inspection package that groups event-spine records into
  memory, specialist, regulation, governance, development, structure, and
  procedure activities. Reports preserve payloads, event parents, transitive
  provenance IDs, governance IDs, and Silk trace references.
- Added same-organism baseline comparison and cross-organism report comparison.
  The runnable example demonstrates initial-to-current changes and explains
  divergent routing profiles with their experience and approval events.
- Wired the Silk acquisition example through inspection to show candidate
  admission, retention, trace, and reuse events linked to the retention approval.
- Documented the API and its evidence limits in
  [the inspection contract](../docs/ARACHNE_INSPECTION.md).
- Gotchas: the inspector scans the configured event history and cannot expand
  IDs whose events are no longer available. Current specialist and structural
  support is observational; Arachne does not yet implement specialist learning
  or topology mutation. The default event store is bounded and volatile.
