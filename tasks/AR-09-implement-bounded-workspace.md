# AR-09 — Implement bounded workspace coordination

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-08  
**Source:** PRD §8, Arachne Phase 6

## Outcome

Specialist proposals compete for bounded admission, broadcast, and reportability through an observable mechanism.

## Work

Implement selection and admission, broadcast to relevant agents, and tracing of why a proposal won or lost. Carry forward workspace experiment findings from the reference system.

## Acceptance criteria

- Capacity is bounded and selection decisions are recorded.
- A run shows which specialists contributed and why selected content was broadcast.

## Completion notes

- Added a bounded per-interaction workspace with explicit open, submit, select, broadcast, and inspection operations. Run, proposal, recipient, admission, and payload limits fail explicitly without silent eviction.
- Selection admits up to configured capacity using the declared confidence score, then specialist ID and proposal ID as deterministic tie-breakers. Reports explain every selected, over-capacity, and failed proposal.
- Broadcast requires a reason and explicit unique recipients, sends the admitted proposal records plus selection rationale, and captures per-recipient delivery results in both the report and the event spine.
- Updated the runnable two-specialist example to demonstrate collection, bounded admission, and broadcast without executing requested actions. See [the workspace contract](../docs/ARACHNE_WORKSPACE.md).

### Gotchas and CES record

- Confidence is self-declared, not verified quality evidence. It is a visible baseline policy and should be replaced only when a concrete workload supports a different selection rule.
- Broadcast may partially deliver before an error; the returned report preserves per-recipient outcomes, and the caller's context sets the mailbox wait bound.
- Workspace reports are process-local and bounded. Earlier synthetic workspace/contract-net results do not demonstrate production scale or network fault tolerance; the reference inventory retains those limitations.
- A selected/broadcast proposal is still only a candidate. This path has no executor and cannot grant Silk authority or bypass governance.
