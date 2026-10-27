# Arachne experiment runner

The `integrated-engineering` task includes a small experiment runner for controlled, repeated trials. Its task is the existing estimate workflow: infer a total replacement cost in credits from two known prior examples, coordinate specialists, execute the retained procedure through Silk, and optionally apply an experience backed routing update.

## Run an experiment

Build the Silk executable first if it is not already available:

```sh
cd silk2
cargo build -p silk-cli
```

From the `arachne` Go module, run the checked in paired ablation:

```sh
go run ./examples/integrated-engineering experiment run \
  -config ./experiments/integrated-engineering-developmental-learning-ablation.json \
  -silk ../silk2/target/debug/silk \
  -output ./trials.jsonl
```

Summarize the JSONL file:

```sh
go run ./examples/integrated-engineering experiment compare -input ./trials.jsonl -score success
```

`-output -` writes JSONL to stdout. `experiment run` writes one row per profile per trial. Errors are stored as rows and later trials continue. A malformed experiment specification stops before any trial starts.

## Profiles and subsystem controls

An experiment specification is JSON. `trials` is the number of paired trial indices. Each index receives one seed shared across all profiles, so the full and ablated runs are paired. Profile names and their explicit subsystem settings are copied to every output row.

The integrated engineering task currently exposes these three safe controls:

| Setting | Enabled behavior | Disabled behavior |
| --- | --- | --- |
| `developmental_learning` | Apply the governed experience backed update to `engineering:estimate`. | Skip proposal, approval, and routing update. The task still executes and returns its result. |
| `inspection` | Capture initial and final inspection snapshots and evolution. | Skip the inspector and history snapshots. Task execution and its event log are unchanged. |
| `memory_replay` | Schedule replay and provide its attention cues as evidence to later actions. | Skip replay completely. Valid episode and specialist evidence still supports governed procedure retention; no replay IDs or cues enter later actions. |

`governance`, the event spine, episodic memory and consolidation, specialist coordination, regulation, and the Silk runtime are required by this task. Replay is independently ablatable because valid episode and specialist evidence still supports the governed action path. The adapter rejects attempts to disable or override them. In particular, governance cannot be switched off through this task's experiment config because doing so would bypass action approval. These are architectural limits of the current example, not global limits of the runner package.

The minimal baseline is available in `experiments/integrated-engineering-minimal.json`. It turns off developmental learning, inspection, and memory replay while retaining the mechanisms needed for this task. `experiments/integrated-engineering-developmental-learning-ablation.json` is the paired experiment: its two profiles differ only in developmental learning. It runs two smoke trials per profile. Smoke results establish that the runner executes and captures both arms; they are not scientific evidence.

To define another experiment, copy a JSON file and choose a name, task, positive trial count, seed, and one or more uniquely named profiles. Keep every control explicit. The integrated engineering adapter only accepts the three controls above.

## Add an ablatable subsystem

1. Identify the subsystem's actual construction and every path through which its behavior enters the selected task. Search for shared objects and callbacks, not only the primary call.
2. Add the dependency to the task's run state or task factory and put the enable check at the narrowest boundary before any effect is produced. Avoid process globals and ambient environment flags.
3. Add the subsystem to `validateIntegratedProfiles` so unknown and required components cannot be silently overridden.
4. Add a test at the task boundary proving a disabled subsystem's implementation is never called and its observable output is absent. If it can still affect the result through another path, the ablation is incomplete.
5. Include its state and relevant measurements in the trial result, and update this document with any dependencies or limitations.

`internal/experiment` is task agnostic. An adapter supplies an `Execute` callback for each profile and seed. Each call must create its own task state and return a cleanup function for resources owned by that trial.

## Isolation and determinism

Every integrated engineering trial constructs a new event store and spine, memory store and service, governance service, developmental engine, Silk process, and Silk session. The runner executes trials sequentially, and the adapter closes the Silk process before the next trial. The experiment does not reuse an Arachne object between profile runs.

Seeds are derived as `spec.seed + trial_index` and shared across profiles at each trial index. The integrated engineering task currently has no pseudorandom choices and does not consume the seed; the output marks this with `seed_consumed_by_task: false`. A seed is preserved for pairing and for later tasks that inject deterministic random sources.

Event identifiers, wall clock timestamps, concurrent specialist scheduling, OS process behavior, and elapsed timings are not deterministic. JSON row ordering and comparison profile ordering are stable. Runtime and timing scores are sensitive to host load and should not be treated as controlled outcomes without additional environment controls.

## JSONL output

Each line is one `Trial` object with schema version 1:

- `experiment`, `task`, `profile`, `trial`, and `seed` identify the run.
- `configuration` stores the complete profile name and subsystem settings.
- `started_at` and `duration_ms` capture timing.
- `events` and `output` contain the Arachne event history and task report on success; error rows can retain partial events, outputs, and measurements.
- `scores` includes `success` (1 for a task without a captured task failure, otherwise 0).
- `measurements` includes event count, routing weights, selected specialist, consolidated pattern count, replay cue count, and whether the task consumed its seed.
- `error` records startup, execution, or cleanup failures. Failed trials do not abort later trials; comparison excludes errored trials from score averages.

The compare command groups rows by profile and reports trial count, errors, mean duration, and mean of the selected score over trials that contain it. It is descriptive and does not estimate uncertainty or test statistical significance.

## Experimental validity limitations

- The current task has safe ablation seams for developmental learning, memory replay, and inspection. Episodic memory and consolidation, multi-agent coordination, regulation, governance, and runtime toggles need explicit dependency injection and alternate task behavior before they can be disabled without changing the task's inputs or safety properties.
- The task uses fixed prior examples and a fixed arithmetic prompt; it does not sample a scientific task distribution.
- The seed currently labels paired runs but has no effect on task choices.
- Timing differences include process startup, scheduler variance, and machine load.
- Smoke trial count is intentionally small. Do not draw scientific claims from the checked in smoke config.

## Experiment 001: workspace ablation

A second task adapter tests the shared workspace directly using a deterministic synthetic XOR evidence-reconstruction task. Its declarative config is `experiments/workspace-ablation.json`; task, CLI, validity tests, and limitations are in [workspace-ablation.md](workspace-ablation.md). Run from this module with:

```sh
go run ./examples/workspace-ablation run workspace-ablation
go run ./examples/workspace-ablation compare
```

The checked-in 20-seed validation batch is in `experiments/results/experiment-001-workspace-ablation.jsonl`, with a paired comparison in `experiments/results/experiment-001-workspace-ablation-summary.md`. These smoke results validate execution and isolation only; they are not scientific evidence.
