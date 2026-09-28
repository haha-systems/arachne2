# AR-11 — Validate regulatory effects on cognition

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-10  
**Source:** PRD §8, Arachne Phase 7

## Outcome

Regulatory state measurably changes cognitive behavior through documented mechanisms.

## Work

Create controlled scenarios that vary regulatory state and record changes in attention, routing, or action pressure. Compare results with prior active-inference findings.

## Acceptance criteria

- At least one regulatory signal causes a repeatable, observable behavioral change.
- The event history explains the signal and resulting change.

## Completion notes

- Added [a controlled regulation experiment](../docs/ARACHNE_REGULATION_EXPERIMENT.md) using fixed proposals and one-signal-at-a-time baseline, high-load, high-action-pressure, high-surprise, and high-salience conditions.
- Ran three independent repetitions per condition. Baseline admitted both proposals at capacity two; load and action pressure separately reduced capacity to one; surprise and salience separately required evidence and admitted only the evidence-backed proposal.
- Each experiment run prints the regulation and selection event IDs; selection records include signal reasons and parent links to regulation/prediction-error events.
- Results validate the deterministic mechanism only. They do not show improved task quality, causal learning, biological validity, or production-scale behavior.

### Gotchas and CES record

- The unsupported high-confidence candidate intentionally shows that low-regulation ranking trusts declared confidence. High surprise or salience changes that behavior by requiring evidence; these are inspectable policy effects, not quality guarantees.
- The experiment uses fixed synthetic proposals and caller-supplied signals. It isolates implementation behavior but says nothing about whether an autonomous organism would observe or estimate these signals correctly.
- Repeated output is tied to the exact documented policy and fixture. Threshold tuning or broader performance claims require a new workload and preserved source evidence.
