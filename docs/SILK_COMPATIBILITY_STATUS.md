# Silk compatibility comparison status

**Task:** SILK-10  
**Comparison tool:** `silk capture` and `silk compare`.  
**Replay schema:** `silk.replay.v1`.

## Reproducible workflow

Capture a run of the supported Silk subset:

```sh
silk capture examples/hello.silk /tmp/hello-silk2.json
```

Convert the pinned Zig runner's output into the same
`silk.execution_snapshot.v1` envelope. Preserve the exact source SHA-256,
process exit code, timeout flag, stdout, program output, optional JSON result,
and optional deterministic host replay tape. Do not remove ANSI/log output
unless a documented adapter identifies it as runner diagnostics. Then compare:

```sh
silk compare /tmp/hello-zig.json /tmp/hello-silk2.json
```

The command writes a JSON `silk.comparison_report.v1` and exits 0 only when
the source hash, exit status, timeout, stdout, program output, result, and
host-replay fields match. Runtime labels are retained as provenance. Trace
arrays are diagnostic and excluded because the Zig and Rust event schemas
differ; compare host behavior through replay entries instead. A source-hash
mismatch is always a failed comparison.

For embedded runs, wrap the host provider in `RecordingProvider`, serialize
its tape, and replay it later with `ReplayProvider`. Replay validates schema
version and contiguous call order, then checks each exact function name and
argument list. It fails on missing, extra, reordered, or changed calls. The
evaluator's descriptor and grant check still runs before the replay provider,
so a fixture cannot grant authority.

## Reviewed baseline and limits

| Evidence                    | Current status                                                                                                                                                                                                                                                       |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Archived Stage 0 recordings | 200 fixtures load and pass envelope integrity checks. They have not been executed by the Rust evaluator.                                                                                                                                                             |
| Compatibility classes       | 33 required, 60 preferred, 20 legacy, and 87 abandoned recordings remain classified by the archived matrix.                                                                                                                                                          |
| Exact source snapshots      | 56 source files are available and hash-verified against their recordings; 144 source files are absent.                                                                                                                                                               |
| Required-source coverage    | 21 of 33 required files are archived, but that does not mean their runtime outcomes have been compared.                                                                                                                                                              |
| Zig version relationship    | Recordings were copied from the Silk 2 branch at `f7bd485ee108a87259d28108a9c91c0147b044b9`; the frozen Zig checkout is `ab8d9bdadebd48cb58cfeecaa100a951c47e442c`. Do not claim those recordings are outputs from the frozen commit without reproducing them there. |
| Compatibility gate          | The archived 100% required / 90% preferred gate has not been established. Current parser/runtime coverage is a documented subset.                                                                                                                                    |

These cases remain open comparisons rather than passes: missing source bytes,
recordings whose source pin is not the frozen Zig pin, host-dependent programs
without declared replay fixtures, and legacy syntax intentionally rejected by
Silk 2. The reviewed language changes are listed in
[syntax compatibility](SILK_SYNTAX_COMPATIBILITY.md) and
[language semantics](SILK_LANGUAGE_SEMANTICS.md): Arachne authority declarations
and builtins move behind explicit host descriptors and grants; organism
dispatch and persistent memory are not implicit Silk behavior. Unsupported
constructs produce source-located diagnostics.

Do not report the snapshot tool's passing unit tests as a corpus compatibility
percentage. A release gate needs snapshots produced from the pinned Zig runner,
verified source bytes, explicit host replay fixtures, and actual Silk 2 runs
for the required and preferred classes.
