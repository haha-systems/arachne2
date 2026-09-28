# AR-04 — Integrate Arachne with the public Silk runtime

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-03, SILK-05, SILK-09, INT-01  
**Source:** PRD §8, Arachne Phase 3

## Outcome

An Arachne agent can run a Silk procedure through the same public contract available to other hosts.

## Work

Implement session creation, agent bindings, procedure execution, result extraction, grants, and Silk trace ingestion over the selected boundary. Expose Arachne capabilities only where justified.

## Acceptance criteria

- Integration uses the public versioned contract with no shared-memory access to Silk internals.
- Session authority and traces remain visible to Arachne callers.

## Completion notes

- Added the `silk serve` SRP 1.0 framed-stdio server for initialization,
  isolated sessions, program load/run, session close, nested host calls, and
  trace notifications.
- Added the Go `internal/silk` process client and `internal/agent` procedure
  binding. Catalog callbacks and grants are explicit; no Arachne internals are
  exposed automatically.
- Added a cross-language round-trip example proving result extraction, host
  callback dispatch, and trace ingestion over the public byte protocol.
- See [Arachne and Silk integration](../docs/ARACHNE_SILK_INTEGRATION.md) for
  supported methods and interoperability limitations.

### Gotchas

- The wire server does not implement `agent.step`, protocol cancellation, full
  JSON Schema validation, or concurrent in-flight host requests.
- Host argument mapping uses positional Silk values and `inputSchema.required`
  order, then sorted optional properties; keep host schemas explicit.
- Callers must create a session with the exact catalog and grants they intend
  to authorize. The binding returns errors and collected traces to its sender.
- Runtime traces contain arguments/results under the current trace schema; host
  callbacks should apply their data redaction policy before returning values.
