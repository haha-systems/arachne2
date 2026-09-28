# SILK-16 — Compose and validate procedures

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-15, SILK-06  
**Source:** PRD §7, Silk Phase 8

## Outcome

Silk can combine existing procedures into a validated plan for a new task.

## Work

Build deterministic graph composition, check contracts and dependencies, and validate effects and authority before execution. Keep generated plans inspectable.

## Acceptance criteria

- A task can be satisfied by composing registered procedures when their contracts fit.
- Invalid or unauthorized compositions are rejected before execution with actionable reasons.

## Completion notes

- Added bounded breadth-first schema graph composition over exact registered
  revisions, selecting the shortest compatible linear chain deterministically.
- The plan pins every procedure ID and revision digest, includes the aggregate
  effects and authority requirements, lineage, validations, and a plan digest.
- Composition rejects mismatched schemas, cumulative effect overflow,
  unavailable authority labels, missing revision-bound validation profiles,
  and unresolved exact procedure dependencies before any execution.
- See [procedure composition](../docs/SILK_COMPOSITION.md) for search bounds
  and current contract limitations.

### Gotchas

- Schema compatibility is exact JSON equality; contracts' pre/postcondition
  expressions are not executed or proven by this composer.
- Plans are proposals and are not executable/admitted artifacts. Consumers must
  validate and authorize execution through the normal runtime path.
- A declared authority requirement matching `available_authorities` is not a
  runtime grant; session authorization remains mandatory.
