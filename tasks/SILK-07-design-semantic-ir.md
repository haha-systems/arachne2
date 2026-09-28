# SILK-07 — Design the semantic IR

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-03, SILK-05, SILK-06  
**Source:** PRD §7, Silk Phase 4

## Outcome

Silk has a canonical semantic representation for human-written and generated procedures.

## Work

Design procedures, capabilities, control flow, dependencies, effects, authority, contracts, provenance, and identity. Define validation and versioning expectations without making the IR a copy of the parser AST.

## Acceptance criteria

- Source syntax is one input to the IR, not its definition.
- The representation supports inspection and future construction without source text.

## Completion notes

- Designed a versioned CFG-style semantic representation in [the Silk semantic IR contract](../docs/SILK_SEMANTIC_IR.md), including procedures, semantic capability contracts, values/instructions, control-flow terminators, dependencies, effects/authority, optional source maps, and provenance.
- Specified that source lowering and generated procedures produce the same IR, while parser AST and source text remain optional inputs/metadata.
- Defined deterministic structural/effect/contract validation, runtime grant checks at dispatch, and clear follow-on ownership for lowering, evaluation, tracing, inspection, and identity.
- Kept bytecode and content-address identity decisions deferred to evidence and SILK-13; the example is illustrative until the `silk-ir` crate freezes exact field encoding.
