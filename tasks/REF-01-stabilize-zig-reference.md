# REF-01 — Stabilize the Zig reference system

**Status:** Done  
**Track:** Shared foundation  
**Depends on:** —  
**Source:** PRD §6

## Outcome

The current Zig system remains runnable and reproducible while the rewrites proceed.

## Work

- Choose and create a stable reference tag or branch.
- Record build and run instructions, required inputs, and known environmental assumptions.
- Preserve the existing tests and document how to run them.

## Acceptance criteria

- A clean checkout can build and run the reference system using documented steps.
- The reference revision is identified unambiguously and is protected from rewrite work.

## Progress and remaining check

- Created remote tag `arachne2-start-v1` and branch `reference/arachne2-start-2026-09` at `ab8d9bdadebd48cb58cfeecaa100a951c47e442c` with Jujutsu after finding that the previously documented `zig-reference-v1` tag was absent from fetched refs.
- A clean clone of that revision passed `mise exec -- zig build`, `mise exec -- zig build test`, and `mise exec -- zig build run -- examples/demo.silk` (all exit 0). The six files listed as frozen by the historical Stage 0 note match the sibling working tree byte-for-byte.
- Reproduction details and checksums are in [the freeze record](../docs/REFERENCE_FREEZE.md).
- GitHub's branch-protection API returned HTTP 403 because the private repository's plan does not support that feature. The exact full commit and separate versioned tag provide the rewrite boundary for this task; do not move the tag. Formal branch protection is unavailable on the current plan.
