# PR4 final quality adjudication

Reviewed commit: `55307907f4a5a21d86f9d8c1fb3cc4f2e91a9c90`  
Comparison parent: `df081738ff10494f636e595ed096fbfe9ac9a19a`

This record adjudicates the committed PR4 quality delta. No production-code refactor was justified by these clone findings. Snapshot and event copies remain explicit so each domain type retains visible defensive-copy semantics.

## Ripwire

`ripwire . --quality-delta=55307907f4a5a21d86f9d8c1fb3cc4f2e91a9c90` reported 34 gating rows: 24 `duplication` rows and 10 repeated `new-clone-of-reused-helper` rows. These are the 34 findings, classified individually. No finding is category A.

| # | Finding | Class | Adjudication |
|---:|---|:---:|---|
| 1 | Conditions / RuntimeRestorability | E | Independent runtime getters take the runtime lock before returning separate state. |
| 2 | Reproducibility / linearParameters | C | Nursery reports runtime reproducibility; agent exports a model parameter vector. Moving either operation would couple embedding and model packages. |
| 3 | RestoreRuntimeState / canonicalJSON | F | Strict state decoding/restoration and canonical JSON normalization have different contracts; similarity is token overlap. |
| 4 | RuntimeRestorability / beginHostCallback | E | Runtime inventory and Silk callback counter use independent synchronization for different state. |
| 5 | RuntimeRestorability / endHostCallback | E | Same independently protected callback counter shape as row 4. |
| 6 | RuntimeRestorability / hostCallbackActive | E | Same mutex discipline, with a read instead of a counter update. |
| 7 | cloneChange / cloneEvent | B | Copies distinct development and event records, including different nested fields. |
| 8 | cloneDeltas / cloneRouting | C | Condition deltas and development routing values have separate ownership and package boundaries. |
| 9 | cloneDeltas / cloneSubsystems | C | Nursery condition-delta copying and example subsystem copying should not create a shared dependency. |
| 10 | cloneEpisode / cloneEvent | B | Episode provenance and event payload/parent/Silk references are different deep-copy contracts. |
| 11 | cloneEpisode / clonePerception | B | Memory episode slices and activation perception buffers have distinct schemas. |
| 12 | cloneEvent / cloneProvider | B | Event payload lineage and provider checkpoint state require type-specific copies. |
| 13 | clonePattern / clonePerception | B | Consolidation pattern members and specialist input buffers are different state. |
| 14 | clonePerception / cloneProposal | B | Input/features and requested-action/event slices must be copied according to their own types. |
| 15 | clonePerception / cloneSelection | B | Perception buffers and workspace selection evidence are different nested values. |
| 16 | clonePerception / cloneSemantic | B | Specialist inputs and semantic provenance have different copy contracts. |
| 17 | clonePerception / cloneWorld | B | Runtime perception and Nursery world maps/slices belong to separate APIs. |
| 18 | deterministicStateID / stableID | C | Runtime interaction-derived IDs and governance IDs are owned by separate packages; sharing a hash helper would add a dependency for identical mechanics only. |
| 19 | finiteScore / validRatioInput | C | Agent learning score bounds and regulation ratio validation are separate domain checks. |
| 20 | mergeIDs / sortedUnique | C | Agent ID merging and Nursery checkpoint ID normalization have distinct owners and input semantics. |
| 21 | mustEventStore / mustMemoryEventStore | D | Independent test packages use separate fixtures for cognition and memory scenarios. |
| 22 | newFixtureRuntime / newInterventionRuntime | D | General runtime fixture configuration differs intentionally from the controlled intervention branch fixture. |
| 23 | sortedUnique / unique | C | Nursery checkpoint normalization and runtime deduplication remain package-local operations. |
| 24 | sortedUnique / uniqueSorted | C | Nursery checkpoint and governance identity normalization remain package-local operations. |
| 25 | Conditions / RuntimeRestorability (reused-helper clone row) | E | Same independent getter lock shape as row 1. |
| 26 | cloneChange / cloneEvent (reused-helper clone row) | B | Same distinct typed copy contracts as row 7. |
| 27 | cloneEpisode / cloneEvent (reused-helper clone row) | B | Same distinct typed copy contracts as row 10. |
| 28 | cloneEpisode / clonePerception (reused-helper clone row) | B | Same distinct typed copy contracts as row 11. |
| 29 | cloneEvent / cloneProvider (reused-helper clone row) | B | Same distinct typed copy contracts as row 12. |
| 30 | clonePattern / clonePerception (reused-helper clone row) | B | Same distinct typed copy contracts as row 13. |
| 31 | clonePerception / cloneProposal (reused-helper clone row) | B | Same distinct typed copy contracts as row 14. |
| 32 | clonePerception / cloneSemantic (reused-helper clone row) | B | Same distinct typed copy contracts as row 16. |
| 33 | newFixtureRuntime / newInterventionRuntime (reused-helper clone row) | D | Same independent behavioural fixtures as row 22. |
| 34 | sortedUnique / unique (reused-helper clone row) | C | Same package-owned helpers as row 23. |

