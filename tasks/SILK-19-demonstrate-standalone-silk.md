# SILK-19 — Demonstrate Silk as a standalone procedural computer

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-12, SILK-16  
**Source:** PRD §7, Silk Phase 10

## Outcome

A user can request an outcome from an application that retrieves, composes, executes, and explains Silk procedures without Arachne.

## Work

Choose a domain supported by real Silk host capabilities, build the application flow, and document its setup and limits.

## Acceptance criteria

- The demonstration uses Silk independently of Arachne.
- It explains the selected procedures, execution, effects, and result.

## Completion notes

- Added an executable standalone inventory example that prepares two candidate
  procedures, admits them, retrieves by intent and exact schema, composes a
  pinned plan, and executes through the Silk session runtime.
- The local host provider implements availability reads and stock reservation;
  explicit descriptors and grants produce authorization and execution traces.
- The example prints selection explanations, exact revision IDs, declared
  effects/authorities, trace categories, and the final reservation result.
- Setup and limitations are documented in the
  [standalone demonstration](../docs/SILK_STANDALONE_DEMO.md).

### Gotchas

- Inventory is in-memory and resets on each run; this is not a persistent host
  service.
- Procedure artifacts and schemas are hard-coded in the example. Retrieval is
  lexical and composition uses exact schemas.
- The trace records what this run did; it does not prove behavior for all
  possible inputs.
