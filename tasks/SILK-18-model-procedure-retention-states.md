# SILK-18 — Model procedure retention states

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-17, SILK-14  
**Source:** PRD §7, Silk Phase 9

## Outcome

Silk distinguishes temporary procedures from candidates and retained knowledge.

## Work

Implement lifecycle states for ephemeral, candidate, and retained procedures, with explicit admission into the registry and lineage from candidate to retained version.

## Acceptance criteria

- Execution does not implicitly retain a generated procedure.
- Retention is an explicit, inspectable action with validation evidence and provenance.

## Completion notes

- Added `Ephemeral`, `Candidate`, and `Retained` lifecycle states. Prepared
  candidates are ephemeral; successful registry admission starts them as
  candidates, including when execution follows admission.
- Added explicit candidate-to-retained transitions requiring an actor, reason,
  and passing `silk.retention_approval.v1` evidence bound to the exact revision.
- Retention does not mutate the immutable procedure artifact or digest. The
  registry exposes current state and append-only transition history.
- Documented persistence atomicity and evidence trust limits in
  [retention lifecycle](../docs/SILK_RETENTION.md).

### Gotchas

- Execution and admission alone never mark a procedure retained.
- Each revision has its own state and needs its own approval; a successor does
  not inherit a parent's retained state.
- Retention evidence is only shape-checked. It is not a verified signature and
  does not grant execution authority.
