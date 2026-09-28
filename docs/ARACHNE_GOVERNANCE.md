# Arachne governance contract

Governance evaluates whether a consequential proposal is eligible under a
versioned host policy. It runs above Silk runtime authorization: an approved
Arachne decision does not create a Silk capability or bypass a Silk check, and
Silk authorization alone does not satisfy Arachne policy.

The Go package `internal/governance` covers explicit classes for external
effects, high-impact actions, memory mutation, semantic promotion, identity
changes, procedure retention, and structural changes. Each configured rule names a target scope,
an independent approval threshold, and whether source evidence is required.
The evaluator rejects unknown classes, out-of-scope targets, missing evidence,
unauthorized or duplicate approvers, duplicate approval IDs, stale proposal or
policy bindings, and approvals dated before the proposal. It reports pending
until the threshold is reached. Any valid authorized rejection rejects the
proposal.

Every evaluation appends a governance event to the shared cognitive event
spine. The decision contains the proposal and policy digests, class, target,
action, payload digest, applicable rule, reasons, review records, and linked
source, evidence, and review event IDs. The raw action payload is not copied to
the decision event. The caller remains responsible for retaining proposal
content when needed for later audit and resolving linked IDs to their source
records.

## Trust and enforcement boundary

Approval records are supplied by the embedding host. The host must authenticate
the human or service represented by each approver ID and protect the approval
submission path. This package checks identity membership and digest binding; it
does not provide cryptographic signatures, reviewer authentication, policy
distribution, or tamper-resistant storage. Policy changes require constructing
a service with the new policy and yield a different policy digest.

An `approved` result means only that the proposal passed this policy
evaluation. The package does not execute it, reserve its target, prevent a
caller from bypassing the evaluator, or make an external system atomic with the
event spine. Applications must route governed operations through this decision
and then through their normal Silk-authorized execution path. Integrations
should record execution outcomes as separate events. The runnable
[`governance` example](../arachne/examples/governance/main.go) shows pending,
approved, and rejected decisions without executing any proposal.

These controls implement a deterministic policy boundary for an embedding
runtime. They do not demonstrate safety against malicious hosts, compromised
approvers, policy misconfiguration, or live-model adversarial behavior.
