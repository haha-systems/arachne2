# Silk and Arachne integration boundary

**Task:** INT-01  
**Protocol:** Silk Runtime Protocol 1.0  
**Related contracts:** [Silk host protocol](SILK_HOST_PROTOCOL.md), [effects and authority](SILK_EFFECTS_AUTHORITY.md), [Arachne architecture](ARACHNE2_ARCHITECTURE.md).

## First transport

Arachne launches Silk as a subprocess and exchanges length-prefixed JSON-RPC 2.0 messages over bidirectional stdio. Each message uses the four-byte unsigned big-endian frame from SRP 1.0. Stdout is protocol-only; Silk diagnostics and application logs use stderr or structured protocol trace notifications. This boundary is black-box testable and does not require Go/Rust FFI, shared memory, or shared database tables.

The framing and method semantics are defined by SILK-05. Unix sockets may carry the same messages later. Transport changes do not change who owns state or the meaning of protocol methods.

## Ownership

| Data or operation                                                                                                 | Owner                                                     | Cross-boundary representation                                                                        |
| ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Silk source, program load, procedure evaluation, session-local bindings, semantic IR, and Silk procedure registry | Silk                                                      | `program.load`, `procedure.run`, and protocol results/errors                                         |
| Organism identity, agent lifecycle, memory, workspace proposals, regulation, and governance state                 | Arachne                                                   | Explicit session input and host-function results; never direct Silk memory access                    |
| Host function implementation and credentials                                                                      | Arachne or another host                                   | Session descriptor plus nested `host.call`; secrets remain on the host side                          |
| Authority grants for a Silk session                                                                               | The controller creating the session, under Arachne policy | Explicit grant entries at `session.create`; Silk source cannot add or widen a grant                  |
| Runtime behavior and actual host-call decisions                                                                   | Silk, with Arachne recording received events              | Versioned `trace.emit` notifications correlated by session, request, call, and sequence IDs          |
| Consequential organism action or lasting organism change                                                          | Arachne governance                                        | Arachne evaluates and records approval before dispatching an authorized action or promoting a change |

## Session and call flow

1. Arachne starts the Silk process and sends `initialize` with SRP version `1.0` and supported optional features.
2. The Arachne controller creates a session with a unique ID, explicit input, host-function descriptors, effect ceilings/grants, limits, and trace preferences.
3. Arachne sends `program.load`, then invokes a procedure with `procedure.run` or the lifecycle method `agent.step`.
4. Silk evaluates in the session. For each external operation, it resolves the exact descriptor, validates arguments, enforces the active effect ceiling and matching session grant, checks session state, and only then sends nested `host.call` to Arachne.
5. Arachne executes only the named host function and returns a result or structured host error. Host functions do not receive ambient access to other Arachne subsystems.
6. Silk returns the procedure result or typed error and emits ordered trace events. Arachne records those events with its own cognitive event identifiers and keeps protocol sequence/call identity for audit.
7. Arachne closes the session. Silk releases session-local state; a later session has no inherited grants or mutable bindings.

For a host-owned consequential operation, Arachne's host function implementation performs its own governance check before mutation or external dispatch. Silk authority answers whether the session may call the declared host function; Arachne governance answers whether the organism may perform or retain the requested change. Both checks are required where both policies apply.

## Versioned data shape

A descriptor contains an exact function name, schema, nonempty authority identifier, and a set of stable effect labels. Grants are objects containing an exact authority identifier and an effect ceiling. A function is callable only when its name is declared and at least one exact-authority grant covers every declared effect. Unknown effect sets fail closed. `pure` is represented by an empty effect set; it is not a grant label.

```json
{
  "host_functions": [
    {
      "name": "language.summarize",
      "authority": "language.summarize",
      "effects": ["host_read"],
      "inputSchema": { "type": "object" }
    }
  ],
  "grants": [
    {
      "authority": "language.summarize",
      "effect_ceiling": ["host_read"]
    }
  ]
}
```

This example aligns SILK-05 session creation with the SILK-06 grant model and the Rust protocol types. JSON-RPC method fields and errors follow the SRP 1.0 specification. Trace event schema remains independently versioned from the JSON-RPC method envelope.

## Failure, cancellation, and replay rules

- Unsupported major protocol versions, invalid frames, invalid schemas, duplicate session IDs, and invalid limits fail closed before session use.
- A missing descriptor returns `HostFunctionNotDeclared`. A descriptor without a covering grant returns `AuthorityDenied`. Both occur before `host.call` is sent.
- A host error is surfaced with its stable category and safe host message. Silk and Arachne do not assume a failed response means no partial external effect occurred.
- A transport failure after dispatch has an unknown outcome. Neither side retries automatically unless a future operation contract explicitly defines idempotency.
- Cancellation is best-effort for work already dispatched. Governance and recovery policy decide whether to reconcile uncertain actions; the protocol does not promise rollback.
- Replay may return a previously recorded host result only for a matching session/program/call identity, arguments, and sequence position. Replay records never confer authority.

## Evolution and conformance

SRP method compatibility is versioned and negotiated at `initialize`. Trace schemas carry their own version. Arachne may use only negotiated methods and features. Optional fields may be ignored as SRP specifies; a changed security meaning requires a new feature or major protocol version.

Each implementation can be tested against a black-box peer. Before Arachne integration, Silk must interoperate with a simple non-Arachne test host. Before release, Arachne must start a Silk subprocess, create an isolated session, run a trivial procedure, receive a result and trace, exercise one granted host call, observe a denied call with no dispatch, and close the session cleanly.

Direct language bindings remain deferred. They may be proposed only with evidence that the protocol boundary blocks a concrete requirement that cannot be met with local stdio or Unix sockets.
