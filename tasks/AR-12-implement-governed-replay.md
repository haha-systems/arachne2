# AR-12 — Implement governed replay and consolidation

**Status:** Backlog  
**Track:** Arachne  
**Depends on:** AR-07, AR-13  
**Source:** PRD §8, Arachne Phase 8

## Outcome

Past experiences can influence future attention or candidates through controlled replay without gaining authority silently.

## Work

Use prior SWR findings to schedule and run replay, record its inputs and outputs, and route consequential changes through governance. Replay may shape candidates but cannot bypass permission checks.

## Acceptance criteria

- Replay is observable and reproducible from recorded inputs.
- Replay cannot grant authority or perform governed mutations on its own.

