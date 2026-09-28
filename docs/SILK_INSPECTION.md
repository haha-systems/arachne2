# Silk semantic inspection

**Task:** SILK-12  
**Command:** `silk inspect`

Inspect supported Silk source without executing it:

```sh
silk inspect examples/hello.silk
```

The JSON `silk.inspection.v1` report lists the provisional program key, exact
source SHA-256, procedures, parameter names, block and instruction counts,
entry blocks, the serialized semantic operations and terminators, direct
state/output/time/delay effects, static procedure dependencies, and host
function dependencies. Dynamic and host calls are conservatively marked
`unknown`. The current CLI has no active host catalog, so it reports authority
requirements as unresolved rather than inferring permissions from source.

Inspect an execution snapshot to see what happened:

```sh
silk inspect /tmp/hello-silk2.json
```

The report includes process and program output, result, replay data, the full
ordered trace, and event counts. Snapshot inspection requires the
`silk.execution_snapshot.v1` envelope written by `silk capture` or an adapter
that preserves that schema. Static inspection does not execute the program.

The current IR subset does not carry semantic descriptions, typed contracts,
or a host descriptor catalog. Inspection reports those gaps explicitly. Static
effect summaries are local to each procedure; call edges are shown but not yet
expanded transitively. The versioned identity contract is in
[procedure identity and lineage](SILK_PROCEDURE_IDENTITY.md).
