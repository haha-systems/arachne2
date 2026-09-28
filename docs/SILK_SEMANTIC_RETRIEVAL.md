# Silk semantic capability retrieval

**Task:** SILK-15  
**API:** `silk_registry::ProcedureRegistry::retrieve`

`RetrievalQuery` separates ranking terms from hard filters. `intent` and
`preferred_terms` contribute to a deterministic lexical score. Required terms,
input/output schemas, allowed effect ceilings, available authority labels,
validation profiles, and exact dependencies filter candidates. Schemas use
exact JSON equality in this version; a future schema compatibility engine can
replace that rule without changing the explainable response shape.

Each `RetrievalCandidate` contains the full source-independent artifact and its
score, matched preferred and intent terms, and a human-readable explanation.
Candidates sort by score descending, then procedure ID and revision digest.
Optional rejection reports identify each failed filter. A query can cap result
count without changing ranking.

Retrieval only establishes compatibility with labels supplied by the caller.
`available_authorities` is a search constraint, not a Silk session grant. A
candidate's declared requirements never authorize host dispatch; the runtime
must still check the active session catalog and grant immediately before the
call.

The first implementation uses Unicode lowercase plus alphanumeric tokenization
and term overlap. It does not use embeddings, infer behavior from executable
payloads, or guarantee semantic equivalence. The score is a ranking aid, not a
confidence or safety value. Callers should inspect the returned contract,
dependencies, effects, provenance, and revision-scoped evidence before use.
