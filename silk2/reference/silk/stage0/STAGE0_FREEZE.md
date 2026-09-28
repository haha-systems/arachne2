# Stage 0.1 — Zig reference freeze

**PRD:** `docs/silk2/SILK_PRD.md` §10 Stage 0, item 1.

## Tag

- **Tag:** `zig-reference-v1`
- **Commit:** `8a16b6a76008ed423d60f5e4e86469b89c659b04`
- **Commit subject:** docs: add session handoff for DCHM-1 gates, Experiment 1 timeline, and the memory-game/H3 detour
- **Tagged:** 2026-09-05

This tag is the immutable reference for the Silk 2 rewrite (PRD §2): the Rust
implementation is built and validated against the behaviour captured at this
commit, via the golden corpus and traces produced in the rest of Stage 0.

## Frozen files

No further edits to these files, with the one exception below:

- `src/Lexer.zig`
- `src/Ast.zig`
- `src/Interpreter.zig` (4,976 lines — the file Silk2 exists to decouple from, PRD §2)
- `src/ModuleLoader.zig`
- `src/AstSerializer.zig`
- `src/Value.zig`

## The one permitted post-tag change

**Stage 0.3 — host-call tracing patch** (PRD §10 item 3): every builtin
dispatch emits `{seq, name, args, result|error, duration}` as JSON lines,
which becomes the Rust replay fixture format.

Per PRD §12 risk mitigation ("tag before the patch"), this patch:

- lands after this tag exists, so its diff is reviewable against this frozen
  baseline;
- is scoped to tracing only — no other behavioural change to the frozen
  files;
- must be reviewed against a no-behaviour-change diff of corpus output
  before merge.

Any other change to the six files above is out of scope until Silk 2 has
replaced them.
