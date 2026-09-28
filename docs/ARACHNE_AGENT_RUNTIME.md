# Arachne agent lifecycle and message transport

**Task:** AR-03  
**Implementation:** [`internal/agent`](../arachne/internal/agent/) and [`internal/daemon`](../arachne/internal/daemon/).

## Lifecycle

An embedding process creates a daemon, registers a fixed set of agents, and runs the daemon with a parent context. Registration is only valid before startup. The daemon starts one goroutine per agent and passes each a context, a bounded inbox, and a sender bound to that agent's ID.

```go
worker := agent.Func(func(ctx context.Context, inbox <-chan agent.Message, sender agent.Sender) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case message := <-inbox:
            // Handle message and return results through sender.Send.
            _ = message
        }
    }
})
```

The current router does not close inbox channels. Agent code should use the context cancellation branch and return promptly. This avoids send/close races. A first non-cancellation agent error is retained, cancels the remaining agents, and is returned after they exit. An embedding caller may use `Wait` for natural completion or `Stop` to cancel and join. Daemon shutdown uses its configured timeout; if an agent ignores cancellation, shutdown returns a timeout error instead of claiming it stopped cleanly.

## Message contract

Messages are point-to-point JSON payloads with a stable kind, sender, recipient, and monotonically assigned sequence number. Each destination has a bounded FIFO inbox. Concurrent sends to one destination are serialized; messages from different destinations have no shared total-order guarantee. Sequence numbers identify accepted send attempts and do not define a global cognitive timeline. AR-05 owns the organism event spine and its causal/order semantics.

Backpressure is explicit: a sender blocks when the recipient inbox is full until delivery, sender-context cancellation, or router shutdown. Unknown recipients and sends after stop return typed sentinel errors. Payload bytes are copied before delivery so a sender cannot mutate the delivered message through a reused buffer.

## Deterministic harness

Tests can register fixed agents before startup, send known JSON payloads, and coordinate their work with channels and contexts. Startup launches IDs in sorted order, but Go goroutine scheduling is concurrent; tests should synchronize on messages/barriers rather than assume a particular scheduling interleaving. The agent tests cover per-recipient order, sender/recipient attribution, failure propagation, cancellation/join, and rejection after router close.

## Scope and gotchas

- The supervisor is a lifecycle and transport primitive. It does not select specialists, admit workspace proposals, persist events, or make governance decisions.
- Every agent must honor its context. Go cannot forcibly stop an uncooperative goroutine; the daemon enforces a shutdown deadline and reports incomplete shutdown.
- The command currently registers no agents and listens on no network port. The public embedding API exercises agents; AR-04 adds the Silk client and later tasks add cognition/event persistence.
