# AR-14 — Implement provenance-bearing development

**Status:** Done
**Track:** Arachne  
**Depends on:** AR-07, AR-09, AR-11, AR-13  
**Source:** PRD §8, Arachne Phase 10

## Outcome

Experience can create governed, inspectable differences in an organism’s memory, routing, salience, specialization, procedures, or justified topology.

## Work

Implement selected developmental changes with explicit proposal, approval where required, application, and provenance. Start with changes justified by the architecture specification.

## Acceptance criteria

- Two initially equivalent instances can diverge after different experiences.
- Every persistent change has provenance and an observable event history.

## Completion

- Added a governed routing-profile change engine. Proposals bind an instance,
  route key, expected revision, prior value, proposed value, and evidence;
  stale proposals are rejected before evaluation.
- The profile changes only after an approved governance decision and a
  development event append. Change records link governance, source, and
  evidence events. Engines reconstruct their profile from the initial state
  and matching development events.
- The runnable [development example](../arachne/examples/development/main.go)
  shows initially equivalent instances diverging from different evidence and
  recovering those differences from event history.
- Limitation: durable recovery requires a durable cognitive event store and
  the same host-supplied initial profile. The default daemon event store is
  process-local; this experiment does not claim improved performance or
  generally safe adaptation.
