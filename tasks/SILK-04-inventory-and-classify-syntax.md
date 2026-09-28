# SILK-04 — Inventory and classify legacy syntax

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-03, REF-02, REF-04  
**Source:** PRD §7, Silk Phase 2

## Outcome

Each relevant legacy syntax form has an explicit disposition in the new language.

## Work

Classify forms as retained generic syntax, syntax with new semantics, compatibility syntax, or deprecated Arachne-specific syntax. Resolve the meaning of `capability` and document migration examples.

## Completion notes

- The archived Stage 0 inventory covers observed declarations and builtin calls across the 200 corpus files, separates core functions from generic and Arachne-specific host calls, and records disposition and use counts.
- Added [syntax compatibility and migration guidance](../docs/SILK_SYNTAX_COMPATIBILITY.md), including explicit dispositions for `state`, `policy`, `learn`, `act`, `tool`, `capability`, `learning`, and memory-policy metadata.
- Resolved `capability` as host-supplied session grants rather than executable authority; documented a migration from embedded `tool`/`capability` declarations to an ordinary procedure and session descriptor/grant.
- Recorded parser details that remain implementation decisions for SILK-07/08, so legacy syntax is not silently treated as a permanent core requirement.

## Acceptance criteria

- The inventory covers syntax used by the golden corpus.
- Deprecated and compatibility forms cannot silently become permanent core requirements.
