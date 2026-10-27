# Organism Runtime State Inventory (PR4)

This inventory describes behavior-affecting state at the PR2/PR3 baseline, before snapshot support. It distinguishes the organism runtime from embedding-owned services and the Nursery environment. A checkpoint is exact only when required state can be exported and restored into independent dependencies.

| State | Owner | Classification | PR4 handling |
|---|---|---|---|
| Organism ID, workspace bounds, registered specialist IDs and declared capabilities | Runtime/config | Required for behavioural continuity and restore compatibility | Include stable identity/config manifest; reject organism or specialist manifest mismatch. Keep actuator, gate and callback implementations outside the snapshot. |
| Runtime step counter and accumulated generic conditions | Runtime | Required | Serialize and restore. Conditions are copied into later activations. |
| Event sequence and existing cognitive events | Cognition spine/store | Required causal history | Export ordered events and sequence; import into a fresh branch store. Preserve parent event IDs. Assign new continuation event identities to each branch and record parent checkpoint lineage. |
| Attributed episodes, semantic records, consolidation runs and patterns | Memory store | Required for memory retrieval/consolidation | Serialize as sorted plain-data collections, including provenance and revocation state. Restore without re-emitting memory events. |
| Learnable specialist weights, bias, RNG state, update count and last update | Learnable specialist | Required | Add validated deep-copy restore. Do not retain references into the source runtime. |
| Other specialist state | Specialist implementation/provider | Required if it can change later proposals; otherwise stateless | Require a plain-data snapshot/restore participant for mutable implementations. Treat opaque or external inference state as nondeterministic; reject exact restoration when required state is absent. |
| Proposal ID randomness | Specialist agent | Currently impossible to replay exactly; IDs also participate in deterministic tie-breaking | Generate runtime proposal IDs deterministically from branch-independent interaction and specialist identity. |
| Runtime/commitment RNG | Runtime | None at baseline | No separate runtime RNG exists. Preserve this finding in tests and inventory. |
| Regulator condition map and immutable policy | Regulator | Conditions required; policy is configuration | Serialize conditions; compare policy identity/config supplied at restore. Regulation and prediction-error events are reconstructed from the event history. |
| Replay schedules and completed runs | Memory/event spine | Reconstructable from event history; current runtime schedules and executes synchronously | Preserve replay and cue events plus memory records. There is no pending replay queue at step boundaries. |
| Development/routing profile and revision | Development engine | Reconstructable from development events; runtime does not own the engine | Preserve its causal events. Hosts must rebuild the engine from the same event spine and initial profile. |
| Workspace runs, proposals and selections | Per-step workspace | Ephemeral between completed steps | Recreated for each step; its durable event records remain in the spine. There is no cross-step queue. |
| Pending messages/work | Runtime/specialists | None between synchronous completed steps; a deferred Nursery action is transient during a tick | Complete the consequence before checkpointing. Reject a snapshot while a deferred outcome is pending. |
| Silk procedure program/session values, counters, callbacks and process state | Silk client/session | Impossible to reproduce exactly through the current public client API | Do not serialize callbacks, credentials, processes or pointers. A specialist using non-exportable session state must be reported nondeterministic and cannot claim exact replay. |
| Gate, actuator, scorer, replay selector, external provider and clocks | Embedding environment | Not organism-owned; may be nondeterministic or stateful | Never serialize executable capabilities, opaque provider configuration/state, or ambient secrets. Re-supply dependencies on restore; omission of provider configuration/state marks the checkpoint nondeterministic. |
| Nursery world RNG, world state, tick and environment event counter | Nursery | Required environment state, separate from organism cognition | Existing world checkpoint already stores this state; verify deep-copy isolation across forks. |

## Snapshot modes and current contracts

`Runtime.RuntimeRestorability` returns one explicit report for active components. A snapshot of an active gate, actuator, scorer, replay selector, or other embedding-owned component is exact only when its configuration declares it `stateless` or `reconstructable`; undeclared components are `opaque`. Those declarations are stored in the runtime's component inventory and compared on restore. Exact stateful specialist and learner contracts export their model data through `SnapshotParticipant`; registered learnable models include configuration, parameters, RNG state, update count, and last update.

`SnapshotState` is exact and fails closed. `SnapshotBestEffortState` exports available state with mode `best-effort` and opaque blockers; `RestoreState` rejects it. Nursery checkpoints record `mode`, `exact_restoration`, and the component report. `Fork` accepts only an exact checkpoint. `CreateBestEffortCheckpoint` archives available state, and cannot be passed to the exact fork API.

An exact-restorable checkpoint and a deterministic continuation are separate claims. The runtime records inference nondeterminism independently and also marks unknown opaque dependencies as nondeterministic. It never serializes executable embedding capabilities, callbacks, credentials, or ambient secrets.

Silk is currently `opaque` when its specialist lacks an explicit snapshot participant or stateless declaration. Exact checkpointing therefore fails with the registered specialist ID. Silk remains usable in ordinary and best-effort contexts. Implementing Silk snapshot/export semantics requires a separate change to its public protocol when an experiment needs Silk in exact developmental branches.
