# Stage 0.3 corpus output comparison

Status: complete

The frozen baseline `zig-reference-v1` and the patched reference were run over
the same sorted set of **200 `.silk` files**. Each program had a five-second
process timeout; the same timeout and corpus list were used for both runs.

## Result

| Check | Result |
|---|---:|
| Programs attempted | 200 |
| Baseline exit codes | 116 × 0, 44 × 1, 40 × 124 |
| Patched exit codes | 116 × 0, 44 × 1, 40 × 124 |
| Exit-code mismatches | 0 |
| Existing program-output mismatches | 0 |
| Patched trace records | 1,469 |
| Files containing trace records | 86 |
| Invalid trace records | 0 |
| Files with an incomplete sequence set | 0 |

The comparison removed only `silk.builtin_trace.v1` JSONL records from the
patched debug stream. Diagnostic stack metadata was excluded from the
program-output comparison because binary paths, addresses, and the added
tracing wrapper frame are build metadata. The resulting diff is empty.

Trace validation checked that every record has the documented fields, exactly
one of `result` or `error`, a JSON-array `args` field, a numeric non-negative
`duration`, and a complete per-interpreter sequence set `0..N-1`.

