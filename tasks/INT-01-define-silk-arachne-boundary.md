# INT-01 — Define the Silk and Arachne boundary

**Status:** Done  
**Track:** Integration  
**Depends on:** SILK-01, AR-01  
**Source:** PRD §§2, 9, 12

## Outcome

The projects share an explicit, versionable contract and a deliberate first transport boundary.

## Work

Specify sessions, procedure execution, host capabilities, grants, results, errors, and trace exchange. Select a first local IPC, structured RPC, or subprocess protocol. Treat direct FFI as a separately justified later option.

## Acceptance criteria

- Silk and Arachne can evolve independently behind a documented public contract.
- The transport choice supports black-box testing and does not require shared internal state.

## Completion notes

- Selected the subprocess with framed bidirectional stdio transport already specified by SRP 1.0; FFI and shared mutable storage remain deferred.
- Specified data ownership, session/call flow, host-grant and organism-governance separation, versioning, failure/replay rules, and black-box acceptance in [the integration boundary](../docs/SILK_ARACHNE_BOUNDARY.md).
- Aligned the SILK-05 JSON example with SILK-06 effect-set descriptors and authority/effect-ceiling grant objects, eliminating the prior single-effect/string-grant mismatch.
