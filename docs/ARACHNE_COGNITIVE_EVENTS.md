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

`EventStore` defines append and sequence-based reads. The daemon currently
provides a bounded in-memory implementation with a default capacity of 10,000
events. A full store returns `ErrEventStoreFull`; the spine never evicts or
drops records silently. Applications can read through `Spine.Read` and can
provide a durable store when constructing a spine directly. Durable recovery
of sequence state and atomic persistence are responsibilities for a future
store implementation.

Payloads preserve supplied JSON, and Silk trace payloads can include host-call
arguments and results. Embedders should avoid secrets in traced values or add
redaction before recording sensitive interactions. Timestamps use the local
process clock converted to UTC; ordering comes from the sequence, not clock
synchronization.
