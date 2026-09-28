# AR-08 — Implement specialist agents and proposals

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-05  
**Source:** PRD §8, Arachne Phase 6

## Outcome

Specialist agents can contribute candidate interpretations or actions through a common proposal contract.

## Work

Define specialist roles, activation inputs, proposal contents, and lifecycle. Route proposals through the event spine and preserve agent attribution.

## Acceptance criteria

- Multiple specialists can produce proposals for one interaction.
- Proposals are observable and do not directly execute consequential actions.

## Completion notes

- Added activation and proposal contracts with interaction IDs, explicit input/evidence, specialist and correlation attribution, confidence, candidate action intents, timestamps, and failure status.
- Added `SpecialistAgent` and `SpecialistFunc`. The wrapper validates bounds, attributes outputs to its registered specialist identity, and emits perception, activation, and proposal events through the shared spine.
- Multiple specialists can return proposals to one coordinator through bounded router mailboxes. Proposal actions are data only and have no execution capability; selection is left to AR-09's workspace.
- Added [the specialist contract](../docs/ARACHNE_SPECIALISTS.md) and a runnable two-specialist example that collects proposals and event records without executing intents.

### Gotchas and CES record

- Router delivery order is transport order only. It does not establish proposal quality, ranking, or admission; the later workspace must record its own selection rationale.
- Specialist identities are assigned by the registered wrapper, not trusted from handler output. Requested action payloads are limited to 32 entries and one MiB per serialized proposal.
- A proposal is an untrusted candidate even when confidence is high. It cannot grant Silk host authority, invoke a host function, or trigger a consequential action.