Class A means a shared semantic operation with a clear owner; none was found. A shared reflection/JSON/map clone or generic helper would hide the copy contract, blur package ownership, or couple distinct state types. No clone detection was disabled and no directory was excluded.

The supported Ripwire acknowledgement ledger is per-finding and ratchets when an acknowledged finding worsens. `--ack-only=gating` selected exactly the 34 rows that would otherwise exit 2; the other 50 non-gating findings remain unacknowledged and visible. The committed `.ripwire_quality_acks` records those 34 rows individually with this document as their reason. Re-running the commit-range comparison reports `gating=0, acked=34`. The separate working-tree quality check compares against HEAD and reports zero code-quality changes.

## Gortex

The earlier Gortex review blocked on 12 of 25 files at CRITICAL file-level blast-radius risk. After full re-indexing and fresh profile import, the comparison against the PR4 parent reports 14 of 27 files as CRITICAL, 13 without covering tests; the additional two changed files are this adjudication record and Ripwire's per-finding ledger. One of the 14 is the snapshot test file itself, where zero executable coverage is expected. The risk remains a file-wide aggregation: Gortex reports many unexercised symbols in large files even though the checkpoint, restore, and branch acceptance paths have named behavioral tests. There was no confirmed acceptance-behavior gap that justified adding a superficial test.

