# Silk and Arachne 2 — Kanban and Roadmap

This board tracks the work derived from [the reimplementation PRD](prds/prd-arachne-and-silk-2.md). Each task has its own file under [`tasks/`](tasks/).

## Board

Move a task by changing its **Status** here and in its task file. Use `Ready` only when dependencies are complete and the task can start; use `Blocked` with a short reason in the task file. All work starts in Backlog. The VM decision remains Deferred until evidence justifies revisiting it.

### Backlog

| ID                                                               | Task                                                    | Depends on                                                    |
| ---------------------------------------------------------------- | ------------------------------------------------------- | ------------------------------------------------------------- |
| [AR-12](tasks/AR-12-implement-governed-replay.md)                | Implement governed replay and consolidation             | AR-07, AR-13                                                  |
| [AR-14](tasks/AR-14-implement-developmental-changes.md)          | Implement provenance-bearing development                | AR-07, AR-09, AR-11, AR-13                                    |
| [AR-15](tasks/AR-15-connect-arachne-to-silk-acquisition.md)      | Connect Arachne development to Silk acquisition         | AR-14, AR-13, SILK-14, SILK-17, SILK-18                       |
| [AR-16](tasks/AR-16-build-developmental-visualization.md)        | Build developmental visualization and inspection        | AR-14, AR-15                                                  |
| [AR-17](tasks/AR-17-run-integrated-engineering-experiment.md)    | Run an integrated engineering experiment                | AR-04, AR-05, AR-07, AR-09, AR-11, AR-12, AR-13, AR-14, AR-15 |
| [INT-01](tasks/INT-01-define-silk-arachne-boundary.md)           | Define the Silk and Arachne boundary                    | SILK-01, AR-01                                                |
| [INT-03](tasks/INT-03-establish-independent-release-strategy.md) | Establish independent repository and release identities | SILK-01, AR-01                                                |

### Ready

| ID                                                | Task                                        | Depends on   |
| ------------------------------------------------- | ------------------------------------------- | ------------ |
| [AR-12](tasks/AR-12-implement-governed-replay.md) | Implement governed replay and consolidation | AR-07, AR-13 |

### In Progress

No tasks are marked In Progress.

### Review

No tasks are marked Review.

### Done

