# Stage 0.5 — corpus shim and host-dependent classification

**PRD:** `docs/silk2/SILK_PRD.md` §10 Stage 0, item 5; §12.
**Status:** complete — 200/200 programs classified.

## What "stubbed" already means for this corpus

Every Arachne-specific host builtin the frozen interpreter implements
(`active_inference_*`, `contract_net_*`, `swr_*`, `organism_*`,
`governance_evaluate`, `episode_inspect`, `workspace_*`) reaches in-process
Zig state on the `Interpreter` struct itself — `self.contract_runtime`,
the active-inference store, the SWR replay lane, `OrganismDeltaQueue`, an
optional `self.workspace_binding` — never a socket, subprocess, or network
call. The Stage 0.4 golden corpus (`docs/silk2/corpus/*.recording.json`)
was recorded with `zig-out/bin/silk run <file>`, which never starts
`OrganismHost`'s dispatch loop and never binds a workspace API. So the
Stage 0.4 recording environment already *is* "organism builtins stubbed to
whatever this process has locally, with no live daemon behind it" — item 5
does not require a second execution pass. It requires reading what already
happened in that recording and attributing every non-clean result to a
cause.

`tools/silk2/corpus_shim.py` does that attribution and writes
`docs/silk2/corpus/SHIM_CLASSIFICATION.json` (one entry per program, plus a
summary block).

## Classification rule

A program is **stub-sufficient** when its Stage 0.4 recording completed
cleanly (`exit_code == 0`, not timed out) — by construction, whatever
organism state it touched was satisfied by the in-process stub it actually
ran against.

A program is **host-dependent** when it did not complete cleanly, with the
reason attributed by inspecting the recording:

| Reason | Programs | What it means |
|---|---:|---|
| `task_input_unbound` | 41 | The program references `task_input`, a binding only `OrganismHost.zig`'s dispatch runtime installs before executing an agent's task. Plain `silk run` never binds it, so `evalExpression` raises `UndefinedVariable` before any builtin dispatch — every one of these 41 recordings has an empty or near-empty trace and the literal string `undefined identifier: 'task_input'` in its output. |
| `dispatch_timeout` | 40 | The process hit the 5s harness timeout. Every timeout recording has >=1 completed trace record (31 end in `sleep`, 6 in `print`, 3 in `contract_net_announce`) — the program made normal progress and then blocked on the *next* call. `sleep`'s own body pumps `self.dispatch_fn` while waiting; CNP announce/bid sequences wait for a counterpart's message. Neither arrives without `OrganismHost`/`Dispatch` actually driving the loop, so the program blocks until killed. |
| `generic_host_unavailable` | 3 | A *generic* host function (never Arachne-specific) errored — all 3 are `__mcp__:c7:get-library-docs`, an MCP tool whose subprocess (`Failed to init MCP client: error.FileNotFound`) does not exist in this environment. This is a real host dependency, but on a protocol any host can implement (LLM/MCP), not evidence of Arachne-organism coupling. |

`task_input_unbound` and `dispatch_timeout` are **organism** reasons: the
program cannot complete without live Arachne state (a bound task input, or
an actual dispatch loop driving forward progress). `generic_host_unavailable`
is counted in the overall host-dependent total — the program still cannot
complete under this stub — but reported as its own bucket since it is not
an organism-specific dependency; a working MCP host (real or a Phase 3 test
double) would resolve it independently of anything Arachne-specific.

No recorded trace ever shows an `active_inference_*` / `contract_net_*` /
`swr_*` call itself failing (0 error records across 1,469 trace records for
those families — the only 3 error records in the whole corpus are the MCP
case above), and `organism_*`, `governance_evaluate`, `episode_inspect`, and
`workspace_*` never appear as trace records at all in this corpus: every
corpus reference to them lives inside a `handle`/`policy`/`learn` body that
`run` mode's top-level-only execution never invokes (the same reason 114
files have an empty trace, per `STAGE0_4_GOLDEN_CORPUS.md`).

## Result

| Check | Result |
|---|---:|
| Programs classified | 200 / 200 |
| Stub-sufficient | 116 (58.0%) |
| Host-dependent | 84 (42.0%) |
| — organism-specific (`task_input_unbound` + `dispatch_timeout`) | 81 (40.5%) |
| — generic-host-only (`generic_host_unavailable`) | 3 (1.5%) |
| Stub-sufficient programs that exercised >=1 organism builtin | 31 |
| Stub-sufficient programs that used no organism builtin at all | 85 |

This confirms PRD §12's anticipated risk — "`host-dependent` programs are a
large fraction of the corpus" — and the specified mitigation applies as
written: accept it, report the number here, and hand these 84 programs to
Arachne 2 as integration tests rather than trying to force them through
Silk's own gate. The 116 stub-sufficient programs (including the 31 that
genuinely exercise `active_inference_*` / `contract_net_*` / `swr_*` and
completed cleanly against nothing but in-process interpreter state) are
Silk's actual Stage 0 gate corpus going into Phase 1.

## Out of scope here

Item 6, the `required` / `preferred` / `legacy` / `abandoned` compatibility
matrix, is a separate Stage 0 step and is not addressed by this
classification. A program's `host_dependent` flag here is orthogonal to
that matrix: a host-dependent program can still be `required` (as an
Arachne 2 integration fixture) or `abandoned`, and a stub-sufficient program
is not automatically `required` either.
