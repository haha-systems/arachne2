# SILK-01 — Specify Silk Rust Phase 0/1

**Status:** Done  
**Track:** Silk  
**Depends on:** REF-02  
**Source:** PRD §§7, 16

## Outcome

A detailed implementation specification defines the first independent Silk foundation.

## Work

Specify the reference corpus, language inventory, Rust workspace and package boundaries, testing approach, documentation structure, and smallest independent runtime. Identify decisions that need later semantic specifications.

## Acceptance criteria

- The document defines a buildable first milestone and its exit criteria.
- It keeps Arachne-specific concepts out of Silk except as compatibility fixtures.
- It provides enough detail to start the Rust project without deciding the full future language prematurely.

## Completion notes

- Added [the Rust foundation specification](../docs/SILK_RUST_FOUNDATION.md) with workspace packages, corpus and test approach, CLI behavior, Phase 1 exit criteria, and decisions reserved for later tasks.
- Cross-checked it against the Silk PRD, frozen Stage 0 corpus/trace documents, builtin inventory, and the reference repository's existing Phase 1 plan.
- The reference repository contains no Rust workspace at `silk2/`; the specification records that the plan is not implementation evidence.
