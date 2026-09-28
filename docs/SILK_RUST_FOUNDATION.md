# Silk Rust foundation specification

**Task:** SILK-01  
**Milestone:** Phase 1 — independent workspace and trivial execution path  
**Inputs:** [Silk 2 PRD](../../arachne/docs/silk2/SILK_PRD.md), [Stage 0 corpus](../../arachne/docs/silk2/STAGE0_4_GOLDEN_CORPUS.md), [builtin inventory](../../arachne/docs/silk2/BUILTIN_INVENTORY.md), [trace format](../../arachne/docs/silk2/TRACE_FORMAT.md), and the [existing Phase 1 implementation plan](../../arachne/docs/superpowers/plans/2026-09-05-silk2-phase-1.md).

## Goal and scope

Create the smallest buildable Silk Rust workspace that can read a source file, expose it through a source-preserving program value, return a deterministic placeholder execution result, and load every Stage 0 corpus recording. This phase proves project independence and the command/data plumbing. It does not define Silk semantics or claim that any corpus program executes correctly.

Silk must build, format, and run tests without an Arachne checkout. Do not add Arachne dependencies, organism state, agent scheduling, governance policy, memory stores, or Arachne-specific builtin implementations. Legacy Arachne calls may occur in corpus fixtures as data only.

## Workspace layout and package boundaries

Create the new Silk repository with a Cargo workspace:

```text
Cargo.toml
rust-toolchain.toml                 # stable toolchain channel
crates/
  silk-syntax/                       # source text and Phase 1 parse wrapper
  silk-runtime/                      # deterministic Phase 1 execution result
  silk-protocol/                     # versioned protocol value types only
  silk-cli/                          # `silk run` binary and CLI integration tests
  silk-corpus/                       # non-published, test-only reference loader
reference/
  corpus/MANIFEST.json
  corpus/*.recording.json
docs/
  README.md
  PHASES.md
```

All packages use version `0.1.0`, Rust edition 2024, and explicit workspace dependencies. Keep `silk-corpus` `publish = false` and use it only as a `silk-cli` dev-dependency. The workspace has no path dependencies outside its own repository. Phase 1 dependencies are limited to `serde` and `serde_json` for decoding the checked-in corpus and protocol primitives; use the standard library for argument parsing and file I/O.

| Package         | Public Phase 1 surface                                                                                      | Must not own                                                              |
| --------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| `silk-syntax`   | `Program` holding original source text; `parse(&str) -> Result<Program, ParseError>`                        | Evaluation, host calls, Arachne declarations or policy                    |
| `silk-runtime`  | `Runtime::execute(&Program) -> ExecutionResult`; stable result for the placeholder evaluator                | Parsing, transport, organism state                                        |
| `silk-protocol` | Protocol version constants and minimal JSON-RPC request/response value types                                | Socket/stdio server, execution semantics, Arachne-specific message schema |
| `silk-cli`      | `silk run <path>`; reads UTF-8 source, invokes syntax and runtime packages, prints the result               | Corpus fixture definitions or hidden dependence on an Arachne executable  |
| `silk-corpus`   | Deserialize manifest and `silk.corpus_recording.v1`; validate referenced fixture presence and schema fields | Runtime behavior, language parsing, compatibility judgments               |

Crate APIs are intentionally provisional where later phase documents own semantics. Avoid adding an AST, evaluator, host registry, IR, or application-wide abstraction before its task specifies the contract.

## Corpus and reference language inventory

Use the frozen Stage 0 recordings as versioned input data. Import the manifest and all referenced recordings into `reference/corpus/` without rewriting the envelopes. Keep each program path, source hash, invocation, result, stdout/output, builtin trace, and trace-completeness field intact. Preserve the source checkout identity alongside the archive: `zig-reference-v1` at commit `8a16b6a76008ed423d60f5e4e86469b89c659b04`.

Phase 1's corpus test loads every manifest row, opens every recording, and validates envelope/schema and required fields. Its baseline report is deliberately `0/N passing (N failing)` because the Phase 1 executor is a placeholder. This is a successful loader gate, not a semantic compatibility score. Phase 4 will apply the separate compatibility classifications: all `required` fixtures and at least 90% of `preferred` fixtures.

