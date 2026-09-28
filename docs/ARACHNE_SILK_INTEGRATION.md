# Arachne and Silk runtime integration

**Task:** AR-04  
**Boundary:** SRP 1.0 over framed stdio

The Silk CLI now runs as a protocol peer:

```sh
silk serve
```

It uses four-byte big-endian length-prefixed JSON-RPC frames on stdin/stdout.
Logs stay on stderr. Arachne's `internal/silk` client starts this process,
negotiates SRP 1.0, creates isolated sessions, loads source, invokes procedures,
services nested `host.call` requests, and collects `trace.emit` events. It
serializes access to one process stream so responses and nested calls cannot
cross sessions.

The host must pass an explicit catalog of `HostCapability` descriptors and
callbacks plus explicit grants. The client exposes no ambient Arachne access.
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
load, procedure run, nested host calls, and trace notifications. `agent.step`,
cancellation messages, full JSON Schema validation, and concurrent requests on
one stream remain unsupported. Host `inputSchema` validation currently covers
basic JSON types and object properties; positional Silk arguments map by the
schema's `required` order followed by sorted optional property names. Procedure
input schemas are not represented in the current IR. The Arachne client
serializes one process stream; use separate clients when independent concurrent
runtime work is needed.
