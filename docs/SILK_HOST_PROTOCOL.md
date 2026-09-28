# Silk host protocol contract

**Task:** SILK-05  
**Protocol:** Silk Runtime Protocol (SRP)  
**Initial version:** `1.0`

SRP is the language-independent boundary between a Silk runtime process and any host application. The host may provide filesystem, language-model, retrieval, memory, or other functions. Silk does not depend on a particular host implementation. Arachne is one possible host.

The protocol reuses JSON-RPC 2.0 request/response semantics and MCP-shaped function descriptors (`name`, `description`, JSON Schema `inputSchema`, an effect set, and an authority identifier). Silk uses the term **host function**; host functions are not Silk core builtins. Declaring a function and authorizing a call are separate operations. SILK-06 defines grants and the authority decision.

## Transport and framing

The runtime runs as a subprocess. Version 1 transport is bidirectional stdio: each message is a UTF-8 JSON-RPC 2.0 object framed by a four-byte unsigned big-endian length followed by exactly that many bytes. The length excludes the prefix. The receiver rejects a zero-length, oversized, malformed UTF-8, malformed JSON, or truncated frame with a protocol error and closes the connection when it cannot safely resynchronize.

The initial default maximum frame size is 16 MiB. Implementations may expose a lower configured limit; peers must not assume an unlimited frame. No log line, debug message, or program output may be written into the framed protocol stream. A later Unix-domain socket transport carries the same frames and messages without changing method semantics.

## Version negotiation

After transport setup, the host sends `initialize` with `protocol_version: "1.0"` and supported optional features. The runtime returns its version and features. Unknown major versions are refused with a JSON-RPC error and the connection closes. A peer may accept an older minor version when it ignores unknown optional fields and does not invoke unsupported features. Required fields may not be silently defaulted when their absence changes meaning.

Every session belongs to one initialized connection. A runtime process may serve multiple sessions on that connection, but session state is isolated. Version negotiation is connection-scoped; host-function catalogs, input, grants, trace sequence, and pending requests are session-scoped.

## JSON-RPC envelope

Requests follow JSON-RPC 2.0:

```json
{ "jsonrpc": "2.0", "id": 1, "method": "session.create", "params": {} }
```

Successful responses contain exactly one `result`; failed responses contain exactly one `error` object with a stable numeric `code` and structured `data`. Notifications omit `id` and receive no response. Request IDs are strings or integers and are unique among outstanding requests on a connection. Null IDs are not used.

The host is the controller and sends lifecycle requests. The runtime may issue a nested `host.call` request while processing a host request; the host must service it and return the matching response before the blocked runtime request can complete. Responses are correlated by request ID, not by arrival order. Implementations must support interleaved notifications and responses without crossing session boundaries.

## Session creation and host-function catalog

`session.create` establishes a new isolated execution context. The host supplies a unique `session_id`, optional input, function descriptors, grant identifiers, limits, and trace preferences. SILK-06 defines grant meaning; this request carries the session's authorization context without interpreting it here.

```json
{
  "session_id": "session-42",
  "input": { "task": "summarize", "text": "..." },
  "host_functions": [
    {
      "name": "language.summarize",
      "description": "Summarize a text value",
      "inputSchema": {
        "type": "object",
        "properties": { "text": { "type": "string" } },
        "required": ["text"],
        "additionalProperties": false
      },
      "effects": ["host_read"],
      "authority": "language.summarize"
    }
  ],
  "grants": [
    {
      "authority": "language.summarize",
      "effect_ceiling": ["host_read"]
    }
  ],
  "limits": { "fuel": 100000, "call_depth": 128, "timeout_ms": 30000 },
  "trace": { "level": "standard" }
}
```

Function names are stable, case-sensitive, dot-separated identifiers. A descriptor is a declaration of a callable interface, not executable code. Names are unique within a session. Duplicate names, invalid schemas, unsupported effect labels, duplicate session IDs, or invalid resource limits fail creation atomically; no partially usable session is left behind.

`inputSchema` is a JSON Schema object that constrains call arguments. A descriptor may include an output schema in a versioned extension. Unknown required descriptor fields/features fail closed; unknown optional fields may be ignored. The `effect` and `authority` fields are descriptive protocol data consumed by the runtime and the later effect/grant checks. A host implementation is responsible for validating returned values against any declared output schema.

The response confirms the session and returns the accepted protocol version, function catalog digest, and runtime limits:

```json
{
  "session_id": "session-42",
  "protocol_version": "1.0",
  "catalog_digest": "sha256:…",
  "limits": { "fuel": 100000, "call_depth": 128, "timeout_ms": 30000 }
}
```

No session is created on a failed response.

## Lifecycle methods