| ID                                                                | Task                                                    | Evidence                                                       |
| ----------------------------------------------------------------- | ------------------------------------------------------- | -------------------------------------------------------------- |
| [REF-02](tasks/REF-02-inventory-research-assets.md)               | Inventory subsystems and research assets                | [Reference inventory](docs/REFERENCE_INVENTORY.md)             |
| [REF-01](tasks/REF-01-stabilize-zig-reference.md)                 | Stabilize the Zig reference system                      | [Reference freeze](docs/REFERENCE_FREEZE.md)                   |
| [SILK-01](tasks/SILK-01-write-rust-foundation-spec.md)            | Specify Silk Rust Phase 0/1                             | [Foundation specification](docs/SILK_RUST_FOUNDATION.md)       |
| [SILK-03](tasks/SILK-03-define-core-language-semantics.md)        | Define core language semantics                          | [Language semantics](docs/SILK_LANGUAGE_SEMANTICS.md)          |
| [SILK-05](tasks/SILK-05-define-host-capability-contract.md)       | Define the host capability contract                     | [Host protocol contract](docs/SILK_HOST_PROTOCOL.md)           |
| [SILK-02](tasks/SILK-02-create-independent-rust-project.md)       | Create the independent Rust project                     | [Rust workspace](silk2/README.md)                              |
| [REF-03](tasks/REF-03-build-golden-silk-corpus.md)                | Build a golden Silk corpus                              | [Versioned corpus](silk2/reference/silk/README.md)             |
| [REF-04](tasks/REF-04-classify-compatibility-and-experiments.md)  | Classify compatibility and experiments                  | [Compatibility decisions](docs/COMPATIBILITY_DECISIONS.md)     |
| [AR-01](tasks/AR-01-write-arachne-2-architecture-spec.md)         | Specify the smallest coherent Arachne 2                 | [Architecture specification](docs/ARACHNE2_ARCHITECTURE.md)    |
| [SILK-04](tasks/SILK-04-inventory-and-classify-syntax.md)         | Inventory and classify legacy syntax                    | [Syntax compatibility](docs/SILK_SYNTAX_COMPATIBILITY.md)      |
| [AR-02](tasks/AR-02-build-go-runtime-skeleton.md)                 | Build the Go runtime skeleton                           | [Go runtime](arachne/README.md)                                |
| [INT-01](tasks/INT-01-define-silk-arachne-boundary.md)            | Define the Silk and Arachne boundary                    | [Integration boundary](docs/SILK_ARACHNE_BOUNDARY.md)          |
| [INT-03](tasks/INT-03-establish-independent-release-strategy.md)  | Establish independent repository and release identities | [Release strategy](docs/RELEASE_STRATEGY.md)                   |
| [AR-03](tasks/AR-03-implement-agent-lifecycle-and-transport.md)   | Implement agent lifecycle and event transport           | [Agent runtime](docs/ARACHNE_AGENT_RUNTIME.md)                 |
| [SILK-06](tasks/SILK-06-model-effects-and-authority.md)           | Model effects, grants, and authority checks             | [Effects authority](docs/SILK_EFFECTS_AUTHORITY.md)            |
| [SILK-07](tasks/SILK-07-design-semantic-ir.md)                    | Design the semantic IR                                  | [Semantic IR](docs/SILK_SEMANTIC_IR.md)                        |
| [SILK-08](tasks/SILK-08-lower-source-to-ir.md)                    | Lower source programs into semantic IR                  | [Parser and lowering](docs/SILK_PARSER_LOWERING.md)            |
| [SILK-09](tasks/SILK-09-implement-ir-evaluator.md)                | Implement the IR evaluator                              | [IR evaluator](docs/SILK_EVALUATOR.md)                         |
| [SILK-11](tasks/SILK-11-emit-structured-traces.md)                | Emit structured execution traces                        | [IR evaluator](docs/SILK_EVALUATOR.md)                         |
| [SILK-10](tasks/SILK-10-compare-legacy-behavior.md)               | Compare behavior with the Zig reference                 | [Compatibility status](docs/SILK_COMPATIBILITY_STATUS.md)      |
| [SILK-13](tasks/SILK-13-define-procedure-identity-and-lineage.md) | Define procedural identity and lineage                  | [Identity and lineage](docs/SILK_PROCEDURE_IDENTITY.md)        |
| [SILK-12](tasks/SILK-12-build-semantic-inspection.md)             | Build semantic program inspection                       | [Semantic inspection](docs/SILK_INSPECTION.md)                 |
| [SILK-14](tasks/SILK-14-implement-procedure-registry.md)          | Implement the procedure registry                        | [Procedure registry](docs/SILK_PROCEDURE_REGISTRY.md)          |
| [SILK-15](tasks/SILK-15-add-semantic-retrieval.md)                | Add semantic capability retrieval                       | [Semantic retrieval](docs/SILK_SEMANTIC_RETRIEVAL.md)          |
| [SILK-16](tasks/SILK-16-compose-and-validate-procedures.md)       | Compose and validate procedures                         | [Procedure composition](docs/SILK_COMPOSITION.md)              |
| [SILK-17](tasks/SILK-17-add-procedural-synthesis-pipeline.md)     | Add the procedural synthesis pipeline                   | [Synthesis pipeline](docs/SILK_SYNTHESIS.md)                   |
| [SILK-18](tasks/SILK-18-model-procedure-retention-states.md)      | Model procedure retention states                        | [Retention lifecycle](docs/SILK_RETENTION.md)                  |
| [SILK-19](tasks/SILK-19-demonstrate-standalone-silk.md)           | Demonstrate standalone Silk                             | [Standalone demonstration](docs/SILK_STANDALONE_DEMO.md)       |
| [AR-04](tasks/AR-04-integrate-silk-client.md)                     | Integrate Arachne with the public Silk runtime          | [Integration contract](docs/ARACHNE_SILK_INTEGRATION.md)       |
| [AR-05](tasks/AR-05-implement-cognitive-event-spine.md)           | Implement the cognitive event spine                     | [Cognitive event model](docs/ARACHNE_COGNITIVE_EVENTS.md)      |
| [AR-06](tasks/AR-06-implement-attributed-memory.md)               | Implement attributed organism memory                    | [Attributed memory](docs/ARACHNE_ATTRIBUTED_MEMORY.md)         |
| [AR-07](tasks/AR-07-add-memory-consolidation.md)                  | Add memory consolidation                                | [Memory consolidation](docs/ARACHNE_MEMORY_CONSOLIDATION.md)   |
| [AR-08](tasks/AR-08-implement-specialist-proposals.md)            | Implement specialist agents and proposals               | [Specialist proposals](docs/ARACHNE_SPECIALISTS.md)            |
| [AR-09](tasks/AR-09-implement-bounded-workspace.md)               | Implement bounded workspace coordination                | [Bounded workspace](docs/ARACHNE_WORKSPACE.md)                 |
| [AR-10](tasks/AR-10-model-regulation-and-prediction-error.md)     | Model regulation and prediction error                   | [Regulation model](docs/ARACHNE_REGULATION.md)                 |
| [AR-11](tasks/AR-11-validate-regulatory-effects.md)               | Validate regulatory effects on cognition                | [Regulation experiment](docs/ARACHNE_REGULATION_EXPERIMENT.md) |
| [AR-13](tasks/AR-13-implement-governance.md)                      | Implement governance for consequential changes          | [Governance contract](docs/ARACHNE_GOVERNANCE.md)              |
| [INT-02](tasks/INT-02-test-legacy-arachne-with-new-silk.md)       | Run the legacy Arachne compatibility experiment         | [Experiment report](docs/LEGACY_ARACHNE_SILK_EXPERIMENT.md)    |

