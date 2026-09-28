# INT-02 — Run the legacy Arachne compatibility experiment

**Status:** Done  
**Track:** Integration  
**Depends on:** SILK-05, SILK-09, INT-01  
**Source:** PRD §10

## Outcome

The new Silk boundary is tested against legacy Zig Arachne as a diagnostic experiment.

## Work

Attempt integration through the public contract, record required adapters and special cases, and classify each gap as a Silk contract deficiency or historical Arachne coupling.

## Acceptance criteria

- Findings inform the contract and Arachne 2 design without distorting Silk solely for legacy compatibility.
- The experiment records its setup, outcome, and unresolved boundary issues.

## Completion notes

- Probed the exact source of required matrix entry 123,
  `agents/episodic_loop_smoke.silk`, through the Rust parser used by SRP
  `program.load`; verified the archived source digest and reference recording.
- The load failed at line 5 on deprecated Arachne-specific `capability`
  syntax. A separate Go/Rust public-protocol round trip succeeded with an
  explicit host descriptor and grant.
- Classified `active_inference.*` and `contract_net.*` as host-owned
  capabilities, while the unsupported `learn` block is a Silk implementation
  gap against the existing Silk 2 semantics.
- The legacy Zig Arachne process is absent from this workspace, so this is a
  diagnostic source/boundary experiment, not process-level interoperation.
  See [experiment report](../docs/LEGACY_ARACHNE_SILK_EXPERIMENT.md).

### Gotchas

- The archived fixture recording is not a frozen-Zig run; reference pins differ.
- Do not add Arachne's `capability` declaration back into Silk to make this
  source load. Declare host functions and grants through session creation.
- `learn` remains intended syntax and should be implemented in Silk before
  reporting this required fixture as compatible.
