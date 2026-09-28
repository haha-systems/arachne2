# SILK-13 — Define procedural identity and lineage

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-07  
**Source:** PRD §7, Silk Phase 7

## Outcome

Reusable procedures have stable semantic identity and explainable history.

## Work

Define identity, semantic descriptions, contracts, provenance, version lineage, and validation evidence. Clarify how source names relate to identity.

## Acceptance criteria

- Procedures can be distinguished across renames and revisions.
- Identity and provenance data can be inspected without relying on source-level names.

## Completion notes

- Defined opaque logical `procedure_id`, immutable `revision_digest`, mutable
  display name, and separate capability identity/digest in
  [procedure identity and lineage](../docs/SILK_PROCEDURE_IDENTITY.md).
- Specified canonical executable content, rename behavior, lineage parent
  references, semantic descriptions, contracts, provenance claims, and
  revision-scoped validation evidence.
- Clarified that the current FNV source fingerprint is provisional and that
  persisted identity fields require schema evolution before artifacts rely on
  them. The current parser/runtime IR does not yet implement this contract.

### Gotchas

- Never treat a name, source hash, lineage ID alone, provenance field, or
  validation record as an exact executable revision or authority grant.
- Runtime callers, traces, and replay need the revision digest pinned alongside
  the lineage ID; `latest` is not reproducible.
- Canonical digesting must version its encoding and reject unsupported numeric
  values instead of hashing incidental serializer output.
