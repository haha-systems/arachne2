# Silk2 builtin dispatch trace format

Status: Stage 0.3 reference format

The Zig reference emits one JSON object per line for every builtin dispatch.
The stream is JSON Lines (JSONL). It is written to the interpreter's debug
stream (`stderr`); it does not replace or alter the program's existing output.
Consumers can select trace records by their `schema_version` value.

## Record schema

Each record has this shape:

```json
{
  "schema_version": "silk.builtin_trace.v1",
  "seq": 0,
  "name": "len",
  "args": [[1, 2, 3]],
  "result": 3,
  "duration": 12345
}
```

Fields:

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | string | Format identifier. The current version is `silk.builtin_trace.v1`. A consumer must reject an unknown major version. |
| `seq` | unsigned integer | Zero-based dispatch sequence for one `Interpreter` instance. It increments once when a builtin call is entered. |
| `name` | string | The resolved builtin dispatch name. Direct builtins use their registered name. Dynamic calls retain their resolved form, such as `__llm__:anthropic:call` or `__mcp__:c7:get-library-docs`. |
| `args` | array | Arguments in dispatch order, encoded with Silk's JSON value representation. |
| `result` | JSON value | Present only when dispatch succeeds. It is the builtin return value. |
| `error` | string | Present only when dispatch fails. It is the Zig error name, for example `InvalidArity` or `RuntimeError`. |
| `duration` | non-negative integer | Monotonic dispatch duration in nanoseconds. It covers the builtin body and excludes trace serialization. |

Exactly one of `result` and `error` is present. `args` is always an array,
including for a zero-argument call. A record is terminated by one newline and
does not contain an envelope or a cross-record timestamp.

The sequence is local to an interpreter. A replay harness that combines more
than one interpreter stream must retain the stream identity separately; `seq`
is not a process-wide identifier. Nested dispatch can complete before its
caller, so emitted lines are completion-ordered while `seq` records entry
order. A harness must validate that the sequence set is exactly `0..N-1`, not
that file order is numerically sorted.

## Value encoding

Arguments and successful results use the existing `Value.toJson` encoding:
numbers, booleans, strings, `null`, arrays, and objects are JSON values.
Function, builtin, and tool values use the existing placeholder strings because
they are not serializable runtime objects.

## Corpus comparison

The trace is diagnostic data and is not part of the reference program output.
For the Stage 0 no-behaviour-change check, capture the reference output and
the patched output separately, remove only lines whose
`schema_version` is `silk.builtin_trace.v1` from the patched debug stream, and
compare the remaining program output bytes. If diagnostic stack metadata is
included, normalize binary paths, addresses, and the tracing wrapper frame
before comparing; those are build metadata, not program output. The
comparison must also verify that every dispatch in the patched run has a
corresponding trace record with sequence values whose set is exactly
`0..N-1`.