### Blocked

No tasks are marked Blocked. Record the blocking reason on the task card if one arises.

### Deferred

| ID                                                   | Task                                  | Revisit when                                                                                                                            |
| ---------------------------------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| [SILK-20](tasks/SILK-20-reassess-bytecode-and-vm.md) | Reassess the bytecode and VM decision | The evaluator exposes measured needs such as performance, isolation, resource accounting, portable artifacts, or another runtime target |

## Roadmap

The sequence follows the PRD’s dependency gates. Detailed implementation is staged behind the Silk foundation and Arachne architecture specifications. Arachne governance is scheduled before replay because replay must not bypass it.

| Stage                                 | Focus                                                                                    | Tasks and exit gate                                         |
| ------------------------------------- | ---------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| 0. Preserve the reference             | Keep Zig runnable; capture research evidence, corpus, and compatibility choices.         | REF-01 + REF-02 → REF-03 → REF-04                           |
| 1. Specify both projects              | Define the first Rust Silk milestone and the smallest coherent Arachne organism.         | SILK-01 and AR-01; then INT-01 and INT-03                   |
| 2. Build independent foundations      | Establish Rust Silk and the Go daemon, agent lifecycle, and transport.                   | SILK-02; AR-02 → AR-03                                      |
| 3. Define and execute core Silk       | Specify semantics, host contracts, authority, IR, parsing/lowering, and evaluation.      | SILK-03 → SILK-09; compare required behavior in SILK-10     |
| 4. Establish the runtime boundary     | Use a versioned public contract for Arachne’s Silk host and traces.                      | INT-01; AR-04 → AR-05                                       |
| 5. Make Silk inspectable and reusable | Trace and inspect programs; admit, retrieve, compose, synthesize, and retain procedures. | SILK-11 → SILK-18; standalone demonstration in SILK-19      |
| 6. Build cognitive foundations        | Implement attributable memory, specialists, workspace, regulation, and governed replay.  | AR-06 → AR-07; AR-08 → AR-09 → AR-10 → AR-11; AR-13 → AR-12 |
| 7. Enable development                 | Apply governed changes and connect procedural acquisition to Silk.                       | AR-14 → AR-16                                               |
| 8. Exercise the integrated organism   | Run an experiment that reconstructs perception through developmental change.             | AR-17                                                       |
| Diagnostic, when ready                | Test whether legacy Arachne can use the public Silk boundary.                            | INT-02 after Silk execution and the contract are usable     |
| Deferred, evidence-led                | Revisit a bytecode or VM only when a concrete requirement exists.                        | SILK-20                                                     |

## Working rules

- Update the status in both this board and the task file when work moves.
- A task may be split further if implementation reveals independent deliverables; link the new cards here.
- Keep compatibility decisions and experiment findings connected to their evidence.
- Track Silk and Arachne separately while preserving the contract and dependency gates between them.
