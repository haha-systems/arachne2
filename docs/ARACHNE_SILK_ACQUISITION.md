# Arachne and Silk procedural acquisition

SRP 1.1 adds a public process boundary for the procedural acquisition path:

1. Arachne consolidates attributed episodes and selects an evidence-backed
   recurring pattern.
2. A caller supplies candidate source and metadata to Silk's
   `candidate.prepare`. Silk lowers the source, checks the selected entry,
   effect ceiling, and required authorities, and returns an ephemeral,
   revision-digested artifact. Preparation neither admits nor runs it.
3. Arachne evaluates a procedure-retention proposal against its governance
   policy. The proposal binds the exact Silk revision and source evidence.
4. Only after approval does Arachne call `registry.admit`, then `registry.retain`
   with evidence bound to that exact revision and governance decision.
5. `registry.run` invokes that retained revision through an existing Silk
   session. Normal host grants, effect checks, limits, and trace delivery still
   apply.

The [`silk-acquisition` example](../arachne/examples/silk-acquisition/main.go)
performs this full path. It records two identical successful episodes,
consolidates them into one exact-repeat pattern, prepares a small addition
procedure, approves its exact revision, retains it in Silk, and reuses it to
produce `5`. The example source is deliberately fixed. It demonstrates the
acquisition boundary and provenance path; Arachne does not synthesize code in
this example.

The `silk` CLI keeps the registry in memory, so admitted and retained
procedures disappear when that process exits. The retention record validates
shape and digest binding only. The host must authenticate approvers and ensure
protected operations go through governance; neither the SRP method nor Silk's
retention record proves a human reviewed the change. Registry admission and
retention are separate from runtime authority, and execution still uses the
current session's exact grants. This local deterministic run is not evidence
that a generated procedure is generally correct or improves task performance.
