# AR-13 — Implement governance for consequential changes

**Status:** Done
**Track:** Arachne  
**Depends on:** AR-01, AR-05  
**Source:** PRD §8, Arachne Phase 9

## Outcome

Arachne distinguishes technically executable behavior from behavior it is permitted to perform or retain.

## Work

Define review and decision paths for authority grants, memory mutation, identity changes, procedural retention, structural modification, and high-impact actions. Keep governance above Silk’s runtime authority checks.

## Acceptance criteria

- Governed decisions record proposal, policy/context, outcome, and provenance.
- Silk authorization remains necessary but does not replace Arachne governance.

## Completion

- Added a versioned policy evaluator for consequential action classes, explicit
  target scopes, evidence requirements, and independent approvals.
- Decisions bind proposal and policy digests, preserve review and provenance
  records, and append to the cognitive event spine. Approval means eligible;
  the package never executes a proposal.
- Documented the host authentication and enforcement boundary in
  [the governance contract](../docs/ARACHNE_GOVERNANCE.md). The runnable
  [governance example](../arachne/examples/governance/main.go) demonstrates
  pending, approved, and rejected paths.
- Limitation: approver authentication and ensuring all governed operations pass
  through this evaluator remain responsibilities of the embedding host.
