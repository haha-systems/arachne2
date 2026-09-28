# AR-15 — Connect Arachne development to Silk acquisition

**Status:** Done
**Track:** Arachne  
**Depends on:** AR-14, AR-13, SILK-14, SILK-17, SILK-18  
**Source:** PRD §8, Arachne Phase 11

## Outcome

Arachne can propose a recurring useful cognitive structure as a Silk procedure and retain it through governed admission.

## Work

Implement the experience → recurring structure → candidate → Silk validation → Arachne governance → registry admission flow. Keep all registry mutation behind Silk’s public interface.

## Acceptance criteria

- An acquired procedure was absent from the initial repertoire and can later be reused.
- The full acquisition, validation, governance, and admission path is traceable.

## Completion

- Extended SRP to 1.1 with external candidate preparation, exact artifact
  admission, revision-bound retention, and execution limited to retained
  entries. Existing session grants and runtime checks still apply.
- Added a Go client and runnable integration example covering repeated-memory
  consolidation, Silk synthesis checks, Arachne governance, registry retention,
  and later reuse with trace events.
- Documented the process-local registry and host-authentication limits in
  [the acquisition contract](../docs/ARACHNE_SILK_ACQUISITION.md) and
  [SRP 1.1](../docs/SILK_HOST_PROTOCOL.md).
- Limitation: the example's candidate source is fixed and the Silk registry is
  volatile. It demonstrates the public boundary and provenance flow, not
  automatic code generation or durable procedure learning.