| Method           | Direction                   | Required behavior                                                                                                                                        |
| ---------------- | --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `initialize`     | host → runtime              | Negotiate protocol version and optional features before creating sessions.                                                                               |
| `session.create` | host → runtime              | Create isolated state from input, host-function catalog, grants, limits, and trace settings.                                                             |
| `program.load`   | host → runtime              | Load source or a supported serialized program into the named session. The runtime returns a stable `program_id` and diagnostics; load does not run code. |
| `procedure.run`  | host → runtime              | Invoke a named procedure with JSON arguments and return its JSON-compatible result or structured execution error.                                        |
| `agent.step`     | host → runtime              | Run the policy phase and, when requested and outcome input is supplied, the optional learn phase according to SILK-03 lifecycle semantics.               |
| `session.close`  | host → runtime              | Close the session, discard session-local state, cancel its outstanding work, and reject subsequent session operations.                                   |
| `host.call`      | runtime → host              | Invoke a previously declared function for one session; the host returns a JSON-compatible result or structured host error.                               |
| `trace.emit`     | runtime → host notification | Deliver one structured trace event. It is not a program result and carries no request response.                                                          |

`program.load`, `procedure.run`, and `agent.step` require a live session ID. A method called before initialization, with an unknown session, or after close fails with a typed protocol/session error. Loading a second program into a session must follow explicit replace semantics: the runtime refuses replacement while an invocation is active and resets program-owned state only after a successful load.

## Host-call request and response

The runtime invokes a host function by making a JSON-RPC request to the host:

```json
{
  "jsonrpc": "2.0",
  "id": "call-7",
  "method": "host.call",
  "params": {
    "session_id": "session-42",
    "call_id": "call-7",
    "function": "language.summarize",
    "arguments": { "text": "..." },
    "deadline_ms": 30000
  }
}
```

The host returns either a `result` value or an `error` object, never both. The runtime validates argument shape against the declared input schema before sending the call. Authority is checked before dispatch (SILK-06). A host failure is returned to the Silk caller as `HostError` with the stable host error code, safe message, retry hint, and opaque diagnostic data when provided.

The current Rust stdio server maps positional Silk arguments to object properties using the descriptor's `required` order followed by sorted optional property names. It validates basic JSON types and nested object properties. Full JSON Schema validation, output validation, retry hints, and opaque diagnostic propagation remain unimplemented; hosts should use schemas within the supported subset until those features are added.

Hosts must not execute a duplicate call merely because they see a repeated `call_id`; the identifier is unique per session. The runtime does not automatically retry a call whose outcome is unknown after a transport failure. This avoids duplicating non-idempotent actions. Cancellation is best-effort: a deadline/cancel notification may tell the host to stop, but it cannot promise rollback of an already performed host effect.

The protocol does not include secrets or ambient authority by implication. Credentials remain with the host. Host errors and traces must redact values according to host policy before serialization.

## Results and errors

JSON-compatible values follow SILK-03. Procedures, non-finite numbers, and other non-JSON runtime internals cannot cross SRP. The runtime rejects unsupported output with `SerializationError` rather than emitting an implementation-specific placeholder.

Protocol errors use stable categories in `error.data.kind`:

| Kind                                | Meaning                                                                                 |
| ----------------------------------- | --------------------------------------------------------------------------------------- |
| `UnsupportedProtocolVersion`        | Unknown major version or required feature.                                              |
| `InvalidFrame` / `FrameTooLarge`    | Invalid or unsafe framing.                                                              |
| `InvalidRequest` / `InvalidParams`  | Invalid JSON-RPC envelope or method parameters.                                         |
| `UnknownMethod`                     | Method is not part of the negotiated contract.                                          |
| `SessionNotFound` / `SessionClosed` | Session is absent or no longer usable.                                                  |
| `ProgramNotLoaded`                  | Method requires a program that has not loaded successfully.                             |
| `HostFunctionNotDeclared`           | Function name is not in that session's catalog.                                         |
| `AuthorityDenied`                   | Function is declared but not authorized; SILK-06 specifies details.                     |
| `HostError`                         | Host function returned a structured failure.                                            |
| `ExecutionError`                    | Silk evaluation failed; nested data carries SILK-03 error category and source location. |

The JSON-RPC numeric code identifies broad protocol/runtime failure; `data.kind` is the stable machine-readable category. Human-readable messages may improve without changing the category. No peer should branch on message text.

## Tracing and correlation

Each host call has a unique `call_id`, session ID, and request ID. Trace events refer to those IDs and record the function name, effect/authority decision, call phase, result or error, and elapsed duration. Trace ordering is monotonic per session and does not define ordering between sessions. `trace.emit` is a notification; loss of a trace notification must be surfaced by a sequence gap or trace-integrity error, not silently represented as a complete trace.

Arguments/results may contain sensitive user data. Redaction metadata must distinguish an omitted/redacted payload from an actual null. The trace schema is versioned independently from JSON-RPC method envelopes, following the existing `silk.builtin_trace.v1` lesson that per-interpreter sequence and emission order are different concepts.

## Conformance and compatibility

A conforming implementation must reject unknown protocol major versions, isolate sessions, validate method parameters and host-call arguments, correlate nested requests correctly, and preserve stable error kinds. A protocol implementation must pass interoperability tests with a non-Arachne host before Arachne integration begins.

Initial transport is stdio only. Unix socket transport, streaming host-call results, remote network transport, dynamic catalog mutation, and protocol-level batching are deferred. Adding any of them requires a versioned feature or protocol revision and must not alter the meaning of existing methods.

The contract intentionally does not describe how a filesystem, LLM, retrieval service, or Arachne subsystem implements a host function. Those remain host-owned details behind descriptors and the request/response contract.
