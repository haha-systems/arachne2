# Silk 2 builtin inventory

**Status:** Stage 0.2 draft host-contract baseline
**Reference:** `src/Interpreter.zig` at `zig-reference-v1` (`8a16b6a76008ed423d60f5e4e86469b89c659b04`)
**PRD:** `docs/silk2/SILK_PRD.md` §§8.2, 10, 13

This document is the Stage 0 host-contract draft. It separates language
semantics from host authority and resolves PRD §13 question 1 for the legacy
surface.

## Method and scope

- The frozen interpreter has **52 direct string comparisons** in `callBuiltin`
  (including `episode_inspect`), plus five `workspace_*` names handled by
  prefix and two dynamic declaration paths (`__llm__:*` and `__mcp__:*`).
- The inventory has **66 rows**. It includes the PRD's conceptual rows that
  are not callable names in the frozen interpreter (`text`, `memory_layer`,
  `memory_policy`, index assignment, and set accumulation), plus the five
  workspace host functions discovered in the dispatch path.
- Usage counts are executable Silk call/reference occurrences after removing
  comments and string literals. `contract_net_*` counts include both
  `contract_net.*` and its `cnp.*` alias. Provider rows report declarations and
  dynamic calls where those are different surfaces. A row marked “syntax” is
  not a callable builtin and therefore has no call count.
- The filesystem contains 200 `.silk` files when all repository roots are
  included: `agents/` 83, `examples/` 8, `test_modules/` 5, `runs/` 5,
  `.claude/` worktrees 87, `experiments/` 8, and `packets/` 4. The four roots
  named in the task contain 101 files, not 200. Counts below use all 200 files
  to preserve the confirmed 200-file corpus number; worktree copies are not
  treated as independent agents in the lifecycle table (§4).
- “Host” means the Rust runtime exposes a protocol call and the session must
  grant it. It does not mean the name is implemented in the Silk evaluator.
  “Keep” means the semantics belong in Silk 2 core. “Drop” means the legacy
  syntax or surface is not carried forward.

## Inventory

The last two columns are populated for the builtin calls used by a `policy` or
`learn` body. The complete declaration-by-declaration evidence is in §4.

