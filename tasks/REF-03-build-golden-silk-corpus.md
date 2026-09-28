# REF-03 — Build a golden Silk corpus

**Status:** Done  
**Track:** Shared foundation  
**Depends on:** REF-01, REF-02  
**Source:** PRD §§6, Silk Phase 5

## Outcome

A versioned set of representative Silk programs and fixtures can be run against both implementations.

## Work

- Select programs from existing agents, examples, and standalone Silk use.
- Capture expected outputs and relevant errors.
- Record or stub nondeterministic host interactions for repeatable runs.

## Acceptance criteria

- Each fixture has provenance and a clear expected behavior.
- The corpus can be consumed by legacy Zig and new Rust runners without Arachne-specific assumptions in the harness.

## Progress and gotchas

- Imported all 200 Stage 0 recordings, the manifest, host classification, compatibility matrix, builtin inventory, trace format, and Stage 0 comparison/freeze documents into [the versioned corpus archive](../silk2/reference/silk/README.md).
- Preserved 56 exact source snapshots after matching each file's SHA-256 to its recording. The current reference checkout no longer contains 144 historical source files (mostly generated worktree copies); the recordings remain archived but are not source-runnable fixtures until those exact sources are recovered.
- The dataset is plain manifest plus JSON recordings. It has no Arachne runtime dependency; host behavior must be supplied by explicit host stubs or replay records, never inferred by the comparison harness.
- The Rust corpus loader in SILK-02 reads the archived manifest and validates all 200 recording envelopes against it; the integration test confirms the same versioned corpus is consumable without Arachne-specific harness assumptions.
- The 144 recordings whose historical source snapshots are absent can support output/trace comparisons only where the recording contains enough expected behavior; they cannot be rerun as source fixtures until exact sources are recovered.
