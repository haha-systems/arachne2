# Arachne memory consolidation

Consolidation is an explicit, deterministic transformation over a selected set
of stored episodes. The first rule, `episode.exact_repeat.v1`, groups episodes
that have the same kind and the same normalized JSON content. A caller chooses
the episode query and a minimum support of at least two. The rule does not infer
paraphrases, cause, general truth, or semantic equivalence.

Each applied pass creates a `ConsolidationRun` with the rule version, minimum
support, selected episode IDs, output pattern IDs, time, and active/revoked
status. Each `ConsolidatedPattern` includes the canonical content, fingerprint,
support count, first/last observation times, tags, source episode IDs, and
source event IDs. Pattern retrieval requires a run ID and emits a memory event
with the returned IDs, making it possible to inspect which derived records
became available after a pass.

Repeated calls over the same rule, support policy, and selected episode IDs use
a stable run ID and return the existing pass. New episodes produce a new run
with a separate evidence set. `RevokeConsolidation` records an actor and reason
and removes the run's patterns from ordinary retrieval. An inspection call can
request revoked patterns to compare the prior and current available memory.
Revoke state is persisted; apply and revoke operations also emit replay events.

The store persists the run and all output patterns as one snapshot update. The
event spine and memory store are separate, so a post-persistence event failure
returns an error with the run ID and committed result. Callers should retrieve
that run before retrying. The current policy only consolidates exact repeats;
more interpretive rules need explicit evidence and evaluation before they are
added. Consolidated patterns are evidence summaries, not approved beliefs or
identity changes.
