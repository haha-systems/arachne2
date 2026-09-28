# Silk core language semantics

**Task:** SILK-03  
**Status:** Normative proposal for Silk 2 Phase 2  
**Scope:** Values, bindings, procedures, control flow, agent lifecycle, input/output, errors, sessions, imports, and host calls. The detailed wire schema and effect/grant rules are specified separately by SILK-05 and SILK-06.

This document defines Silk without assuming an Arachne host. A host may supply functions, but only through the session contract; Silk syntax and core evaluation do not read or mutate host internals. The frozen Zig interpreter and its corpus are evidence for selective compatibility, not the definition of Silk 2.

## Programs and execution

A program is UTF-8 Silk source parsed into declarations and executable statements. Loading a program checks its source representation and declaration structure; running it evaluates its top-level executable statements in source order. Top-level declarations create procedures or initial bindings and do not execute procedure bodies. A syntax or semantic error prevents execution of the program; runtime errors are reported at the point they occur.

Evaluation is deterministic for a fixed program, session input, and sequence of host-call results. Expressions evaluate left to right. Each procedure invocation has a local call frame. A session owns its bindings and loaded program state; separate sessions share no Silk mutable state.

## Values

The core value set is:

| Value     | Semantics                                                                                                                                        |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `null`    | A present value with no payload. It is distinct from a missing object key or unbound identifier.                                                 |
| Boolean   | `true` or `false`. Only booleans satisfy a boolean condition.                                                                                    |
| Number    | Finite signed integer or finite floating-point value. Integer arithmetic must detect overflow; non-finite results are runtime errors.            |
| String    | Immutable Unicode text. Indexing and substring conventions are defined in UTF-8 byte-independent character terms by the string builtin contract. |
| Array     | Ordered sequence of values. Indexes are zero-based.                                                                                              |
| Object    | String-keyed mapping of values. Key lookup is exact and case-sensitive. Object key order is not semantically significant.                        |
| Procedure | A Silk-defined callable, identified by its declaration within a loaded program. It is not serialized as a JSON value.                            |

Numbers, null, booleans, strings, arrays, and objects map to JSON values for protocol input/output and host calls. Serialization rejects procedures and non-finite numbers; it must not silently replace them with placeholder strings. Object keys are strings. Duplicate literal keys are rejected during parsing or semantic analysis.

Equality is type-sensitive: `1` and `1.0` compare numerically equal; values of different nonnumeric kinds are unequal. Arrays compare elementwise and objects compare by key/value membership, independent of key order. Ordering operators are defined only for numbers and strings; incomparable operands raise a typed runtime error instead of coercing.

## Bindings and state

`let name = expression` creates an immutable binding in the current lexical scope. Assignment updates a mutable binding introduced by `state` or a mutable local form defined by the language. Reassigning an immutable binding, assigning an unknown name, or assigning to a reserved session input is an error. Scope lookup walks from the innermost lexical frame outward; declarations are not implicitly global across sessions.

Arrays and objects support indexed/member assignment only when their owning binding is mutable. An out-of-range array index and a missing object member assignment are errors; object member assignment may create a new key only through the explicit object update operation defined by the language. Reading a missing object key returns `null` only if the language operation explicitly requests an optional lookup; ordinary member access reports `MissingKey`. This distinction prevents an absent field from being confused with an explicitly supplied null.

`state` declares session-local mutable storage initialized when the program is loaded for a new session. Re-loading the same program in another session creates independent state. State is not durable storage and is not a host write. A host interaction that changes data outside the current Silk session is an effectful host call.

No implicit truthiness or string/number coercion is performed. Conditions require booleans. Arithmetic operands must be numeric; `+` additionally supports two strings for concatenation. Array/string length and collection operations use named core functions rather than overloaded host behavior.

## Procedures, calls, and return values

`fn name(parameters) { ... }` declares an ordinary procedure. Parameters are immutable bindings local to the call. Argument count and runtime value constraints are checked at call time where they are not statically known. Procedures return the value of an explicit `return expression`; reaching the end without `return` returns `null`. `return` outside a procedure is invalid.

