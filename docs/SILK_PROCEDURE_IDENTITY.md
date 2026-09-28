# Silk procedure identity and lineage

**Task:** SILK-13  
**Applies to:** semantic procedures and exported capabilities  
**Status:** design contract for registry and inspection work

## Identity layers

Silk distinguishes a procedure's lineage from an immutable revision and from
the source name used to find it in a file:

| Field               | Meaning                                                                                                                   | Changes when                                                                |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `procedure_id`      | Opaque identity for one logical procedure lineage. It is allocated once and retained by an explicitly accepted successor. | A new, unrelated procedure is created.                                      |
| `revision_digest`   | SHA-256 digest of the canonical executable semantics and declared interface for this revision.                            | Executable behavior, signature, effects, dependencies, or contract changes. |
| `name`              | Optional human-readable source name or registry alias. It is not an identity key.                                         | A declaration is renamed or an alias changes.                               |
| `capability_id`     | Opaque identity for a published semantic capability. A capability can expose one or more procedures.                      | A distinct capability is published.                                         |
| `capability_digest` | Digest of the normalized capability description, schemas, guarantees, and referenced procedure revisions.                 | Any of those declared properties changes.                                   |

The pair `(procedure_id, revision_digest)` identifies an exact revision. A
revision is immutable once published. The lineage identifier alone always
refers to a history, never to an implicit `latest` executable. Callers and
replay records that require reproducibility pin both values.

For a newly authored procedure, the creator allocates a fresh opaque ID. Silk
does not derive this ID from a name, source path, source bytes, or semantics.
An accepted edit that continues the same logical procedure retains its ID and
creates a new revision digest. A copied or independently synthesized procedure
gets a new ID and may record the source procedure as a parent. A registry may
reject an attempted ID reuse or conflicting revision; admission policy belongs
to SILK-14.

## Canonical revision content

`revision_digest` is SHA-256 over a versioned canonical encoding of:

- normalized parameter and return schemas, including order where order is
  semantically meaningful;
- canonical executable blocks, instructions, constants, and terminators;
- declared effect ceiling and exact dependency identities, expected digests,
  and interface versions;
- normalized preconditions, postconditions, and behavioral contract version.

The digest excludes source name and path, comments, whitespace, source spans,
debug labels, the `procedure_id`, lineage links, timestamps, author labels,
semantic prose, provenance, and validation evidence. Thus renaming or moving a
procedure preserves its revision digest. Changes to behavior or declared
interface create a different digest. A change to the canonical encoding rules
requires a new digest-domain version; consumers must not compare digests across
different domains as if they were equivalent.

Canonical encoding must reject non-finite numbers, normalize identifiers and
strings according to the language contract, define numeric representation,
sort maps by canonical key, and preserve ordered vectors. It must not rely on
incidental serializer output or source text. Until that encoder exists, the
parser's FNV source fingerprint is only a local compilation key; it is not a
revision digest or trusted identity. Hash implementation and validation belong
to registry/artifact implementation work.

## Semantic description and contracts

A semantic description is a human-readable summary plus normalized intent
terms. It supports inspection and retrieval, but it is not executable meaning
and is excluded from `revision_digest`. Corrections to prose can therefore be
published without pretending that executable behavior changed; the description
has its own content digest when used in an artifact.

Contracts are machine-readable interface declarations: purpose, input/output
schemas, preconditions, postconditions, effect ceiling, required authority
identifiers, and dependency constraints. Contract expressions that constrain
execution are included in `revision_digest`; explanatory prose and examples
are not. A contract claim is a condition to validate, not evidence that it is
true and not an authority grant.

## Lineage and provenance

Every revision may carry an append-only lineage record with its parent revision
references, relation (`revises`, `derived_from`, or `composed_from`), and a
short reason. A logical edit normally has one parent with relation `revises`.
Copies, generated procedures, and compositions receive new procedure IDs and
record their inputs as parents. Parent references include both ID and digest;
missing parents do not invalidate an artifact, but must be reported as
unresolved lineage.

Provenance records origin (`human_authored`, `legacy_migration`, `generated`,
or `composed`), optional source repository/ref/path and source digest, generator
identity/version, composition plan, timestamps, authorship claims, and evidence
references. These fields are inspectable assertions. They are not covered by
the executable revision digest, do not authenticate themselves, and never
confer host authority. Signed attestations may be added by a future trust
policy without changing procedure identity.

Validation evidence is a separate record keyed to the exact procedure ID and
revision digest. Each result names the validator and version, validation
profile, outcome, timestamp, and evidence digest or locator. Evidence for one
revision cannot silently certify another. Registry admission policy decides
which evidence is required; runtime authorization remains a separate session
grant check.

## Inspection and compatibility rules

Inspection must expose IDs and digests, display names, interface and effect
summary, description, parent references, provenance, and revision-scoped
validation evidence without requiring the original source. Missing optional
source or unresolved parent information is represented explicitly rather than
filled from a name.

Names may be ambiguous across modules or registries and may be changed freely;
call resolution uses an explicit procedure ID or capability reference. Dynamic
name lookup, where offered by an embedding, must resolve to a pinned revision
before execution. Trace, cache, and replay correlation should include the
program/session identity plus procedure ID and revision digest so a rename does
not break lineage and an update cannot masquerade as the recorded revision.

This document specifies identity semantics, not the wire representation or
registry acceptance policy. `silk.ir.v1` currently has only provisional string
IDs; adding persisted identity and contract fields requires a compatible schema
evolution or a new schema version before external artifacts depend on it.
