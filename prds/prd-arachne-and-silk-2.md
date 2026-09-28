# Silk and Arachne Reimplementation Plan

**Status:** High-level roadmap  
**Target stack:** Silk in Rust, Arachne in Go  
**Strategy:** Preserve the existing Zig implementation as a reference system, then reimplement both projects around clean architectural boundaries rather than porting them mechanically.

---

# 1. Purpose

Silk and Arachne were originally conceived and implemented together.

That coupling helped both systems evolve quickly, but it now obscures their distinct identities.

The reimplementation separates them into two independent projects:

**Silk** becomes a general-purpose semantic runtime for procedural knowledge.

**Arachne** becomes a developmental cognitive organism that consumes Silk through an explicit external contract.

The rewrite is not intended to reproduce every implementation detail of the existing Zig system.

Instead, the current codebase becomes:

- a behavioural reference,
- an experimental archive,
- a source of proven requirements,
- a source of negative lessons,
- and an executable specification where compatibility matters.

The new systems should implement the architecture that emerged from the research rather than preserve historical accidents.

---

# 2. Core Architectural Relationship

The target relationship is:

Human or agent intent

→ application or Arachne cognition

→ Silk procedural request

→ Silk runtime

→ host capabilities

→ result and trace

Silk knows nothing about Arachne internals.

Arachne knows nothing about Silk implementation internals.

Their only relationship is through explicit contracts.

---

# 3. Project Identities

## Silk

**Language:** Rust

Silk is responsible for:

- source-language parsing,
- procedural semantics,
- semantic IR,
- procedures and capabilities,
- effects,
- authority requirements,
- runtime sessions,
- host-capability invocation,
- validation,
- execution,
- tracing,
- inspection,
- procedural identity and provenance,
- capability registry,
- later procedural composition and synthesis.

Silk should be independently useful without Arachne.

---

## Arachne

**Language:** Go

Arachne is responsible for:

- daemon and organism lifecycle,
- agents,
- cognition,
- memory,
- workspace,
- regulation,
- active inference,
- SWR and replay,
- governance,
- learning,
- developmental state,
- long-running adaptation,
- experiments,
- observability of organism behaviour.

Arachne consumes Silk as infrastructure.

---

# 4. Non-Goals

The reimplementation is not:

- a line-by-line Zig-to-Rust or Zig-to-Go port,
- an attempt to preserve every existing API,
- an opportunity to redesign everything simultaneously,
- a requirement to reproduce historical accidental coupling,
- a reason to immediately retire the existing system.

The existing Zig implementation remains valuable until the new systems prove themselves.

---

# 5. Shared Principles

## Preserve before replacing

The current Zig system must remain runnable and reproducible.

## Compatibility is selective

Behaviour is preserved where it defines meaningful Silk or Arachne semantics.

Historical implementation accidents do not automatically become requirements.

## Nothing is ported merely because it exists

Every subsystem must justify its place in the new architecture.

## Explicit boundaries beat convenience

Neither project may reach into the other's internal state.

## Observability is foundational

Generated or developmental behaviour must be understandable from structured traces and semantic representations.

## Reimplementation proceeds in layers

First preserve knowledge.

Then build Silk.

Then establish the runtime boundary.

Then build Arachne around that boundary.

---

# 6. Shared Stage 0 — Preserve the Existing System

Before either rewrite begins in earnest, freeze the current system as a reference implementation.

## Goals

Preserve:

- existing Silk programs,
- Arachne experiments,
- research results,
- important runtime behaviour,
- existing tests,
- existing architectural documentation,
- known negative results.

## Work

Create a stable reference tag or branch.

Document the currently important subsystem inventory.

Create a golden Silk corpus from existing agent and example programs.

Record nondeterministic external interactions where necessary.

Create a compatibility matrix distinguishing:

- behaviour that must be preserved,
- behaviour that should be preserved if practical,
- legacy behaviour,
- behaviour deliberately abandoned.

Identify which Arachne experiments are scientifically or architecturally important enough to reproduce later.

## Exit criterion

