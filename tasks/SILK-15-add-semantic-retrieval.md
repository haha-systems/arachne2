# SILK-15 — Add semantic capability retrieval

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-14  
**Source:** PRD §7, Silk Phase 8

## Outcome

Callers can find candidate procedures by what they do and what they require.

## Work

Define capability queries and candidate ranking inputs. Return inspectable matches with their contracts, dependencies, effects, authority, and validation evidence.

## Acceptance criteria

- Retrieval returns explainable candidates rather than opaque names or scores alone.
- Callers can filter candidates by compatibility and authority constraints.

## Completion notes

- Added deterministic lexical retrieval to `silk-registry`, separating ranking
  inputs from hard compatibility filters.
- Candidates include the complete artifact, score, matched terms, and reasons;
  filtering covers exact input/output schemas, effects, declared authority
  requirements, validation profiles, and exact dependencies.
- Added optional explainable rejection details and stable score/ID/digest sort
  order. See [semantic retrieval](../docs/SILK_SEMANTIC_RETRIEVAL.md).

### Gotchas

- The score measures lexical overlap only; it is neither confidence nor a
  semantic equivalence proof.
- Schema compatibility is exact JSON equality in this version.
- `available_authorities` filters candidates but does not grant runtime
  permission. The normal session grant check still controls host dispatch.