The legacy inventory currently records 66 builtin/syntax rows, 200 `.silk` programs, and 200 corpus classifications. It separates Silk-core constructs, generic host calls, and Arachne-specific host calls. Keep that inventory as evidence for later semantics and authority decisions; Phase 1 does not implement those calls. The corpus includes duplicate worktree snapshots and host-dependent programs; don't silently trim or reinterpret those records during import. Later tasks decide which fixtures gate the runtime.

At this phase, `parse` only creates the source-preserving wrapper and may reject only invalid UTF-8 or another explicitly documented input-representation failure. It must not pretend to understand the legacy language. The PRD's full syntax and behavior contracts are specified in later language, host, effect/authority, and IR tasks.

## CLI behavior

`silk run <path>` is the only required command. It must:

1. Require exactly one path argument and return a nonzero exit code with a concise usage error when it is absent or malformed.
2. Read the file and report a concise path-aware error for I/O or UTF-8 failures.
3. Pass the source through `silk_syntax::parse` and the public `silk_runtime::Runtime::execute` API.
4. Print a stable, deterministic Phase 1 result and exit successfully for a readable source file.

The result must not claim that the source program's statements ran. `inspect`, `serve`, builtin dispatch, external tools, and full corpus replay are later milestones.

## Test and quality approach

- Unit tests cover source preservation, deterministic runtime output, and protocol JSON round-trips for the minimal versioned structs.
- CLI integration tests create a temporary `.silk` file and exercise the public `silk run` command, including missing-argument and missing-file errors.
- The corpus integration test deserializes all manifest entries and validates all recording envelopes. It emits the explicit `0/N` baseline without failing solely because runtime behavior is not implemented yet.
- CI runs `cargo fmt --all --check`, `cargo build --workspace`, and `cargo test --workspace -- --nocapture` from a clean temporary copy containing only the Silk workspace and its corpus. This demonstrates independence from Zig and Arachne source at build/test time.
- No network, live LLM/MCP, credentials, or host service is required by the Phase 1 tests.

## Documentation

The repository README documents toolchain setup, workspace commands, and `silk run <path>`. A short phase roadmap links requirements to their later semantic tasks. Document the Stage 0 reference revision, corpus provenance, meaning of the `0/N` report, and the boundary between Silk and its eventual host. Keep the corpus schema in its existing versioned documentation; do not copy a prose-only or lossy version into code comments.

## Phase 1 exit criteria

The milestone is complete only when all of these are true:

- The Cargo workspace builds and tests in a clean environment without an Arachne checkout.
- `silk run hello.silk` reads a small fixture and prints a deterministic Phase 1 execution result.
- The public CLI tests cover success and representative input errors.
- The corpus loader reads every recording named by the manifest, validates its envelope, and reports `0/N passing` by design.
- `silk-corpus` is test-only and is not publishable; no crate has an external Arachne path dependency.
- README and phase docs explain what the result and corpus count do and do not mean.

## Decisions reserved for later specifications

- Lexical, syntactic, type, scope, evaluation, errors, serialization, and module-loading semantics (SILK-03).
- Legacy syntax disposition and corpus language classification (SILK-04).
- Host-function schema, protocol transport, session lifecycle, and compatibility (SILK-05 and SILK-08 through SILK-10).
- Effect classes, grants, denial behavior, and authority trace requirements (SILK-06).
- Semantic IR and lowering boundaries (SILK-07 and SILK-08).
- Trace and inspection output beyond the frozen reference record format (SILK-11 and SILK-12).
- Whether future registry, synthesis, retention, VM, WASM, or embedding work is warranted; these are not Phase 1 dependencies.

## Implementation notes

The source reference checkout has a Phase 1 plan and a `silk2` branch with Stage 0 documentation/corpus, but no Rust workspace or `Cargo.toml` was found on that branch. Treat the plan as design evidence, not proof the Rust workspace has been built. The current Arachne 2 checkout contains only task/specification scaffolding; the actual new Rust workspace belongs in SILK-02 after the independent project and reference-preservation prerequisites are in place.