The Zig implementation can serve as a stable behavioural and research reference while new development proceeds independently.

---

# 7. Silk Track

# Silk Phase 1 — Establish the Rust Project

## Goal

Create a completely independent Silk implementation with no Arachne dependency.

## Work

Establish:

- Rust workspace,
- parser crate or module,
- semantic model,
- runtime package structure,
- testing infrastructure,
- reference corpus integration,
- documentation structure.

The new repository should contain no Arachne-specific implementation concepts unless explicitly represented as compatibility fixtures.

A useful rule is:

> Silk must make sense to a developer who has never heard of Arachne.

## Exit criterion

The Rust Silk project builds, tests, and can execute trivial independent programs.

---

# Silk Phase 2 — Define the Core Language Model

## Goal

Define what Silk means before reproducing the full legacy language.

## Work

Specify:

- values,
- bindings,
- procedures,
- control flow,
- inputs,
- outputs,
- errors,
- runtime sessions,
- host calls.

Classify legacy Silk syntax into:

- retained generic syntax,
- generic syntax with new semantics,
- compatibility syntax,
- deprecated Arachne-specific syntax.

Resolve the meaning of the existing `capability` concept.

## Exit criterion

The core Silk language has a documented semantic model independent of the Zig interpreter.

---

# Silk Phase 3 — Host Capability and Authority Model

## Goal

Make all external interaction explicit.

## Work

Define:

- host capability identifiers,
- host provider interface,
- runtime session context,
- grants,
- effects,
- static effect inference,
- conservative dynamic effects,
- host errors.

A host capability represents something the environment can do.

Examples may include:

- LLM invocation,
- MCP interaction,
- filesystem access,
- memory access,
- transport operations.

Silk does not define the host implementation.

## Exit criterion

A Silk program can invoke external functionality without knowing which application provides it, and the runtime can determine whether the current session has authority to do so.

---

# Silk Phase 4 — Semantic IR

## Goal

Create the canonical semantic representation of Silk programs.

## Work

Design an IR centred on:

- procedures,
- semantic capabilities,
- control flow,
- dependencies,
- effects,
- authority,
- contracts,
- provenance,
- identity.

The IR must not simply reproduce the parser AST.

Source becomes one way of constructing semantic IR.

Future generated programs may construct the same IR without ever existing as human-written source.

## Exit criterion

Representative legacy and new Silk programs lower cleanly into one semantic representation.

---

# Silk Phase 5 — IR Evaluator and Compatibility

## Goal

Execute semantic Silk without introducing bytecode prematurely.

## Work

Build a tree- or graph-walking evaluator.

Run the golden corpus against:

- legacy Zig Silk,
- new Rust Silk.

Use versioned record/replay for nondeterministic host interactions.

Document intentional incompatibilities.

## Exit criterion

The new runtime reproduces all behaviour classified as required compatibility.

---

# Silk Phase 6 — Tracing and Inspection

## Goal

Make program behaviour understandable without requiring source-code reconstruction.

## Work

Emit structured events for:

- procedure entry and exit,
- branches,
- host calls,
- results,
- errors,
- authority decisions,
- effect execution.

Build program inspection capable of answering:

- what does this do,
- what can it affect,
- what does it depend on,
- what authority does it require,
- what happened during execution.

## Exit criterion

A human unfamiliar with a program can correctly understand its major behaviour from Silk's semantic inspection interface.

---

# Silk Phase 7 — Capability Registry

## Goal

Represent procedural knowledge as reusable semantic units.

## Work

Add:

- procedural identity,
- semantic descriptions,
- contracts,
- provenance,
- version lineage,
- validation evidence,
- registry admission.

Provide one admission interface for persistent procedural knowledge.

## Exit criterion

Procedures can be registered, discovered, versioned, and inspected independently of their source-level names.

---

# Silk Phase 8 — Semantic Retrieval and Composition

## Goal

Retrieve and combine existing procedural knowledge.

## Work

Support:

- capability queries,
- candidate selection,
- compatibility checking,
- procedural graph composition,
- validation before execution.

Prefer deterministic composition before introducing generative synthesis.

