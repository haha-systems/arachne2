# Stage 0.4 — golden corpus recordings

**PRD:** `docs/silk2/SILK_PRD.md` §10 Stage 0, item 4; FR-R4.
**Status:** complete — 200/200 programs recorded.

## Fixture location

Recordings live under `docs/silk2/corpus/`, alongside the other Stage 0
artifacts (`BUILTIN_INVENTORY.md`, `TRACE_FORMAT.md`, `STAGE0_3_CORPUS_OUTPUT_DIFF.md`).
A separate top-level `fixtures/` tree was considered and rejected: Stage 0 has
no Rust workspace yet to anchor a crate-relative fixtures directory (that
choice belongs to Phase 1, `silk-corpus`), and every other Stage 0 deliverable
is already documentation-adjacent under `docs/silk2/`. When Phase 1 stands up
`silk-corpus`, it can point at this directory (or the corpus can move then,
once there is an actual consumer to design the path for).

One file per program, per the task requirement. The `.silk` corpus spans
several repository roots — `agents/`, `examples/`, `test_modules/`, `runs/`,
`experiments/`, `packets/`, and the `.claude/worktrees/*` copies documented in
`BUILTIN_INVENTORY.md` — so file names are the program's repo-relative path
with `/` replaced by `__`, e.g.:

- `agents/leader.silk` → `docs/silk2/corpus/agents__leader.silk.recording.json`
- `.claude/worktrees/organism-arc/agents/leader.silk` →
  `docs/silk2/corpus/.claude__worktrees__organism-arc__agents__leader.silk.recording.json`

This keeps the corpus a flat directory (no nested dot-directories a glob might
skip) while remaining collision-free across roots that share basenames.

`docs/silk2/corpus/MANIFEST.json` indexes all 200 entries: program path,
fixture file, source SHA-256, exit code, timeout flag, duration, trace record
count, and whether the trace's sequence set is complete. It is the entry
point for Phase 1's corpus harness ("loads all recordings and reports 0/N
passing") rather than a directory listing.

## Recording format

Each `<program>.recording.json` is a `silk.corpus_recording.v1` envelope
around the `silk.builtin_trace.v1` records defined in `TRACE_FORMAT.md`:

```json
{
  "schema_version": "silk.corpus_recording.v1",
  "program": "agents/cnp_leader.silk",
  "source_sha256": "<sha256 of the .silk source at recording time>",
  "invocation": {
    "binary": "zig-out/bin/silk",
    "args": ["run", "agents/cnp_leader.silk"],
    "cwd": ".",
    "timeout_s": 5.0
  },
  "result": {
    "exit_code": 0,
    "timed_out": false,
    "duration_ms": 42.405
  },
  "stdout": "",
  "output": "registered: ok\nbid: accepted\n...\ninfo: Program executed successfully",
  "trace": [
    { "schema_version": "silk.builtin_trace.v1", "seq": 0, "name": "...", "args": [...], "result": ..., "duration": 6292 },
    ...
  ],
  "trace_sequence_complete": true
}
```

- `source_sha256` pins the exact program text a recording was made against,
  so a later divergence between corpus and source is detectable instead of
  silently replaying against stale content.
- `output` is the program's own output (interpreter `print` calls and the
  CLI's info/error reporter lines) with every `silk.builtin_trace.v1` line
  removed — the same stripping rule Stage 0.3 used for its no-behaviour-change
  diff. Both streams share stderr in the reference interpreter (`print` and
  the error reporter both go through `std.debug.print`, which always targets
  stderr), so recordings capture `stdout` separately even though it is empty
  for every program in this corpus.
- `trace_sequence_complete` records whether the per-interpreter sequence set
  is exactly `0..N-1`, per `TRACE_FORMAT.md`'s validation rule. It is `true`
  for all 200 programs.
- `result.exit_code` is `null` and `result.timed_out` is `true` for a program
  that hit the timeout; its `trace` still contains whatever dispatches
  completed before the interpreter was killed.

## Recording method

`tools/silk2/record_corpus.py` runs `zig-out/bin/silk run <file>` once per
program (repo-relative `find . -iname "*.silk"`, sorted — the same list
Stage 0.3 validated: 200 files, spanning the roots above), with a five-second
per-process timeout, matching the Stage 0.3 methodology exactly so this
recording run is checked against a run that was already independently
validated for no-behaviour-change against the frozen baseline.

```
zig build
python3 tools/silk2/record_corpus.py
```

Each program is run exactly once — "recorded once" (FR-R4) refers to the
single deterministic pass per program, not to deduplicating repeated calls
within a program's own execution (a program that calls `now()` in a loop
gets one trace record per call, each with its own `seq`).

### Why this corpus is deterministic without live hosts

Stage 0.4 does not require live LLM/MCP endpoints or credentials. In this
environment, no `.silk` program has network access, so a program that reaches
a live host call has exactly one of two deterministic outcomes: it fails fast
(caught before any I/O) or it blocks on the attempt until the five-second
timeout kills the process. Both are stable across runs, which is exactly what
made this reproducible against Stage 0.3's independently-recorded exit codes.

## Result

Running the recorder over all 200 `.silk` files reproduced Stage 0.3's
corpus-output-diff numbers exactly:

| Check | Result |
|---|---:|
| Programs recorded | 200 / 200 |
| Exit code 0 | 116 |
| Exit code 1 | 44 |
| Timed out (5s) | 40 |
| Total trace records | 1,469 |
| Files with ≥1 trace record | 86 |
| Files with an incomplete sequence set | 0 |

All 200 programs have a recording file, satisfying the Stage 0 exit criterion
("N programs with recordings", PRD §10) for item 4. The 114 programs with
zero trace records are not a gap: they declare `capability`/`state`/`fn`/
`handle` bodies at the top level without a top-level call that invokes them
(the reference interpreter's `run` mode only executes top-level statements,
not `handle` blocks — those require the dispatch/organism runtime, which is
out of scope for this corpus). Their recording still has an empty `trace: []`
and the program's actual (empty) output, which is itself a valid, replayable
fixture: "no host calls occurred" is a real outcome Rust must reproduce.

Programs that time out still have a recording: `result.timed_out` is `true`,
`result.exit_code` is `null`, and `trace` holds whatever builtin dispatches
completed before the kill signal. Stage 0.5 (corpus shim / host-dependent
classification, out of scope here) is where these get triaged — some may be
legitimately host-dependent (excluded from Silk's gate per PRD item 5),
others may just need their trailing host call stubbed from this recording.

## Out of scope here

This is PRD item 4 only. Item 5 (stub organism builtins to the recording and
mark host-dependent programs) and item 6 (the `required`/`preferred`/
`legacy`/`abandoned` compatibility matrix) are separate Stage 0 steps and are
not addressed by this corpus.
