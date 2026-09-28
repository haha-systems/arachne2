# Silk reference corpus

This folder archives the Stage 0 recording set from the legacy Arachne/Silk
repository for use by both the Zig reference runner and the independent Silk
runner. The data format is host-neutral JSON; Arachne host calls occur only as
recorded function names and values, not as harness behavior.

## Contents and provenance

- `corpus/MANIFEST.json` indexes all 200 reference recordings.
- `corpus/*.recording.json` contains each recorded output, exit/timeout result,
  source SHA-256, invocation, and builtin trace.
- `corpus/SHIM_CLASSIFICATION.json` separates clean, organism-host-dependent,
  and generic-host-dependent outcomes.
- `source/` contains 56 exact Silk source snapshots whose
  bytes matched the recording SHA-256 in the current reference checkout on
  2026-09-28. Their original repository-relative paths are preserved.
- `SOURCE_AVAILABILITY.json` records source availability and compatibility class
  per manifest entry. 144 recorded program sources were not present in that
  checkout; their output/trace recordings remain valuable evidence but they are
  not source-runnable fixtures until the exact source bytes are recovered.
- `stage0/` preserves the trace, corpus, builtin, host-classification, and
  compatibility documents used to interpret the recordings.

The recording data and Stage 0 documentation were copied from the remote
`silk2` branch at commit
`f7bd485ee108a87259d28108a9c91c0147b044b9`. The source snapshots were copied
only after validating their SHA-256 against the corresponding recording.
The new Zig freeze pin is documented separately in
[`../../docs/REFERENCE_FREEZE.md`](../../docs/REFERENCE_FREEZE.md).

## Recorded inventory

| Compatibility class | Recording count | Source snapshots included |
|---|---:|---:|
| required | 33 | 21 |
| preferred | 60 | 21 |
| legacy | 20 | 14 |
| abandoned | 87 | 0 |

A manifest recording is not automatically a compatibility requirement. Use the
class in `SOURCE_AVAILABILITY.json` and the detailed rationale in
`stage0/COMPATIBILITY_MATRIX.md`. Host dependence is a separate dimension; a
program can be required and still need a host stub to execute.

## Reproduction and consumption

From a clean checkout of the legacy reference branch, the original corpus was
recorded with `zig build` followed by
`python3 tools/silk2/record_corpus.py`; Stage 0 used a five-second per-program
timeout. The no-behavior-change comparison and recording method are preserved
in `stage0/`.

A runner should load `corpus/MANIFEST.json`, resolve each `fixture` relative to
this folder, and treat output, exit code, timeout flag, and trace as expected
recorded behavior. When it needs source, resolve `program` relative to
`source/` and require `source_status=included_hash_verified` plus a fresh SHA-256
match. The loader and comparison harness must not implement Arachne builtins or
assume an Arachne daemon; host behavior is replayed from trace records or
provided by an explicit test host.
