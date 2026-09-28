# Legacy Arachne through the Silk 2 boundary

**Task:** INT-02  
**Status:** diagnostic experiment completed; no legacy runtime interoperation claim

## Setup and evidence

The probe used the archived required fixture
`agents/episodic_loop_smoke.silk` (matrix entry 123), whose source is present
and hash-verified as
`9917d5f80f9479b8557843e5da5fe3d2e5d7a6432034c305a61ed5fea6cd5aaf`. The
recording reports a successful Stage 0 run with 15 trace records. The Rust
front end was asked to inspect the exact source through the same parser used by
`program.load`:

```sh
silk inspect silk2/reference/silk/source/agents/episodic_loop_smoke.silk
```

It failed at line 5 with `unsupported top-level declaration 'capability'`.
This is expected under the syntax migration rule: source `capability`
declarations are deprecated Arachne-specific syntax and cannot grant their own
authority. The fixture also uses active-inference host calls and a `learn`
block. `learn` is intended Silk 2 lifecycle syntax, but the current parser and
lowerer do not implement it yet. The public SRP path itself was exercised
separately by the Go round-trip example; that synthetic procedure successfully
used an explicit host descriptor and grant and returned its trace.

There is no legacy Zig Arachne source checkout or runnable legacy binary in
this workspace, so the old controller could not be launched against `silk
serve`. The archived recordings are outputs from the Silk 2 branch pin, not a
reproduction on the frozen Zig checkout; do not use them as frozen-runtime
interop results.

## Gap classification

| Finding                                                   | Classification                                     | Required adapter or follow-up                                                                                       |
| --------------------------------------------------------- | -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `capability` declaration in source                        | Historical Arachne coupling, intentionally removed | Move function descriptors and grant decisions to session creation.                                                  |
| `active_inference.*` and `contract_net.*` calls           | Host-owned Arachne behavior                        | Expose selected operations as explicit SRP host functions with schemas and effect/authority grants.                 |
| `learn` block rejected by the current parser              | Silk implementation gap against SILK-03/04         | Implement and lower the already specified learning lifecycle form before this fixture can be rerun.                 |
| No legacy Zig Arachne executable/source in this workspace | Experiment environment limitation                  | Repeat process-level interoperation only when the pinned legacy binary and its exact source revision are available. |

The result supports keeping the public process boundary: a compatible host can
negotiate, create a session, provide an explicit capability, run a procedure,
and ingest traces without shared Rust memory. It does not establish that legacy
Arachne can consume the current Silk subset, and it does not establish the
corpus compatibility gate.
