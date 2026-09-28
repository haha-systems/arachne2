# AR-07 — Add memory consolidation

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-06  
**Source:** PRD §8, Arachne Phase 5

## Outcome

Past experiences can be consolidated into memory structures that influence later cognition.

## Work

Define consolidation triggers and transformations, preserve links to source episodes, and emit events for changes. Keep consolidation inspectable and reversible where practical.

## Acceptance criteria

- Consolidated knowledge can be traced to its supporting experiences.
- Later retrieval can show how consolidation changed the available memory.

## Completion notes

- Added explicit `episode.exact_repeat.v1` consolidation, configurable by an episode query and minimum support of at least two. It groups exact normalized JSON content and episode kind without inferring broader semantic claims.
- Consolidation runs preserve selected episode IDs, output pattern IDs, policy version, and status. Derived patterns retain support count, time range, tags, source episodes, and source event IDs.
- Run IDs are stable for the same rule, support policy, and source selection. Applying a run plus its patterns is one store operation; callers can retrieve by run ID and inspect revoked results.
- Added actor/reason attributed revocation that removes patterns from ordinary retrieval, and replay/memory event records for apply, revoke, and retrieval operations.
- Documented the transform, lineage, idempotency, revocation, and transaction boundary in [the consolidation contract](../docs/ARACHNE_MEMORY_CONSOLIDATION.md).

### Gotchas and CES record

- Recurrence is evidence that an exact episode representation repeated; it is not evidence of causal truth or a general semantic rule. The transform therefore stores a provenance-rich pattern and does not create or promote semantic beliefs.
- Memory persistence and event persistence are separate. A consolidation or revocation may be committed even when its later event append fails; the API returns the stable run ID/state so callers can inspect before retrying.
- Revocation hides derived patterns from ordinary lookup while retaining their run and evidence for inspection. It does not erase original episodes.
