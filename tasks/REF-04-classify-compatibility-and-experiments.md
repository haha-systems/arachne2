# REF-04 — Classify compatibility and experiments to preserve

**Status:** Done  
**Track:** Shared foundation  
**Depends on:** REF-02, REF-03  
**Source:** PRD §§6, 10, 13

## Outcome

Rewrite decisions distinguish meaningful behavior from legacy accidents and preserve important experiments.

## Work

- Classify behavior as required, desirable, legacy-only, or deliberately abandoned.
- Identify Arachne experiments that should later be reproduced.
- Link each decision to evidence or rationale and note unresolved questions.

## Completion notes

- Recorded required, desirable, legacy-only, and deliberately abandoned compatibility decisions with links to the Stage 0 matrix and current Silk contracts in [the compatibility decisions](../docs/COMPATIBILITY_DECISIONS.md).
- Identified governance, constrained adaptation, active-inference/SWR authority separation, identity memory, and measurement/replay experiments for reproduction, with claim limits and unresolved source/host questions.
- Kept Stage 0 program-level labels and host-dependence classification distinct from the rewrite-level decision categories.

## Acceptance criteria

- Compatibility decisions are reviewable and traceable to examples or research.
- No subsystem is treated as a requirement merely because it exists in Zig.