| Builtin name | Class | Corpus usage | Disposition | Rationale | Grant? | Policy/learning agents | Policy/learning state + host calls |
|---|---|---:|---|---|:---:|---|---|
| `len` | Language core | 72 | Keep | Length of arrays, strings, and objects. | No | — | — |
| `str` | Language core | 0 | Keep | Canonical value-to-string conversion; absent from current corpus but required by the core table. | No | — | — |
| `append` | Language core | 1 | Keep | Immutable array extension used by the set-accumulation idiom. | No | — | — |
| `contains` | Language core | 9 | Keep | Scalar membership over arrays. | No | — | — |
| `strings_contains` | Language core | 8 | Keep | Namespaced substring search. | No | — | — |
| `strings_index_of` | Language core | 19 | Keep | Namespaced string index lookup. | No | — | — |
| `strings_substring` | Language core | 21 | Keep | Namespaced half-open substring extraction. | No | — | — |
| `strings_split` | Language core | 9 | Keep | Namespaced delimiter split. | No | — | — |
| `text` | Language core | 0 (syntax) | Keep as a core value/type, not a callable builtin | The PRD names `text`, but the frozen interpreter does not register or dispatch `text(...)`. Silk 2 should define its value/type meaning without inventing a legacy call. | No | — | — |
| `generate_id` | Language core | 12 | Keep | Timestamp-based identifier helper; deterministic replacement can be host supplied during replay. | No | — | — |
| `now` | Language core | 33 | Keep | Current millisecond timestamp. It is nondeterministic and must be traced/replayable, but remains core surface syntax. | No | — | — |
| `sleep` | Language core | 114 | Keep | Duration wait and dispatch pumping in the reference runtime. Silk 2 keeps the call shape; hosts/runtime define scheduling semantics. | No | — | — |
| `print` | Language core | 279 | Keep | Sole legacy stdout/logging primitive. Silk 2 should trace the effect. | No | — | — |
| `args` | Language core | 6 | Keep | Process argument access. | No | — | — |
| `task_input` | Language core | 232 references | Keep as a read-only host binding | Installed by the host before execution and protected from assignment. It is a binding, not a callable function. | No (binding) | — | — |
| index assignment (`a[i] = x`, `o.k = x`) | Language core | syntax | Keep | Mutable array/object index updates are interpreter syntax, not a host grant. | No | — | — |
| set accumulation (`seen = append(seen, x)` guarded by `contains`) | Language core | syntax | Keep as an idiom | There is no set builtin. The corpus uses ordinary state, `contains`, and immutable `append`; preserve the semantics, not a new primitive. | No | — | — |
| `anthropic` | Generic host function | 21 declarations / 15 dynamic calls | Host | LLM provider declaration lowers to `__llm__:*`; any provider implementation may satisfy the generic invocation contract. | Yes | — | — |
| `gemini` | Generic host function | 0 | Host | Provider path exists in the interpreter but has no corpus declaration. Keep as a generic protocol provider, not Silk syntax with Arachne knowledge. | Yes | — | — |
| `tools` (MCP) | Generic host function | 7 declarations / 11 dynamic calls | Host | MCP declarations lower to `__mcp__:*`; tool names and methods are host descriptors, not interpreter builtins. | Yes | — | — |
| `memory_query` | Generic host function | 17 | Host | Legacy identity-memory query surface. Silk 2 keeps only a generic ranked/query contract; retrieval authority belongs to the host. | Yes | — | — |
| `memory_retrieve` | Generic host function | 1 | Host | Ranked host-bound retrieval. The host supplies visibility and policy receipts; Silk does not own the memory store. | Yes | — | — |
| `memory_observe` | Generic host function | 9 | Host | Append an observation through the host memory runtime. The host enforces write mode and safety counters. | Yes | — | — |
| `provenance_record` | Generic host function | 12 | Host | Provenance mutation is an external effect and must be a granted protocol call. | Yes | — | — |
| `provenance_last` | Generic host function | 6 | Host | Reads host-owned provenance state. | Yes | — | — |
| `provenance_count` | Generic host function | 6 | Host | Reads host-owned provenance state. | Yes | — | — |
| `active_inference_reset` | Arachne-specific host function | 15 | Host | Resets the reference active-inference store; not language semantics. | Yes | — | — |
| `active_inference_declare_belief` | Arachne-specific host function | 23 | Host | Writes a bounded local Episodic belief record. | Yes | — | — |
| `active_inference_record_prediction` | Arachne-specific host function | 18 | Host | Writes a prediction record in the active-inference runtime. | Yes | — | — |
| `active_inference_record_observation` | Arachne-specific host function | 16 | Host | Records observed outcome data and is called by every corpus learn block. | Yes | L1–L7 | Local Episodic belief confidence/promotion state; `active_inference.record_observation` in the learn sequence. |
| `active_inference_compute_prediction_error` | Arachne-specific host function | 16 | Host | Computes a prediction error in the active-inference runtime and is called by every corpus learn block. | Yes | L1–L7 | Local Episodic belief confidence/promotion state; `active_inference.compute_prediction_error` in the learn sequence. |
| `active_inference_apply_update` | Arachne-specific host function | 0 | Host | Stateful prediction update exists in the reference but is not used by a corpus learn block. | Yes | — | — |
| `active_inference_learn_contract` | Arachne-specific host function | 16 | Host | Declares bounds, allowed surfaces, and state touched by learning; it is a grant-bearing host call, not lifecycle syntax. | Yes | L1–L7 | `belief.confidence`, `belief.promotion_status`; `active_inference.learn_contract`. |
| `active_inference_apply_learn_update` | Arachne-specific host function | 16 | Host | Applies the bounded learning update after the contract, observation, and error calls. | Yes | L1–L7 | Local Episodic belief confidence/promotion state; `active_inference.apply_learn_update`. |
| `active_inference_inspect` | Arachne-specific host function | 13 | Host | Returns active-inference state for inspection; it is not introspection syntax. | Yes | — | — |
| `active_inference_safety_counters` | Arachne-specific host function | 13 | Host | Reads protected-mutation counters. | Yes | — | — |
| `active_inference_semantic_candidate` | Arachne-specific host function | 3 | Host | Produces a candidate semantic record under host governance; no promotion authority is granted to Silk. | Yes | — | — |
| `contract_net_reset` | Arachne-specific host function | 0 | Host | Resets the reference contract-net runtime. | Yes | — | — |
| `contract_net_register_agent` | Arachne-specific host function | 3 | Host | Registers an agent and capabilities in the host auction runtime. | Yes | — | — |
| `contract_net_announce` | Arachne-specific host function | 44 | Host | Creates an auction and emits a CNP event. | Yes | — | — |
| `contract_net_submit_bid` | Arachne-specific host function | 44 | Host | Submits a bid and may emit a CNP event. | Yes | — | — |
| `contract_net_record_bid` | Arachne-specific host function | 5 | Host | Records a bid in host auction state without making bidding language syntax. | Yes | — | — |
| `contract_net_close` | Arachne-specific host function | 8 | Host | Closes an auction and returns advisory selection data. | Yes | — | — |
| `contract_net_assign` | Arachne-specific host function | 32 | Host | Assigns work and can invoke a host-registered callback. | Yes | — | — |
| `contract_net_complete` | Arachne-specific host function | 87 | Host | Completes an assignment, emits an event, and ends dispatch in the reference. | Yes | — | — |
| `contract_net_stats` | Arachne-specific host function | 0 | Host | Reads host auction statistics; no corpus usage. | Yes | — | — |
| `contract_net_on_assignment` | Arachne-specific host function | 3 | Host | Registers a callback in host runtime state. | Yes | — | — |
| `contract_net_on_complete` | Arachne-specific host function | 3 | Host | Registers a callback in host runtime state. | Yes | — | — |
| `swr_trace` | Arachne-specific host function | 7 | Host | Records a sharp-wave-ripple signal in the Arachne replay lane. | Yes | — | — |
| `swr_ripple_tag` | Arachne-specific host function | 7 | Host | Creates a replay/ripple tag in host state. | Yes | — | — |
| `swr_pending_delta` | Arachne-specific host function | 7 | Host | Records a pending delta signal; it does not apply a delta. | Yes | — | — |
| `swr_schedule_replay` | Arachne-specific host function | 10 | Host | Schedules host replay work subject to allostatic and governance constraints. | Yes | — | — |
| `swr_continuity` | Arachne-specific host function | 7 | Host | Reads replay continuity state. | Yes | — | — |
| `swr_safety_counters` | Arachne-specific host function | 7 | Host | Reads protected replay counters. | Yes | — | — |
| `organism_propose_delta` | Arachne-specific host function | 12 | Host | Submits source to `GovernedDeltaGate`; it never directly applies a rewrite. | Yes | — | — |
| `organism_agent_id` | Arachne-specific host function | 50 | Host | Reads authenticated resident-agent identity. | Yes | — | — |
| `organism_self_source` | Arachne-specific host function | 2 | Host | Reads the live resident-agent source. | Yes | — | — |
| `governance_evaluate` | Arachne-specific host function | 6 | Host | Evaluates a protected change proposal using Arachne governance state. | Yes | — | — |
| `episode_inspect` | Arachne-specific host function | 0 | Host | Workspace-bound episode metadata; dispatched through the workspace binding, not a language primitive. | Yes | — | — |
| `memory_layer` | Arachne-specific host function | 0 (descriptor field) | Host | A memory-layer selector/configuration field, not a callable in the frozen interpreter. Silk 2 exposes it only through host schemas. | Yes | — | — |
| `memory_policy` | Arachne-specific host function | 0 (descriptor field) | Host | A memory-policy selector/configuration surface, not a callable in the frozen interpreter. Policy remains host authority. | Yes | — | — |
| `workspace_propose_candidate` | Arachne-specific host function | 27 | Host | Persists a candidate through the authenticated workspace binding. | Yes | — | — |
| `workspace_inspect_current` | Arachne-specific host function | 0 | Host | Reads current episode candidates through the workspace binding. | Yes | — | — |
| `workspace_inspect_history` | Arachne-specific host function | 0 | Host | Reads candidate history through the workspace binding. | Yes | — | — |
| `workspace_await_broadcast` | Arachne-specific host function | 29 | Host | Reads the next unacknowledged workspace view. | Yes | — | — |
| `workspace_acknowledge_broadcast` | Arachne-specific host function | 37 | Host | Acknowledges delivery and resulting candidates through the workspace binding. | Yes | — | — |

