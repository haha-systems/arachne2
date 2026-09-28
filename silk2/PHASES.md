# Silk 2 phase gates

## Phase 1 — independent workspace

Deliver a source-preserving parser wrapper, placeholder runtime, minimal
protocol data types, `silk run <path>` command, and a corpus loader that reads
all Stage 0 recordings. The corpus loader's green test is a data-integrity
check, not an execution or compatibility result: it reports zero programs
passing until the evaluator exists.

Phase 1 exit requires workspace format, lint, build, and tests to pass from a
copy of this directory with no Arachne or Zig checkout available. The CLI must
execute the public path on `examples/hello.silk` and clearly identify the
placeholder behavior.

## Later decisions

- SILK-03/SILK-04 define values, declarations, grammar, types, control flow,
  imports, and compatibility syntax.
- SILK-05/SILK-06 define host descriptors, effects, grants, and runtime
  authority checks.
- SILK-07 onward define IR, evaluation, traces, inspection, and procedure
  identity/reuse.

Do not introduce a full AST, evaluator, Arachne builtin, host connection, or
semantic compatibility promise into Phase 1.
