# SILK-14 — Implement the procedure registry

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-13  
**Source:** PRD §7, Silk Phase 7

## Outcome

Validated procedures can be admitted, discovered, versioned, and inspected through one interface.

## Work

Implement registry admission and queries over identity, contracts, provenance, versions, and validation evidence. Define persistence boundaries without coupling the registry to Arachne.

## Acceptance criteria

- Admission checks required metadata and validation evidence.
- Registered procedures can be listed and inspected independently of their source names.

## Completion notes

- Added the standalone `silk-registry` crate with versioned artifact,
  description, contract, lineage, provenance, and validation evidence types.
- Admission computes and checks domain-separated canonical SHA-256 revision
  digests, rejects malformed metadata and evidence that is not bound to the
  exact revision, and supports configurable required validation profiles.
- Added exact revision lookup, deterministic listing, ambiguous name lookup,
  and mutable aliases that do not alter the revision digest.
- Defined a storage trait and volatile in-memory backend. Persistent stores must
  implement atomic insertion and durable alias updates; no disk backend is
  included.
- See [procedure registry](../docs/SILK_PROCEDURE_REGISTRY.md) for digest
  boundaries and trust limitations. Validation evidence is checked for shape
  and binding, not signature or truth.

### Gotchas

- Resolve and replay by both procedure ID and revision digest. Do not add an
  implicit `latest` lookup.
- Names and aliases can collide and may return multiple procedures; they are
  discovery labels only.
- Registry admission does not currently validate executable payload against
  `silk.ir.v1`; required evidence is a caller-supplied assertion, not verified
  attestation. Host permission still comes from session grants.
