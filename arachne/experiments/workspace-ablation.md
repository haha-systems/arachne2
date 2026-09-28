# Experiment 001: Workspace Ablation

## Question

Does Arachne's shared workspace improve collective task performance when the same agents, task, prompts, budgets, and runtime are used without shared publication and broadcast?

This is a validation experiment. The initial smoke batch is not evidence that workspace is useful or useless.

## Task

Each of four evidence agents privately holds one seed-generated 64-bit value. The solver must XOR all four values. A fifth agent publishes a decoy proposal. Each proposal receives a deterministic seed-derived confidence; the real workspace admits four of the five proposals. A decoy that outranks a required shard can therefore cause an enabled trial to fail. In the disabled condition, evidence agents retain their private values and publish nothing, and the solver receives no evidence.

Evidence agents use the existing `workspace.Submit`, `SelectWithPolicy`, and `Broadcast` APIs. No alternate agent-to-agent channel carries evidence. The runner's turn gates only serialize proposal attempts for reproducible event ordering.

## Configuration and run

Edit [workspace-ablation.json](workspace-ablation.json) to set the declarative task controls, seed range, repeat count, and paired profiles. The profiles must differ only in `shared_workspace`.

From the `arachne` Go module directory:

```sh
go run ./examples/workspace-ablation run workspace-ablation
go run ./examples/workspace-ablation compare
```

Override the output with `-out PATH`, the config with `-config PATH`, or compare another JSONL file with `compare -input PATH`.

Run validity tests with:

```sh
go test ./examples/workspace-ablation
```

## Add an ablatable subsystem

1. Identify the subsystem's actual publication, read, and broadcast boundaries in production code.
2. Pass it through a narrow explicit dependency at the experiment adapter boundary.
3. In the disabled arm, remove those operations. Do not route their payload through another channel.
4. Add event assertions proving enabled behavior occurs and disabled behavior is absent.
5. Keep all other profile fields and task inputs equal, use paired seeds, and create per-trial state in the executor.
6. Document any indirect path where subsystem state might still be visible.

The experiment adapter is separate from production cognitive logic. It calls the existing workspace implementation without adding a global feature flag.

## Isolation and determinism

Each trial creates a new cognition memory store, event spine, supervisor, and (only when enabled) workspace. The task generator is a local pseudorandom generator initialized from the paired seed. Proposal turns are serialized in fixed agent order. No state is persisted between trials.

Task inputs, private values, confidence values, event sequence order, and outputs are deterministic for a seed and condition. Wall time and event timestamps are not deterministic. The current synthetic provider has no token usage; `tokens_used` is recorded as zero and `token_usage_available` is false. Process scheduling and runtime overhead can still vary.

## JSONL record schema

Each line is one trial from the shared experiment runner and contains:

- experiment, task, profile, trial number, seed, declarative profile configuration, start time, and duration
- structured cognitive events, including proposal submissions, selection, broadcast planning, and delivery where applicable
- output task input, expected answer, final answer, success, and termination reason
- success score and measurements: agent steps, event and message counts, workspace publications/broadcasts/reads, tokens, errors, and all fixed runtime controls

A publication is counted from the workspace's `proposal_received` event. A broadcast is counted from `broadcast_planned`; delivered workspace messages are counted separately. The enabled condition records two workspace reads: one internal inspection performed by `Broadcast`, and one explicit final inspection by the runner. Disabled records must report zero for all workspace counters.

## Validity limits

- This is a deterministic synthetic harness using Arachne agent supervision and the production workspace. It does not use external models, tools, memory, replay, learning, active inference, or adaptive governance.
- The fixed-size XOR task does not model natural-language collaboration or the full Arachne runtime.
- Proposal confidence and the decoy are designed to exercise bounded workspace selection. Results measure this task and configuration only.
- Runtime measurements are local smoke measurements; do not interpret them as performance benchmarks.
- A task runner error invalidates that trial. A wrong or incomplete answer is an ordinary task failure.
- The initial batch is for framework validation, not a scientific conclusion.