Calls resolve in this order: local procedure/binding callable, Silk core function, then a declared host function. A name that cannot be resolved raises `UnknownName`. A local binding that is not callable raises `NotCallable`. Host calls are never selected by fuzzy name matching or dynamic access to host internals.

An `act` declaration, where retained by the final syntax inventory, is an ordinary procedure with an effect classification; it does not create a separate execution engine. `yield expression` returns a yielded value from the active action/procedure invocation and terminates that invocation. Yielding outside a yield-capable invocation is a semantic error. The precise syntax disposition of legacy declarations is finalized in SILK-04; their runtime meanings here do not imply every legacy spelling is retained.

## Control flow

`if` evaluates its condition once and executes exactly one branch. Conditions must be boolean. `else` is optional. `while` reevaluates its boolean condition before every iteration. `for` iterates values in array order; its loop binding is local to each loop scope and cannot be assigned unless separately declared mutable. `break` and `continue` affect the nearest enclosing loop only; use outside a loop is an error.

Evaluation order is left-to-right for operands, call arguments, array elements, and object values. Short-circuit boolean `and` and `or` evaluate the right operand only if required. A failed expression or call stops the current procedure; it is not converted to `null` or `false`.

## Inputs and outputs

Session creation supplies a JSON input object, the loaded program, host-function declarations, protocol version, and grants. The input object is read-only to Silk and session-scoped. `task_input`, if accepted as a compatibility binding, is a named read-only view of the session input; it is not a process global or an Arachne feature.

Top-level `return` is invalid. Program completion produces the last top-level expression result when the execution mode requests expression results; otherwise it produces `null`. The CLI's output format is stable JSON on stdout. Diagnostics and structured trace events use their separately documented channels and never contaminate program output. `print` is a core effect that emits its stringified arguments to the program output channel and is also traced; it does not write to an unspecified host logger.

`args` exposes only explicitly supplied process arguments when the host enables that input. `now`, `sleep`, and other nondeterministic or externally observable core operations are runtime-controlled operations whose result/duration is recorded for replay; a replay supplies their recorded result and does not consult a live clock or wait unless the replay contract says so.

## Agent lifecycle

An agent is a host-independent Silk program that declares one `policy` procedure and may declare one `learn` procedure. The runtime drives `agent.step`; a host does not call lifecycle procedures by reaching into interpreter internals.

1. The runtime passes the policy a read-only input object containing the current stimulus and session-approved context.
2. `policy` runs once and returns a JSON-compatible decision. It may update agent-local state only as allowed by the program's declarations and language rules.
3. The runtime returns the policy decision to the host. External task assignment or execution is a separate host decision and is not implied by the policy result.
4. When the host later supplies an outcome, the runtime may invoke `learn(outcome)` if the agent declares it and the `agent.step` request asks for learning.
5. `learn` returns a JSON-compatible summary. It may update session-local agent state; every other mutation must be a declared, granted host call.
6. The runtime records lifecycle transitions, procedure results, errors, and host calls in the structured trace.

Absent `learn` means the learning step is skipped successfully. A policy or learn error is surfaced to the host with phase and procedure identity; the runtime does not retry automatically. The exact protocol messages and state reset rules belong to SILK-05.

## Imports and modules

An import names a source module through an explicit module search root supplied by the host or CLI. Paths are normalized relative paths: absolute paths, parent traversal outside the configured roots, and implicit network fetches are rejected. A module is loaded at most once per program load. Import cycles are errors. Imported procedures are namespaced by their module unless an explicit import form introduces a selected name; imports do not grant host authority.

The legacy interpreter's dynamic path fallback and global symbol extraction are compatibility observations, not defaults for Silk 2. The final accepted import forms, export rules, and canonical module syntax are decided by SILK-04 and the parser/lowering tasks.

## Host functions, effects, and authority

A host function is declared by the environment for a session with a stable name and input/output schema. Silk can invoke only a declaration present in that session. The host executes the function and returns a JSON-compatible value or a structured error. Silk does not implement the host function body.

