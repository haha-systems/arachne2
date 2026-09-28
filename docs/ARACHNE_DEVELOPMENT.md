# Arachne provenance-bearing development

The first development target is a per-instance routing profile. Each profile
starts from an explicit map of route keys to weights in `[-1, 1]`. A
`RoutingRequest` becomes a governance proposal whose payload binds the
organism instance, expected profile revision, previous weight, and proposed
weight. Its target is scoped to that instance and route key.

`ApplyRoutingChange` checks that the proposal still matches the current
revision and value before asking governance. Pending or rejected decisions do
not change the profile. An approved decision is followed by a
`routing_change_applied` development event. That event records the new value,
revision, governance decision and event IDs, and source/evidence IDs; its parent
links include the approval event and evidence. The profile changes only after
the development event has been appended successfully. Thus a policy decision
does not itself mutate the instance, and each applied value has an inspectable
causal record.

`NewEngine` rebuilds the profile by folding matching change events over the
caller-supplied initial profile. This keeps the live projection aligned with
the event history and rejects gaps, stale before-values, and invalid recorded
weights. With a durable event store and the same initial profile, an embedding
host can recover the instance after restart. The current daemon's default
event store is in memory, so that configuration does not survive process
failure. Initial profiles are also host-supplied; they are not emitted as
development changes.

The [`development` example](../arachne/examples/development/main.go) creates
two instances with equivalent initial routing weights. Different evidenced
experiences lead to separately approved changes, so their snapshots diverge;
recreating the engines from the same event history reconstructs both profiles.

This is a deterministic demonstration of scoped change, governance, and
provenance. It uses synthetic route weights and does not show improved task
performance, a useful learned policy, live-human authentication, or safe
adaptation in an adversarial environment. The embedding host still authenticates
approvers, routes protected updates through this engine, and provides the
ordinary runtime authorization needed for any later action. See the
[governance contract](ARACHNE_GOVERNANCE.md), [reference inventory](REFERENCE_INVENTORY.md),
and [compatibility decisions](COMPATIBILITY_DECISIONS.md) for the broader
evidence limits.
