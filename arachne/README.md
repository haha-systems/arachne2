# Arachne 2 Go runtime

This directory contains the independent Go process foundation for Arachne 2.
It does not depend on the legacy Zig implementation or the Rust Silk workspace.

## Run the daemon

```sh
go run ./cmd/arachned --organism-id local --log-level info
```

The daemon writes JSON lifecycle records to stdout. It emits `daemon.ready`
after configuration validation and `daemon.stopped` after it receives
`SIGINT`, `SIGTERM`, or cancellation from its runtime context. Agent workers
registered by the embedding process are supervised and joined during shutdown.
The command currently registers no workers and has no external network listener.
The daemon exposes a shared, bounded cognitive event spine through `Events()`;
the default in-memory history holds 10,000 events. See the
[event model](../docs/ARACHNE_COGNITIVE_EVENTS.md).

`--organism-id` must be nonempty. `--log-level` accepts `debug`, `info`, `warn`,
or `error`. `--mailbox-capacity` sets the bounded per-agent queue size;
`--shutdown-timeout` sets the maximum graceful wait. Invalid configuration and
command arguments exit with status 2.

`--memory-file PATH` persists episodic and candidate semantic memory in an
organism-scoped JSON snapshot. Without it, memory is process-local. See the
[attributed memory contract](../docs/ARACHNE_ATTRIBUTED_MEMORY.md).

## Checks

```sh
gofmt -w ./cmd ./internal
go test ./...
golangci-lint run
```

Lifecycle records are structured logs and entries in the shared event spine.
Embedding applications can use the versioned Silk
stdio client and register `SilkProcedureAgent` with the supervisor. See the
[integration contract](../docs/ARACHNE_SILK_INTEGRATION.md) and
[`silk-roundtrip` example](examples/silk-roundtrip/main.go).
See the [specialist proposal contract](../docs/ARACHNE_SPECIALISTS.md) and its
[`specialist-proposals` example](examples/specialist-proposals/main.go) for
multi-agent candidate work. See the [bounded workspace contract](../docs/ARACHNE_WORKSPACE.md)
for proposal selection and broadcast.
The [regulation contract](../docs/ARACHNE_REGULATION.md) explains how explicit
signals can adjust workspace admission with linked event evidence.
The [regulation experiment](../docs/ARACHNE_REGULATION_EXPERIMENT.md) compares
five isolated conditions over three repetitions.
The [governance contract](../docs/ARACHNE_GOVERNANCE.md) documents policy-bound
eligibility for consequential proposals and the host's authentication and
enforcement responsibilities; run `go run ./examples/governance` to inspect
pending, approved, and rejected decisions.
