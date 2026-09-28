# Silk candidate synthesis pipeline

**Task:** SILK-17  
**Crate:** `silk-synthesis`

Candidate source is supplied by an external synthesis system through
`CandidateRequest`; Silk does not generate source. `prepare_candidate` lowers
the source into semantic IR, checks the requested entry procedure, runs the
effect analyzer using the caller's exact host descriptor catalog, and rejects
effects or authority requirements that exceed the declared contract. It then
packages the IR, semantic description, contract, source digest provenance, and
revision-bound evidence in a `ProcedureArtifact`.

The pipeline currently produces only `silk.syntax_lowering.v1` and
`silk.effects_authority.v1` evidence profiles. Configure the destination
registry to require the same profiles when that gate is desired. Evidence
records prove that these pipeline checks ran; they are not signed attestations
or proof of semantic correctness.

`PreparedCandidate::admit_and_execute` admits the artifact before invoking the
candidate through the normal `Session`. The caller supplies procedure
arguments, host descriptors, grants, and provider at execution time. Each host
dispatch still passes the runtime's exact descriptor and session grant check.
Rejected candidates cannot use this API to skip admission or static effect and
authority checks.

The current IR serialization still contains provisional internal procedure
names and IDs. Source text and source spans are removed from the artifact
payload, but complete rename-stable IR canonicalization is pending the SILK-13
schema migration. The pipeline accepts the parser's supported subset only; it
does not execute candidates during preparation.