| File | Risk reason | Changed behavior | Existing behavioural test(s) | Actual gap? | Action |
|---|---|---|---|:---:|---|
| `experiments/nursery/arachne_adapter.go` | CRITICAL; 60 of 74 indexed symbols reported uncovered; Gortex misses `Decide` test edge | Nursery perception-to-runtime step, action conversion, consequence and snapshot adapters | `TestArachneNurseryRuntimeCheckpointRestoreSmoke`, `TestCheckpointOmitsOpaqueProviderState` | No | Go profile executes `Decide` (56.5%); retained end-to-end smoke. |
| `experiments/nursery/checkpoint_validation.go` | CRITICAL; helper line/call coverage is aggregated at file level | Checkpoint history/digest validation | `TestCheckpointRestorationCreatesIndependentExactBranches`, `TestNurseryExactModePreflightAndBestEffortCheckpointMetadata` | No | Retained malformed/valid checkpoint paths through public Nursery APIs. |
| `experiments/nursery/checkpoint.go` | CRITICAL file risk; helper/file association absent | Exact/best-effort checkpoint and fork creation | `TestCheckpointRestorationCreatesIndependentExactBranches`, `TestForkInterventionIsRecordedAndDoesNotMutateItsSource`, `TestNurseryExactModePreflightAndBestEffortCheckpointMetadata` | No | Retained tests; association is indirect through Nursery APIs. |
| `experiments/nursery/runner.go` | CRITICAL file risk; runner test edge absent | Tick execution and consequence handoff | `TestArachneNurseryRuntimeCheckpointRestoreSmoke` | No | Retained end-to-end smoke test. |
| `internal/agent/learnable_snapshot.go` | CRITICAL file risk; file-level test association absent | Learnable parameters/RNG restore and rejection | `TestLearnableSpecialistSnapshotRestoresParametersAndRNGIndependently`, `TestLearnableSpecialistRestoreRejectsIncompatibleOrMissingState` | No | Retained state-equivalence tests. |
| `internal/agent/restorability.go` | CRITICAL file risk; association absent | Specialist state classification | `TestSpecialistAgentSnapshotContractsNameOpaqueState` | No | Retained contract test. |
| `internal/agent/runtime_snapshot.go` | CRITICAL file risk; association absent | Specialist snapshot/restore contract | `TestSpecialistAgentSnapshotContractsNameOpaqueState` | No | Retained opaque/exact contract test. |
| `internal/agent/specialist_sync.go` | CRITICAL file risk; sync test edge absent | Attributed specialist proposal execution | `TestSpecialistAgentActivateRecordsAttributedProposal` | No | Retained behavioral event-lineage test. |
| `internal/cognition/events.go` | CRITICAL file risk; association absent | Event snapshot, independent branch lineage, restore validation | `TestSpineSnapshotRestoresIndependentBranchLineage`, `TestSpineRestoreRejectsMismatchedIdentityAndIncompleteHistory` | No | Retained lineage and malformed-history tests. |
| `internal/memory/snapshot.go` | CRITICAL file risk; restore method association absent | Attributed memory snapshot/restore and independent copies | `TestMemoryStateSnapshotRoundTripsAttributedEpisodesIndependently` | No | Retained round-trip/provenance/aliasing test; Gortex does not resolve the restore call edge. |
| `internal/organism/restorability.go` | CRITICAL file risk; association absent | Exact inventory and opaque component rejection | `TestRuntimeRestorabilityReportsConfiguredExactComponents`, `TestRuntimeRejectsOpaqueSpecialistForExactSnapshot` | No | Retained exact/opaque tests. |
| `internal/organism/runtime.go` | CRITICAL file risk; association absent | Runtime step, consequence, commitment and state counters | `TestRuntimeSnapshotRestoresDeterministicContinuationAndIndependentForks`, `TestRuntimeSnapshotControlledObservationInterventionCausesAttributedDivergence` | No | Retained branch and intervention tests. |
| `internal/organism/snapshot.go` | CRITICAL file risk; broad method surface association absent | Snapshot validation, restoration, determinism metadata | `TestRuntimeSnapshotRejectsCorruptionIdentityAndMissingState`, `TestRuntimeSnapshotReportsOpaqueComponentsAndMarksBestEffort`, `TestRuntimeSnapshotRestoresDeterministicContinuationAndIndependentForks` | No | Retained validation and continuation tests. |
| `internal/organism/snapshot_test.go` | CRITICAL classification on a test file; zero executable coverage is expected for test code | Behavioural test infrastructure | The file contains the runtime restore/branch tests listed above | No | Treated as test infrastructure, not uncovered production behavior. |

The earlier 11/11 named-symbol result is meaningful for that selected symbol set; it does not imply every file-level association is present. Re-indexing and importing a fresh Go coverage profile enriched 20 indexed symbols (3,775 segments). The profile path needed the indexed-module prefix rewrite from `github.com/haha-systems/arachne2/` to `github.com/haha-systems/ghost/arachne/`. A fresh `change.tests` query for the 11 named snapshot/restore/branch symbols linked ten to behavioral test functions; the sole unmatched method was `ArachneOrganism.Decide`, reached through the Nursery organism interface. The Go profile reports `Decide` at 56.5% statement coverage, and the end-to-end Nursery smoke exercises that dispatch. Thus the current missing edge is dynamic-dispatch indexing; it is not an untested adapter path.

The Gortex checkout metadata still names Git HEAD `df081738ff10494f636e595ed096fbfe9ac9a19a`, while this jj working copy is the committed tree `55307907f4a5a21d86f9d8c1fb3cc4f2e91a9c90`. As a result, the ordinary Gortex review sees zero changed files; reviewing the explicit jj diff lists all 25 files but currently reports zero affected indexed symbols. This is a jj/Git-view association limitation, not evidence of missing behavior. The named test graph and actual Go coverage profile provide independent behavior evidence. Do not add superficial tests to manufacture file links.

## Validation record

- `go test ./...`: passed.
- `golangci-lint run ./...`: passed (0 issues).
- `gofmt -d .`: clean.
- Exact checkpoint/restore, deterministic branch continuation, controlled intervention divergence, and 8-tick Nursery checkpoint/restore smoke tests: passed.
- The 256-tick Experiment 002 pilot was not run.