Every host invocation crosses the versioned protocol, is ordered with respect to the calling procedure, and emits a trace event containing the resolved function name, arguments or their approved redacted form, result/error, authority decision, and call identity. A denied call fails before host dispatch. A missing declaration is `UnknownHostFunction`; a denied declared call is `AuthorityDenied`; host-reported failure is `HostError`. Effects and grants are explicit; a declaration alone is not permission. SILK-05 specifies descriptors and protocol behavior; SILK-06 specifies effect classes and grant evaluation.

Legacy LLM, MCP, memory, provenance, coordination, replay, and organism builtins are not implicit Silk core functions. A third-party host may offer generic functions under its own declared contract. Arachne-specific coordination or memory functions remain host extensions and do not change core semantics.

## Errors

Errors are structured values with a stable category, message, source span when available, procedure/call context, and optional host detail. The minimum categories are:

- `ParseError`, `InvalidDeclaration`, `DuplicateName`, `UnknownName`, `ImmutableBinding`, `TypeError`, `ArityError`, `NotCallable`;
- `MissingKey`, `IndexOutOfBounds`, `NumericOverflow`, `SerializationError`, `FuelExhausted`, `RecursionLimit`;
- `UnknownHostFunction`, `AuthorityDenied`, `HostError`, `ProtocolError`, `SessionClosed`.

An error unwinds the current call and terminates the current top-level execution or lifecycle step. Earlier host calls are not automatically rolled back. The runtime still emits procedure-exit/error and trace records for work completed before failure. No error is swallowed by a default value. Source spans use UTF-8 byte offsets internally and convert to line/column for human diagnostics.

## Runtime sessions and resource bounds

A session is one execution context bound to a program, immutable input, host connection, and grant set. The process may serve multiple sessions, but session bindings, loaded mutable state, traces, and pending calls are isolated. Closing a session rejects later operations with `SessionClosed` and releases its state.

Each execution has configurable instruction fuel, call-depth, and wall-clock limits. Exceeding an execution limit yields a structured error and trace; it does not continue in a partially valid state. A session never gains grants because another session has them. Process isolation remains a host deployment choice; Silk's session isolation is required regardless.

## Examples

### Pure procedure and explicit types

```silk
fn clamp(value, lower, upper) {
  if value < lower {
    return lower
  }
  if value > upper {
    return upper
  }
  return value
}

let result = clamp(12, 0, 10) // 10
```

### Session-local state

```silk
state attempts = 0

fn next_attempt() {
  attempts = attempts + 1
  return attempts
}
```

Two sessions executing this program each observe `1` on their first call.

### Agent lifecycle boundary

```silk
policy choose(stimulus) {
  return {accept: stimulus.priority >= 3}
}

learn record(outcome) {
  // Only agent-local declared state may be changed directly.
  return {observed_success: outcome.success}
}
```

The host supplies the stimulus and later outcome. Returning `accept: true` does not assign work or execute a tool.

### Granted host call

```silk
fn summarize(text) {
  return language.summarize({text: text})
}
```

This call succeeds only if `language.summarize` is declared and the session has the required grant. A missing declaration and a denied call produce distinct errors before any host function body runs.

## Compatibility and open follow-up

The semantics above define Silk 2, not a promise to accept all legacy syntax. SILK-04 must inventory every legacy construct and map it to retained syntax, compatibility-only syntax, or removal. The evaluator and corpus tasks must then test the required/preferred classes against pinned recordings. In particular, the existing reference Stage 0 says 84/200 programs are host-dependent and that standalone `run` does not bind `task_input` or drive the organism dispatch loop; those fixtures must not be mislabeled as pure language failures.

The exact numeric precision model, Unicode string indexing, optional object lookup spelling, accepted `act`/`yield` forms, import syntax, and lifecycle request fields need implementation-ready resolution in SILK-04/05 before the evaluator depends on them. Where this document leaves an explicit decision open, implementation must not guess silently: update this specification and add a focused compatibility fixture first.