### Grant rule

Every row marked **Grant? Yes** is an authority-bearing host interaction. A
Silk 2 runtime must not implement its body as interpreter syntax, even when the
reference body calls a local Zig runtime directly. In particular:

- `active_inference_*` bodies reach the active-inference store and must be
  granted as bounded Episodic operations.
- `contract_net_*` and `swr_*` bodies reach Arachne coordination/replay state
  and may emit or schedule events.
- `organism_*`, `governance_evaluate`, `episode_inspect`, `memory_*`,
  `provenance_*`, and `workspace_*` reach host-owned identity, governance,
  memory, provenance, episode, or workspace state.
- LLM and MCP declarations lower to dynamic host calls. Their provider/tool
  descriptors and arguments are session-granted data.

The grant is checked before dispatch and is traced. A denied call is a
structured `AuthorityDenied` event, not an undefined builtin error.

## Syntax disposition and PRD §13 question 1

| Legacy surface | Silk 2 disposition | Decision |
|---|---|---|
| `state` | Keep | Session/agent-local binding. It may be changed by the agent lifecycle, subject to the language rules. It is not a host write by itself. |
| `action` | Drop as a keyword; keep the concept | An action is an ordinary procedure with an effect class, as required by PRD §8.1 FR-L4. The legacy keyword is `act`, not `action`; no separate `action` builtin is carried forward. |
| `tool` | Drop as executable syntax; replace with host declaration | Host functions are declared in `session.create` using MCP-shaped descriptors. A call requires a grant. Legacy `tool ... using ...` is a compatibility input only. |
| `capability` | Drop as executable syntax; replace with grants | A capability declaration is not language authority. The host supplies the session grant set and enforces it before dispatch. |
| `policy` | Keep | It is an agent lifecycle declaration. The corpus has no actual `policy` declaration, but the PRD explicitly retains the construct. |
| `learning` | Drop the spelling; use `learn` | The corpus has no `learning` declaration. The seven legacy declarations use `learn`; Silk 2 names the lifecycle procedure `learn`. |

