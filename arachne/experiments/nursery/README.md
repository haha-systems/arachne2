# Experiment 002: The Nursery

The Nursery is an isolated two-dimensional environment. It does not change
Arachne's cognitive packages. It exposes one organism through a local
perception and a bounded action vocabulary (`move`, `observe`, `inspect`,
`take`, `drop`, `interact`, `use`, and `wait`). It provides no network, shell,
filesystem, credential, or external-service actions.

The world generator uses a serializable deterministic PRNG. Perception contains
only nearby cells and entities, measurable appearance, recent changes, and the
organism's energy, integrity, and stability. Its only imperative is `remain
viable`, with explicit viable ranges for those variables. Entity effects, resource availability,
the full map, and PRNG state remain in the environment/checkpoint. No reward,
task-completion, or correctness signal is defined. The teacher interface is
available for later experiments and is disabled in the checked-in configuration.

## Validate the environment

From the `arachne` module:

```sh
go run ./examples/nursery validate --seed 2026002
```

This checks deterministic generation, local perception, persistent ticking,
checkpoint restoration and branching, and the action boundary. Its wait-only
adapter is a test fixture and is not an Arachne pilot.

## Checkpoints and logs

`Session.Step` records full before/after world snapshots, perception, cognitive
trace supplied by the organism adapter, action, consequences, viability, and a
hash-linked event history. `Session.CreateCheckpoint` captures the complete
world, PRNG state, adapter state, provider metadata, configuration, and event
history. `SaveCheckpoint` writes atomically; `Fork` restores independent
continuations. `ForkWithIntervention` records a named intervention before
restoring the changed branch. The baseline run does not perform cognitive
ablations or teacher interventions.

Use `WriteJSONL` and `ReadJSONL` for appendable/reconstructable tick history.
Offline measurements are available with:

```sh
go run ./examples/nursery analyze -input ticks.jsonl
```

The summary reports counts, entropy, sequence motifs, locations, entity
interactions, viability trajectories, and recorded memory/replay timing. These
are descriptive measurements, not success or intelligence scores.

## Pilot status and limitations

The repository currently has no inference provider, action-producing Arachne
organism runtime, or constructor that keeps the existing cognitive services as
one persistent organism across ticks. The existing `agent.Specialist` contract
produces proposals, not environment actions. The Nursery therefore exposes a
stateful `Organism` adapter contract but does not invent an action policy or
claim a pilot run. See `../experiment-002-nursery.json` for the seed and intended
pilot length. A substantive pilot requires wiring an existing Arachne runtime
and its configured inference provider to this adapter, then recording its state
and cognitive event IDs through `CognitiveTrace`.

Provider inputs/outputs and provider state can be recorded by the adapter.
Exact cognitive replay depends on the provider's determinism and serialized
state; this status is carried in `ProviderRecord` and the checkpoint. Environment
generation and transitions remain reproducible from a checkpoint.
