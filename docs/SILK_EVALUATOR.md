# Silk IR evaluator

**Task:** SILK-09  
**Implementation:** [silk-runtime](../silk2/crates/silk-runtime/src/evaluator.rs).  
**CLI:** `silk run <path>` compiles source to `silk.ir.v1` and writes one JSON result object.

## Execution API

`Session` runs a named procedure with JSON arguments, a per-session state map, an instruction-fuel budget, and a maximum call depth. Each call receives a new local frame; session state remains shared between calls on the same `Session`. `Runtime::execute` is the standalone CLI path and starts a fresh session for the source's synthetic `main` procedure.

The evaluator walks basic blocks directly. It evaluates constants, locals, session state, arrays, objects, arithmetic, comparisons, short-circuit boolean operations, calls, member/index access, and iterator operations. `if`, `while`, `for`, `break`, `continue`, `return`, and `yield` use CFG terminators. `print` records output lines; the CLI serializes those lines, the return value, and structured trace events into one JSON object.

Before execution, the runtime rejects unsupported schema versions, invalid procedure/block IDs, missing entry/successor blocks, and duplicate result IDs. Runtime errors carry a stable kind, procedure/block/instruction context, source span when available, and trace events emitted before failure. Fuel exhaustion and recursion limits stop execution with structured errors.

Trace events have monotonically increasing sequence numbers, deterministic per-session run IDs, nested call correlation IDs, procedure/block/instruction context, and JSON detail. Events cover procedure entry/exit, branches, effects, host call attempts and decisions, completion, and errors. Denied host calls have attempt and denial events but no authorization or completion event.

## Host calls

The caller provides a `HostFunctionProvider`, exact function descriptors, and session grants. Every `host_function` call passes through `dispatch_host_call` immediately before provider invocation. Missing descriptors return `UnknownHostFunction`; missing or insufficient grants return `AuthorityDenied`; only an authorized call invokes the provider. Provider failures return `HostError`.

## Current limits

- `now` and `sleep` are recognized core names but fail with `UnavailableCoreFunction`; the replay-aware runtime provider is not implemented.
- The CLI currently executes only `main` with an empty input object, host catalog, and grant set. Embedders can invoke named procedures and supply host services through `Session`.
- Program load initialization and top-level statements are represented by `main`; an embedder that invokes a named procedure directly must arrange any required program initialization.
- Session-state slots are keyed by their source binding name, so independently scoped state declarations with the same name are not distinguished yet.
- The runtime's effect analyzer still accepts a generic graph input; extraction of effect facts from lowered IR and validation of all instruction operands remain follow-up work.
- Numeric source literals preserve integer versus floating-point values. Integer `+`, `-`, `*`, `%`, and unary negation detect overflow. Division currently uses floating-point arithmetic.
- Trace redaction policy, runtime contracts, host input/output schemas, replay, and full 200-program compatibility remain later work.
