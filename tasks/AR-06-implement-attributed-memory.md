# AR-06 — Implement attributed organism memory

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-05  
**Source:** PRD §8, Arachne Phase 5

## Outcome

Arachne can store and retrieve experiences with explicit source and context.

## Work

Implement the justified episodic and semantic memory forms, attribution, retrieval interfaces, and persistence. Keep Ghostdive separate and use the old system as behavioral evidence rather than a structural template.

## Acceptance criteria

- Stored experiences retain source, time, and relevant agent or session context.
- Retrieval returns attributable memories through explicit interfaces.

## Completion notes

- Added immutable, source-attributed episode records and sourced semantic candidate records with confidence, evidence links, contradiction links, and epistemic status.
- Added deterministic filtered retrieval interfaces and a `memory.Service` that binds operations to one organism and records write/retrieval IDs through the cognitive event spine.
- Added `InMemory` and atomic versioned JSON `FileStore` implementations. `Daemon.Memory()` exposes the service; `--memory-file` opts into persistence.
- Documented retrieval, event coupling, persistence behavior, and scale/security limits in [the attributed memory contract](../docs/ARACHNE_ATTRIBUTED_MEMORY.md).

### Gotchas and CES record

- Semantic interpretation is not the same as accepted knowledge. The service only records candidates; it exposes no promotion operation that could bypass the later governance path.
- The file store atomically replaces a snapshot and is single-process/single-writer. Multi-process locking and large-scale indexing remain future work.
- Memory writes and cognitive events use separate stores. A post-write event failure is returned explicitly, while the record remains committed; callers must inspect by ID before retrying to avoid duplicate intent.
- Text retrieval is deterministic literal matching, not semantic vector search. This keeps the initial store small and inspectable until a measured workload justifies another retrieval mechanism.
