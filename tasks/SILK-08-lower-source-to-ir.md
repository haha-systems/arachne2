# SILK-08 — Lower source programs into semantic IR

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-04, SILK-07  
**Source:** PRD §7, Silk Phases 1 and 4

## Outcome

Representative Silk source can be parsed, checked, and converted into the canonical IR.

## Work

Implement parsing and lowering for the supported language subset. Report errors with source locations and preserve provenance from source into IR.

## Acceptance criteria

- Trivial and representative corpus programs lower successfully or produce documented diagnostics.
- Lowering does not embed host-specific or Arachne-specific behavior in the IR.

## Completion notes

- Added the canonical `silk.ir.v1` Rust data model with procedures, CFG blocks, instructions, typed call-target categories, terminators, and source mappings.
- Implemented lexing, parsing, diagnostics, and lowering for the documented initial subset. Top-level execution becomes `proc:main`; `fn`, `policy`, and `learn` become ordinary procedure IR.
- Legacy authority declarations fail with a source location. Dotted calls are classified as host dependencies but carry no implementation or permission.
- The exact grammar and known limits are recorded in [the parser/lowering contract](../docs/SILK_PARSER_LOWERING.md).
- Checks: `cargo fmt --all --check`, `cargo test --workspace`, and `cargo clippy --workspace --all-targets -- -D warnings`.
- Gotchas/CES: the first accepted design had to preserve the Phase 1 CLI's top-level statement form by lowering it to `main`; unsupported `tool` input initially parsed as harmless name expressions, so declaration-shaped legacy forms now fail closed. The parser subset is not the full 200-program legacy corpus, and source-derived `program_id` is explicitly provisional rather than cryptographic identity.
