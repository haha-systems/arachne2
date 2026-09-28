# Silk procedure retention lifecycle

**Task:** SILK-18  
**API:** `ProcedureRegistry::retention_state` and `retain`

Silk models three lifecycle states: `ephemeral`, `candidate`, and `retained`.
A `PreparedCandidate` is ephemeral and is not visible in the registry. Explicit
admission makes an immutable artifact a candidate. Running that candidate has
no retention side effect. Only an explicit `retain` call can move a candidate
to retained.

Retention requires an actor, a reason, and passing
`silk.retention_approval.v1` evidence bound to the exact procedure ID and
revision digest. The registry records an append-only transition with sequence,
previous/next state, actor, reason, and evidence. A revision cannot be retained
twice or silently changed during retention; the artifact and its digest stay
immutable. A new revision starts as a candidate and must be separately
approved.

The evidence validator and digest are shape-checked, not cryptographically
authenticated. The caller is responsible for applying its governance policy
before issuing the approval record. Retention is a lifecycle decision and does
not grant host authority or authorize a particular execution.

`MemoryStore` keeps states and transitions in process memory. A persistent
`RegistryStore` must make candidate admission atomic and append the transition
and state change as one durable operation to preserve the audit history.
