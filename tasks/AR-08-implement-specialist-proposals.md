# AR-08 — Implement specialist agents and proposals

**Status:** Ready  
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
