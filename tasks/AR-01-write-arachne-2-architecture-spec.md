# AR-01 — Specify the smallest coherent Arachne 2

**Status:** Done  
**Track:** Arachne  
**Depends on:** REF-02, REF-04  
**Source:** PRD §§8, 16

## Outcome

An architecture specification describes Arachne as one organism and justifies each retained subsystem.

## Work

Review old subsystems against research and engineering needs. Define the organism daemon, agents, event spine, Silk client, memory, workspace, regulation, governance, development, and deferred or removed components.

## Acceptance criteria

- Every core subsystem has a role and boundary.
- The document defines the smallest coherent organism and avoids prescribing implementation tickets prematurely.
- It incorporates important positive and negative research findings.

## Completion notes

- Specified the organism boundary, independent Silk protocol use, core subsystem roles and invariants, event-to-action cycle, smallest coherent initial organism, and deferred components in [the Arachne 2 architecture](../docs/ARACHNE2_ARCHITECTURE.md).
- Incorporated governance, constrained adaptation, active-inference/SWR, identity-memory, and negative/noisy measurement findings while retaining the limits of each local or synthetic result.
- Kept the document at architecture level and recorded open questions for the Go runtime, event persistence, and first integrated experiment rather than prescribing implementation tickets prematurely.
