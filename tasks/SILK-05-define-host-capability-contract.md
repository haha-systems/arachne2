# SILK-05 — Define the host capability contract

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-03  
**Source:** PRD §7, Silk Phase 3

## Outcome

Hosts have a language-independent contract for providing external functionality to Silk.

## Work

Define capability identifiers, provider calls, request and result shapes, runtime session context, and host error behavior. Keep implementations such as filesystem or LLM access in the host.

## Acceptance criteria

- A program can name an external capability without depending on its host implementation.
- The contract supports multiple hosts and can be versioned at the boundary.

## Completion notes

- Added [the host protocol contract](../docs/SILK_HOST_PROTOCOL.md) with namespaced function descriptors, JSON-RPC 2.0 framing/version negotiation, request/result/error shapes, session scope, and host-call correlation.
- Kept host implementations external and documented declared functions separately from SILK-06 authorization grants.
- Used the PRD's four-byte length-prefix transport decision as the base and specified initial framing limits and failure handling.
