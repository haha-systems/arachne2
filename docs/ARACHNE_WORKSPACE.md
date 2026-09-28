# Arachne bounded proposal workspace

The workspace coordinates one interaction through four explicit steps:
`Open`, `Submit`, `Select`, and `Broadcast`. Every run has stable run and
interaction IDs, a session, and its source event links. Submitted proposals
must match the interaction, carry their own proposal/event IDs and specialist
attribution, and come from a specialist that has not already contributed to
that run. Failed proposals are retained in the report but are not eligible for
admission.

## Bounds and selection

`workspace.DefaultConfig()` caps the in-memory workspace at 128 runs, 16
proposals per run, 4 admitted proposals, 16 broadcast recipients, and 4 MiB
per broadcast payload. A limit returns an error; old reports are never evicted
silently. Callers can set lower bounds for a workload. The supervisor's
mailboxes add delivery backpressure around proposal and broadcast messages.

Selection orders eligible candidates by their declared confidence, then
specialist ID, then proposal ID. It admits up to the run capacity and records
the reason for every selected, over-capacity, or failed proposal. Confidence is
a declared ranking input, not verified quality evidence; this simple policy is
observable and replaceable when a measured workload justifies another rule.
Salience, regulation, learned ranking, fairness optimization, and governance
are not inferred by the workspace. An explicit regulation policy may pass a
reduced capacity or evidence requirement through `SelectWithPolicy`; those
signal event IDs and reasons are retained with the selection.

## Broadcast and inspection

Broadcast requires an explicit reason and a unique bounded recipient list. It
sends the selected proposal records together with the full selection entries,
policy, and rationale. Each delivery result is retained in the report and
recorded as a decision event linked to the broadcast plan. Partial delivery
errors are returned alongside the report; a caller's context controls any
mailbox wait.

`Inspect` returns an immutable report containing every contributor, proposal,
selection reason, and delivery result. The cognitive event spine carries the
same sequence of opened run, admitted proposal, selection, broadcast plan, and
delivery records. A proposal broadcast remains information for another agent;
it does not invoke requested actions or grant authority.

The report map is process-local and bounded. It does not recover after restart
or use an external durable workspace store. Earlier synthetic workspace and
contract-net measurements are not evidence of production reliability or
scaling; the [reference inventory](REFERENCE_INVENTORY.md) records those
limits. The current confidence rule is therefore a transparent baseline, not a
claim that one proposal metric is generally superior.
