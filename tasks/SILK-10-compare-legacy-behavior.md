# SILK-10 — Compare behavior with the Zig reference

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-08, SILK-09, REF-03, REF-04  
**Source:** PRD §7, Silk Phase 5

## Outcome

Required compatibility is measured against the same corpus on the legacy and Rust implementations.

## Work

Build a comparison workflow, add versioned record/replay for nondeterministic host interactions, and document intentional incompatibilities.

## Acceptance criteria

- Required behaviors have matching observable outcomes or explicitly reviewed exceptions.
- Replay fixtures make comparisons repeatable without live external services.

## Completion notes

- Added `silk.execution_snapshot.v1` capture and `silk compare` with source SHA-256 validation, machine-readable outcome differences, and CI-friendly exit status.
- Added versioned `silk.replay.v1` recording/replay providers. Replay checks function order, exact names, and arguments; authority gates still run before replay dispatch.
- Reviewed and documented the corpus baseline, intentional Silk 2 compatibility changes, missing source coverage, and the distinction between the archived Stage 0 source pin and the frozen Zig reference.
- The historical 100% required / 90% preferred corpus gate is **not established**. The workflow reports concrete comparisons; it does not claim the current subset matches the full corpus.
- Checks: `cargo fmt --all --check`, `cargo test --workspace`, and `cargo clippy --workspace --all-targets -- -D warnings`.
- Gotchas/CES: a passing manifest integrity test only proves the 200 recording envelopes agree with their index; it does not execute any Silk program. Snapshot comparisons therefore have their own outcome schema and test gates. Trace formats remain diagnostic and are excluded from cross-runtime equality; host effects are compared through the versioned replay tape.

See [compatibility status and workflow](../docs/SILK_COMPATIBILITY_STATUS.md).
