# SILK-17 — Add the procedural synthesis pipeline

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-07, SILK-14  
**Source:** PRD §7, Silk Phase 9

## Outcome

Higher-level systems can submit a missing procedure as a candidate through Silk’s ordinary semantic pipeline.

## Work

Accept candidate procedures, convert them to semantic IR, and route them through validation, effect analysis, authority analysis, and execution. Keep the source of synthesis outside Silk unless separately justified.

## Acceptance criteria

- A candidate cannot bypass the same checks applied to other executable procedures.
- Candidate origin and validation results are preserved as provenance.

## Completion notes

- Added `silk-synthesis` with source candidate preparation through parser/
  lowering, effect analysis against exact host descriptors, effect ceiling and
  declared authority checks, artifact packaging, and revision-bound evidence.
- Only generated/composed origins are accepted by the default pipeline policy.
- `PreparedCandidate::admit_and_execute` requires successful registry admission
  before execution in the existing `Session`; runtime grant checks still happen
  immediately before host dispatch.
- Documented supported evidence profiles and current IR identity limits in
  [the synthesis pipeline](../docs/SILK_SYNTHESIS.md).

### Gotchas

- Configure registry validation profiles to match those requested for candidate
  preparation; the pipeline only produces syntax/lowering and effect/authority
  profiles.
- Effects are analyzed using the submitted descriptor catalog. Missing host
  declarations become `unknown` and must be covered by the declared ceiling or
  preparation fails.
- The current IR payload retains provisional internal procedure IDs/names, so
  the digest is not yet rename-stable for arbitrary source refactors.
- Evidence is pipeline-generated but unsigned; provenance and evidence do not
  grant authority or guarantee semantic safety.
