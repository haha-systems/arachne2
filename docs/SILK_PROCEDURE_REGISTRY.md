# Silk procedure registry

**Task:** SILK-14  
**Crate:** `silk-registry`  
**Artifact schema:** `silk.procedure_artifact.v1`

## Artifact and digest

An admitted `ProcedureArtifact` contains an opaque procedure ID, immutable
revision digest, optional display name, semantic description, behavioral
contract, executable semantic payload, lineage, provenance, and validation
evidence. The payload is source-independent JSON so producers can package IR
without requiring the registry to depend on a parser or Arachne.

The crate calculates a domain-separated `sha256:` digest over canonical JSON
for executable semantics and the contract's input/output schemas,
preconditions, postconditions, effect ceiling, required authorities, and exact
dependencies. The v1 canonical encoding sorts object keys recursively,
preserves array order, uses compact JSON separators and JSON string escaping,
and uses the JSON number spelling from `serde_json::Number::to_string`; the
domain string is followed by a zero byte before the encoded payload. Producers
must normalize semantic identifiers and strings before constructing payloads.
Display names, purpose prose, descriptions, provenance, lineage,
timestamps, and validation evidence do not affect the revision digest. Schema
or canonical encoding changes require a new domain/schema version.

Admission rejects unsupported schemas, missing required metadata, malformed or
mismatched revision digests, invalid lineage/dependency digests, and absent
passing evidence for configured validation profiles. Evidence must name a
validator and version, profile, exact subject revision, outcome, and evidence
digest. The registry verifies binding and shape only; evidence remains an
assertion until a separate trust policy verifies signatures or validator
records. A procedure artifact does not grant runtime authority.

## Lookup and storage

`ProcedureRegistry` supports admission, deterministic listing, exact lookup by
`(procedure_id, revision_digest)`, name/alias lookup, and adding a display alias
without changing the immutable artifact. Name lookup may return multiple
results. There is no implicit `latest` resolution.

Storage is injected through `RegistryStore`; `MemoryStore` is the volatile
default. A persistent backend must provide atomic immutable insertion for an
exact ID/digest key and durable alias updates. Registry policy validates before
storage and does not couple to Arachne. The crate does not yet ship a disk or
network backend, capability-level registry, IR type checker, evidence
signature verifier, or semantic retrieval index.