## Exit criterion

Previously unseen tasks can be satisfied by combining existing registered procedures.

---

# Silk Phase 9 — Procedural Synthesis and Retention

## Goal

Realise the original machine-authored Silk vision.

## Work

Allow higher-level systems to create missing procedures.

Generated procedures must enter the ordinary Silk pipeline:

candidate

→ semantic IR

→ validation

→ effect and authority analysis

→ execution

→ optional registry admission.

Distinguish:

- ephemeral,
- candidate,
- retained procedures.

## Exit criterion

Silk can safely execute and later reuse a procedure that no human manually authored.

---

# Silk Phase 10 — Standalone Procedural Computer

## Goal

Demonstrate Silk outside Arachne.

Build an application where a user requests an outcome and the system retrieves, composes, executes, and explains Silk procedures.

The initial domain should use capabilities Silk genuinely supports rather than requiring an artificial benchmark.

## Exit criterion

Silk is demonstrably useful as an independent public technology.

---

# Silk Deferred Work — Bytecode and VM

Bytecode remains planned but not required initially.

Introduce it when justified by concrete needs such as:

- performance,
- portable executable artifacts,
- hard resource accounting,
- stronger isolation,
- deterministic low-level replay,
- multiple execution engines,
- WASM or other runtime targets.

Semantic IR remains canonical.

Bytecode is an execution representation.

---

# 8. Arachne Track

Arachne development begins conservatively while Silk matures.

The first Arachne work should focus on architecture and contracts rather than immediately implementing the full organism.

---

# Arachne Phase 1 — Define Arachne 2

## Goal

Specify the smallest system that still deserves to be called Arachne.

Start from the research architecture rather than the existing module tree.

## Questions

For every old subsystem:

- What role does this play in the organism?
- Which hypothesis or engineering requirement requires it?
- Is it still necessary?
- Can it be substantially simpler?
- Does another subsystem now make it redundant?

## Deliverable

Arachne 2 architecture specification.

The core should likely include:

- organism daemon,
- agents,
- event spine,
- Silk runtime client,
- memory,
- workspace,
- regulation,
- governance,
- development.

Everything else must justify itself.

## Exit criterion

Arachne can be described as one coherent organism rather than a catalogue of research modules.

---

# Arachne Phase 2 — Go Runtime Skeleton

## Goal

Create the smallest long-running organism runtime.

## Work

Implement:

- process lifecycle,
- contexts and cancellation,
- agent lifecycle,
- message/event transport,
- runtime configuration,
- structured logging,
- tracing,
- deterministic test harness.

Avoid implementing advanced cognition yet.

## Exit criterion

Arachne can start, supervise agents, exchange events, and shut down cleanly.

---

# Arachne Phase 3 — Silk Integration

## Goal

Make Arachne a real external consumer of new Silk.

## Work

Implement the Arachne Silk host.

Expose Arachne capabilities through the Silk host interface where justified.

Define:

- session creation,
- agent bindings,
- procedure execution,
- result extraction,
- capability grants,
- Silk trace ingestion.

No direct shared-memory shortcuts are allowed.

## Exit criterion

An Arachne agent can execute Silk through the same public contract available to any other Silk host.

---

# Arachne Phase 4 — Observable Cognitive Event Spine

## Goal

Create the common substrate through which organism behaviour can be understood.

Major cognitive events should flow through one observable event model.

Examples may include:

- perception,
- memory retrieval,
- agent activation,
- workspace proposal,
- workspace admission,
- decision,
- action,
- prediction error,
- replay,
- regulation change,
- governance decision,
- developmental change.

## Exit criterion

A complete agent interaction can be reconstructed from structured events.

---

# Arachne Phase 5 — Memory

## Goal

Implement only the memory architecture required by the new organism.

Potential layers include:

- episodic memory,
- semantic memory,
- identity or continuity structures where still justified,
- retrieval,
- consolidation.

The existing Zig implementation should inform behaviour but not dictate implementation structure.

Ghostdive remains separate.

## Exit criterion

