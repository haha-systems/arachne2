# Arachne cognitive event spine

Arachne records process and agent activity through one append-only event model.
Each event has schema version `arachne.cognitive_event.v1`, an organism-scoped
event ID, a monotonically increasing sequence, a UTC wall-clock timestamp,
optional agent/session/correlation attribution, parent event IDs, a kind, and a
JSON payload. The sequence is the authoritative order; timestamps describe
wall-clock observation and do not establish a total order.

## Early interaction

The agent router assigns every message a sequence and a correlation ID. A
`SilkProcedureAgent` configured with `Daemon.Events()` records a request as a
`perception`, starts an `activation`, records each structured Silk trace event
as a `silk_trace`, then records the returned result or failure as an `action`.
Parent IDs form a chain through these records. Each Silk record also retains
the Silk `trace_id`, trace sequence, call correlation IDs, procedure, and trace
event kind in its `silk` link. The agent response continues to carry the value,
output, trace, and any runtime error.

Other kinds in the shared vocabulary include memory, proposals, selections,
decisions, prediction errors, replay, regulation, governance, and development.
They can use the same `Spine.Emit` path as those subsystems arrive; the event
store does not require a private tracing implementation for each subsystem.
Daemon startup and shutdown are currently represented as development events.

## Storage and limits

`EventStore` defines append and sequence-based reads. The daemon continues to
use a bounded in-memory implementation with a default capacity of 10,000
events. Embedders can instead construct a bounded `FileEventStore` with
`OpenFileEventStore(path, organismID, capacity)`. It stores a versioned
`arachne.event_log.v1` header followed by append-only JSON Lines records. The
header binds the log to one organism; each record wraps the unchanged
`arachne.cognitive_event.v1` event and a CRC32 checksum of its JSON encoding.

Each successful append is synced before it is published to readers. Reopening
validates the header, organism, event schema, checksums, and strict sequence
order, then rebuilds the in-memory read index. An incomplete final line is
truncated and reported by `RepairInfo`; malformed complete lines and earlier
corruption return typed errors without repairing the file. Files are secured
with 0600 permissions. The file store is bounded, never evicts, and returns
`ErrEventStoreFull` at capacity. It assumes a single writer process.

CRC32 detects accidental record corruption; it is not authentication or
tamper resistance. The store validates sequence numbers supplied by the
unchanged `Spine` and does not assign them. A newly constructed `Spine` starts
its local sequence at zero, so this store does not provide fresh-spine N+1
append continuation. Restart recovery currently means replaying stored events
through `Spine.Read` into the development routing profile; other runtime state
is not rebuilt. See [development restart recovery](ARACHNE_DEVELOPMENT.md#restart-recovery).

Payloads preserve supplied JSON, and Silk trace payloads can include host-call
arguments and results. Embedders should avoid secrets in traced values or add
redaction before recording sensitive interactions. Timestamps use the local
process clock converted to UTC; ordering comes from the sequence, not clock
synchronization.
