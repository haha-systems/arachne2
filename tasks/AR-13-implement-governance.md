# AR-13 — Implement governance for consequential changes

**Status:** Ready  
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