Experiences can be stored, retrieved, attributed, and later influence cognition through explicit interfaces.

---

# Arachne Phase 6 — Workspace and Specialist Coordination

## Goal

Reintroduce meaningful multi-agent cognitive coordination.

Implement:

- specialist agents,
- proposals,
- competition,
- bounded workspace,
- broadcast,
- reportability,
- tracing of selection.

Preserve lessons from prior workspace experiments, including relevant negative results.

## Exit criterion

Multiple specialists can contribute to cognition through an observable selective coordination mechanism.

---

# Arachne Phase 7 — Regulation and Active Inference

## Goal

Restore the systems that determine how the organism allocates cognitive effort and responds to uncertainty.

Potential responsibilities include:

- prediction error,
- salience,
- allostatic load,
- action pressure,
- cognitive state,
- authority modulation.

These should be implemented as organism-level concepts rather than Silk-language internals.

## Exit criterion

Regulatory state measurably changes cognitive behaviour through explicit mechanisms.

---

# Arachne Phase 8 — Replay and Consolidation

## Goal

Reintroduce governed offline or deferred learning.

Use lessons from existing SWR work.

Maintain the authority principle that replay may influence attention or candidates without silently bypassing governance.

## Exit criterion

Past experiences can influence future behaviour through observable consolidation without direct uncontrolled authority escalation.

---

# Arachne Phase 9 — Governance

## Goal

Provide explicit oversight over consequential cognitive and developmental changes.

Governance may control:

- authority grants,
- memory mutation,
- identity changes,
- procedural retention,
- structural modification,
- high-impact actions.

Governance should operate above Silk runtime authority rather than replacing it.

## Exit criterion

The organism can distinguish what is technically executable from what it is permitted to do or retain.

---

# Arachne Phase 10 — Development

## Goal

Allow the organism to change meaningfully through experience.

Development may affect:

- memory,
- routing,
- salience,
- agent specialisation,
- learned procedures,
- Silk repertoire,
- possibly topology where justified.

Changes must retain provenance.

## Exit criterion

Two initially equivalent Arachne instances can acquire observable differences as a result of differing experience.

---

# Arachne Phase 11 — Silk Procedural Acquisition

## Goal

Connect Arachne development to Silk's procedural registry.

Arachne may detect recurring useful cognitive structures and propose them as Silk candidates.

Pipeline:

experience

→ recurring structure

→ procedural candidate

→ Silk validation

→ Arachne governance

→ registry admission.

Arachne does not directly mutate Silk internals.

## Exit criterion

Arachne acquires a reusable Silk procedure absent from its initial repertoire.

---

# Arachne Phase 12 — Developmental Visualisation

## Goal

Make the organism's changing cognitive architecture visible.

Support inspection of:

- acquired procedures,
- memory changes,
- specialist development,
- regulation changes,
- structural changes,
- governance history,
- divergence between organisms.

## Exit criterion

A human can answer:

> Why is this organism different now from when it started?

---

# Arachne Phase 13 — Integrated Engineering Experiment

## Goal

Fully exercise the organism as one system.

The experiment should require meaningful contribution from:

- agents,
- Silk,
- memory,
- workspace,
- regulation,
- replay,
- governance,
- development.

The goal is integration and observability rather than proving a grand cognitive claim.

The central question is:

> Can we trace how Arachne perceived, remembered, reasoned, acted, learned, and changed?

## Exit criterion

A sustained run produces an intelligible developmental history, whether the task itself succeeds or fails.

---

# 9. Integration Strategy Between the Projects

Silk should mature ahead of Arachne's deeper implementation.

A useful sequencing is:

1. Preserve Zig reference system.
2. Begin Rust Silk.
3. Establish Silk host/runtime contract.
4. Build semantic IR and evaluator.
5. Begin minimal Go Arachne runtime.
6. Integrate Arachne with new Silk.
7. Continue semantic Silk and cognitive Arachne in parallel.
8. Add autonomous Silk composition.
9. Add Arachne development.
10. Connect Arachne development to Silk procedural retention.

This prevents Arachne 2 from immediately becoming dependent on unstable Silk internals.

