# AR-02 — Build the Go runtime skeleton

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-01  
**Source:** PRD §8, Arachne Phase 2

## Outcome

Arachne can run as a supervised, cancellable long-lived process.

## Work

Implement process lifecycle, configuration, contexts and cancellation, structured logs and traces, and clean shutdown. Keep advanced cognition out of this foundation.

## Acceptance criteria

- The daemon starts, reports readiness, responds to cancellation, and shuts down cleanly.
- Runtime configuration and lifecycle behavior are documented and observable.

## Completion notes

- Added the independent Go module at [`arachne/`](../arachne/), with a configurable `arachned` command, validated organism ID/log level, context-based lifecycle, and JSON readiness/shutdown events.
- The command handles `SIGINT` and `SIGTERM` through a cancellation context and exits cleanly. Agent workers, network transport, and cognitive tracing remain in AR-03/AR-05.
- Documented configuration and local formatting, test, and lint commands in the Arachne README.
- Verified with Go 1.23.12: `gofmt`, `go test ./...`, and strict `golangci-lint run` pass. A built binary emitted `daemon.ready`, handled `SIGTERM`, emitted `daemon.stopped`, and exited successfully.
- CES follow-up during INT-03 module identity selection: after changing the module path, Go could not resolve the old `arachne2/internal/daemon` import. The import still referenced the staging module name; updating it to `github.com/haha-systems/arachne2/internal/daemon` restored package resolution.
