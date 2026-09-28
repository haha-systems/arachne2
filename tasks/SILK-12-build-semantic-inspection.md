# SILK-12 — Build semantic program inspection

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-07, SILK-11  
**Source:** PRD §7, Silk Phase 6

## Outcome

Users can understand a program’s purpose, dependencies, effects, authority, and execution history through Silk interfaces.

## Work

Provide inspection views over semantic IR and execution traces. Answer the documented questions: what a program does, can affect, depends on, requires, and did.

## Acceptance criteria

- A reviewer unfamiliar with the program can identify its major behavior without reconstructing it from source.
- Inspection uses semantic data and structured traces.

## Completion notes

- Added `silk inspect <source.silk|snapshot.json>` with a versioned JSON
  report for static IR blocks/operations/terminators, direct effects, procedure and host
  dependencies, and captured execution outputs/results/traces.
- Static inspection compiles to semantic IR without executing source. Snapshot
  inspection preserves the ordered structured trace and provides event counts.
- Documented the current subset limits in
  [semantic inspection](../docs/SILK_INSPECTION.md): no active host catalog,
  contracts/descriptions absent from IR, and procedure effects are direct only.

### Gotchas

- Host calls are marked `unknown`; without a session descriptor catalog, do not
  claim a required authority or permission.
- A captured trace is runtime evidence for that snapshot, not a static proof of
  all possible behavior.
- `program_id` remains the parser's provisional source key; use source SHA-256
  for the inspected bytes until SILK-13 identity fields are implemented.