This resolves the open question: **`state` and `policy` survive as Silk
syntax; `learn` survives as the lifecycle spelling; `action`/`tool`/`capability`
do not survive as authority-bearing syntax.** Actions become effect-classed
procedures, and tools/capabilities become host declarations plus grants.

## Policy and learning declarations

No actual `policy` or `learning` declaration occurs in the 200-file filesystem
corpus. There are **seven `learn` blocks in six canonical files**. The
`.claude/worktrees` copies repeat nine declarations and are excluded from this
table so the same agent is not counted as a new agent.

| ID | Agent/declaration | State touched or read | Host functions called |
|---|---|---|---|
| L1 | `agents/active_inference_adaptive_routing_organism.silk` — `learn reconcile_route(outcome)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `outcome` fields; no direct Silk state write. | `active_inference.learn_contract`, `active_inference.record_observation`, `active_inference.compute_prediction_error`, `active_inference.apply_learn_update` |
| L2 | `agents/agw_1/br1/belief_revision_response_actor.silk` — `learn revise_belief(io)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `io`; no direct Silk state write. | Same four active-inference calls as L1 |
| L3 | `agents/agw_1/br1/belief_revision_response_actor.silk` — `learn reinforce_belief(io)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `io`; no direct Silk state write. | Same four active-inference calls as L1 |
| L4 | `agents/episodic_loop_smoke.silk` — `learn reconcile_smoke(outcome)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `belief` and `outcome`; no direct Silk state write. | Same four active-inference calls as L1 |
| L5 | `agents/organism/adaptive_router.silk` — `learn reconcile_route(outcome)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `outcome`; no direct Silk state write. | Same four active-inference calls as L1 |
| L6 | `examples/active_inference_episodic_substrate.silk` — `learn reconcile_substrate_episode(outcome)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `substrate_episode`, `belief`, `prediction`, and `outcome`; no direct Silk state write. | Same four active-inference calls as L1 |
| L7 | `examples/active_inference_minimal.silk` — `learn reconcile_minimal(outcome)` | Declares host state `belief.confidence`, `belief.promotion_status`; reads `belief`, `prediction`, and `outcome`; no direct Silk state write. | Same four active-inference calls as L1 |

The repeated sequence is significant for the lifecycle design: `learn` is
Silk-defined orchestration, while the mutation authority is in the granted
`active_inference.*` host calls. The `state_touched` field is a host contract
declaration, not permission to write Semantic memory, Canonical memory,
governance state, routing, identity, credentials, or user configuration.

## Consequences for Stage 0.3 and Phase 2

1. Stage 0.3 tracing must trace both direct builtin names and dynamic
   `__llm__:*` / `__mcp__:*` calls, with the normalized host name in the event.
2. The Rust evaluator implements only the Language core rows. Generic and
   Arachne-specific rows are protocol calls selected from session descriptors.
3. The corpus shim can stub every host row by normalized name. Programs using
   Arachne-specific rows remain host-dependent integration fixtures unless the
   shim supplies a recording.
4. The lifecycle model must allow `policy` and `learn` to orchestrate granted
   calls while restricting learning mutation to agent-local Episodic state.
