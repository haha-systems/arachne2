# Arachne and Silk runtime integration

**Task:** AR-04  
**Boundary:** SRP 1.1 over framed stdio

The Silk CLI now runs as a protocol peer:

```sh
silk serve
```

It uses four-byte big-endian length-prefixed JSON-RPC frames on stdin/stdout.
Logs stay on stderr. Arachne's `internal/silk` client starts this process,
negotiates SRP 1.1, creates isolated sessions, loads source, invokes procedures,
services nested `host.call` requests, and collects `trace.emit` events. It
serializes access to one process stream so responses and nested calls cannot
cross sessions.

The host must pass an explicit catalog of `HostCapability` descriptors and
callbacks plus explicit grants. The client exposes no ambient Arachne access.
Silk validates the declared function, session grant, and effect ceiling before
it emits `host.call`; the client routes that request only to the callback
registered for its session. Consequential callbacks must still apply Arachne
governance. The client does not duplicate Silk's grant policy.
`internal/agent.SilkProcedureAgent` binds a loaded session to agent messages of
kind `silk.procedure.run` and returns the JSON result, output, and trace (or the
error and trace) as `silk.procedure.result`.

The black-box Go round-trip example exercises the public boundary without
sharing Rust memory:

```sh
cd silk2 && cargo build -p silk-cli
cd ../arachne && go run ./examples/silk-roundtrip ../silk2/target/debug/silk
```

The current wire service implements initialize, session create/close, program
load, procedure run, candidate preparation, registry admission/retention/run,
nested host calls, and trace notifications. Registry storage is volatile for
the Silk process lifetime. `agent.step`, cooperative cancellation messages,
full JSON Schema validation, and concurrent requests on one stream remain unsupported. Host `inputSchema` validation currently covers
basic JSON types and object properties; positional Silk arguments map by the
schema's `required` order followed by sorted optional property names. Procedure
input schemas are not represented in the current IR. The Arachne client
serializes one process stream; use separate clients when independent concurrent
runtime work is needed. Calls made from a host callback using its supplied
context return `silk.ErrReentrantCall`; callbacks must not re-enter the same
client. Requests waiting for the serialized stream can be cancelled while
queued. Cancelling after protocol I/O begins terminates the Silk process to
unblock the framed read. If a host callback was already dispatched, its external
effect may have occurred; callbacks must honor their context, and the client
does not retry or imply rollback. The client instance cannot be reused after
in-flight cancellation or transport loss.
