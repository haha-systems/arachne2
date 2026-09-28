# Arachne attributed memory

The memory package provides two small, source-linked record forms. Episodes
preserve observed experience. Semantic records preserve sourced interpretations
as candidates with confidence, attribution, evidence links, and explicit
contradiction links. Retrieval is read-only and always returns the provenance
fields with each record.

## Records and retrieval

Each episode has an organism, stable ID, source kind and ID, occurrence and
recording times, optional agent/session/correlation attribution, a kind, JSON
content, tags, and links to source cognitive events. Semantic records carry
their source episodes/events, assertion, confidence in the inclusive range
0–1, and status. The memory service only creates `candidate` semantic records;
it has no operation that promotes candidates. Governance and any future
retention policy own that decision.

`memory.Service` assigns an ID and record time when absent, fixes organism
attribution to the configured organism, and routes writes and reads through
explicit methods. Episode queries filter by source, agent, session, correlation,
time range, tags, and literal terms. Semantic queries filter by source, agent,
session, status, and literal terms. All filters combine with AND. Results are
ordered by source time then ID and require a positive limit. Term matching is
case-insensitive substring matching; there is no embedding or inferred
similarity search.

Successful writes and retrievals emit `memory` events through the shared
cognitive spine. Those events include record IDs and source links, but omit
episode content and semantic assertion text. Storage and event emission do not
share a transaction: if event persistence fails after a successful write, the
service reports the error and the stored record remains committed. Callers
should use the returned record ID and inspect the spine before retrying.

## Persistence

`InMemory` supports ephemeral use. `FileStore` stores one organism's records in
a versioned JSON snapshot. It writes a private temporary file, syncs it, and
atomically replaces the snapshot; a failed pre-replacement write leaves the
previous snapshot intact. `arachned --memory-file PATH` selects this store;
without the flag, daemon memory is process-local. The cognitive event history
remains a separate bounded in-memory store.

The file store is intended for a single writer process. It does not provide
cross-process locking, incremental indexing, compaction, encryption, or
multi-file transaction coordination. Retrieval scans the in-memory maps and is
intended for the initial organism workload, not large-scale archival search.
Memory payloads may contain sensitive information; the file is created with
owner-only permissions, and operators remain responsible for path, backup, and
host filesystem security.

For the exact-repeat transformation and revocation path, see the
[consolidation contract](ARACHNE_MEMORY_CONSOLIDATION.md).