---

# 10. Legacy Arachne Compatibility Experiment

Once new Silk is capable enough, attempt to connect the existing Zig Arachne to it.

This is not required as the permanent architecture.

Its purpose is diagnostic.

If legacy Arachne can consume new Silk through the public contract with reasonable effort, the Silk boundary is probably good.

If integration requires numerous special cases, investigate whether the Silk API is insufficient or whether those requirements are historical Arachne coupling that should not survive.

The experiment informs Arachne 2 design.

It must not distort Silk solely to accommodate the old implementation.

---

# 11. Repository Strategy

Initially:

- existing Zig Arachne remains preserved,
- new Rust Silk receives its own repository,
- new Go Arachne may begin in a separate repository or a clearly independent next-generation location.

Silk should have its own release identity from the beginning.

Arachne should consume explicit Silk releases or protocol versions once the interface stabilises.

The two projects should not share source-code modules.

---

# 12. Protocol Versus Embedding

The first Silk/Arachne integration should prefer a boundary that makes cheating difficult.

Possible implementations include:

- local IPC,
- Unix sockets,
- structured RPC,
- subprocess protocol.

Direct Rust-to-Go FFI should not be the default merely for convenience.

A protocol boundary gives:

- language independence,
- stronger isolation,
- easier third-party usage,
- explicit versioning,
- better black-box testing.

An embeddable Silk library can be added later for hosts that genuinely benefit from it.

---

# 13. What Must Not Be Lost

The rewrite must preserve intellectual and experimental progress even where implementation changes completely.

Important assets include:

- experiment definitions,
- experimental outcomes,
- negative results,
- governance principles,
- replay authority boundaries,
- workspace findings,
- memory architecture lessons,
- active-inference findings,
- developmental hypotheses,
- provenance requirements,
- reproducibility practices.

The objective is to discard accidental code complexity without discarding research history.

---

# 14. Success Criteria for Silk

The rewrite succeeds when Silk is:

- independently buildable,
- independently understandable,
- usable without Arachne,
- capable of representing procedural knowledge semantically,
- safe to execute through explicit host authority,
- observable,
- semantically inspectable,
- capable of procedural retrieval and composition,
- eventually capable of synthesis and retention.

A stranger should be able to use Silk without reading Arachne documentation.

---

# 15. Success Criteria for Arachne

The rewrite succeeds when Arachne:

- exists as one coherent long-running organism,
- consumes Silk as an external system,
- has observable cognitive behaviour,
- maintains memory and continuity,
- coordinates specialist cognition,
- regulates itself,
- governs consequential changes,
- learns through experience,
- can acquire new procedural knowledge,
- and can explain how its architecture changed over time.

The new implementation should make the organism easier to see, not merely easier to compile.

---

# 16. Immediate Next Steps

The roadmap should now split into two implementation programmes.

## Silk first

The first detailed specification should cover:

**Silk Rust Phase 0/1: reference corpus, language inventory, project skeleton, and minimal independent runtime.**

Its output should be the clean foundation of the new Silk implementation.

## Arachne second

In parallel or immediately afterwards, create:

**Arachne 2 Architecture Specification.**

That document should not yet prescribe implementation tickets.

Its job is to answer:

> Knowing everything learned from the existing system, what is the smallest coherent organism we would build today?

Only after those two documents exist should detailed implementation decomposition begin.

---

# 17. Long-Term Vision

The rewrite creates two systems with independent identities.

**Silk** asks:

> What does this system know how to do?

It represents procedural knowledge in a form machines can retrieve, compose, execute, and explain.

**Arachne** asks:

> What should this organism do, remember, learn, and become?

It develops through interaction with its environment and may expand the Silk procedural repertoire it can use.

The architecture deliberately separates:

- procedural knowledge,
- developmental cognition,
- host effects,
- execution authority,
- governance,
- memory.

That separation makes both projects simpler conceptually while allowing them to become more ambitious independently.

The existing Zig implementation discovered many of these ideas.

The Rust and Go implementations are intended to build the systems those discoveries now imply.
