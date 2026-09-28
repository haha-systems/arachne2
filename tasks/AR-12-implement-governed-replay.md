# AR-12 — Implement governed replay and consolidation

**Status:** Done
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

## Completion

- Added explicit scheduled replay over immutable episode IDs. The schedule and
  completion events bind a stable plan digest to an ordered, reproducible set
  of attention cues and source evidence.
- Replay returns cues only. The runnable
  [governed replay example](../arachne/examples/governed-replay/main.go)
  submits a separate consequential proposal to governance and leaves it
  unexecuted.
- Documented scheduler durability and exactly-once limits, and retained the
  existing null/noisy replay learning evidence in
  [the replay contract](../docs/ARACHNE_GOVERNED_REPLAY.md).
