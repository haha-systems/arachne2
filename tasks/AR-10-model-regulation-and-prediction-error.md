# AR-10 — Model regulation and prediction error

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-05, AR-09  
**Source:** PRD §8, Arachne Phase 7

## Outcome

Organism-level signals can represent surprise, salience, load, action pressure, and cognitive state.

## Work

Select justified signals from research, define how they are computed and emitted, and specify which cognitive mechanisms they may influence. Keep these concepts outside Silk internals.

## Acceptance criteria

- Each signal has an explicit meaning and observable source.
- Signal changes can be traced to affected Arachne behavior.

## Completion notes

- Added an organism-level regulator with explicit inputs for prediction/outcome, declared salience, active specialist load, pending action pressure, and configured capacities.
- Defined normalized numeric prediction error and canonical JSON mismatch rules, bounded signal formulas, and a configurable `steady`/`alert`/`strained` state.
- Regulation returns a workspace selection policy: high load/action pressure reduce admission capacity; high surprise/salience require proposal evidence. Selection and broadcast preserve the source regulation event IDs and reasons.
- Updated the integrated specialist example to demonstrate capacity changing from two to one and evidence enforcement. The run selected one proposal and broadcast it without executing the candidate action.
- Documented the definitions, formulas, causal links, and evidence limits in [the regulation contract](../docs/ARACHNE_REGULATION.md).

### Gotchas and CES record

- Salience and workload values are explicit caller inputs, not inferred measurements. Default thresholds and effects are configuration examples, not biological facts.
- Prediction events store content digests rather than expected/observed JSON values. The metric is intentionally bounded and simple; object/string mismatch is binary and numeric distance is normalized absolute difference.
- The configured rule changes workspace admission only. It cannot grant Silk host authority, approve an action, promote memory, or bypass governance.
- Existing local active-inference and workspace experiments do not establish production reliability, causal learning, or general cognitive effectiveness. AR-11 owns an isolated validation of these observable policy effects.
