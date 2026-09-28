# Regulation effect experiment

This controlled in-process experiment varies one input signal at a time over a
fixed pair of proposals and a workspace capacity of two. It runs three
independent repetitions for each condition. The proposals are intentionally
fixed: `unsupported` has confidence `0.95` and no evidence link; `supported` has
confidence `0.7` and cites the source event. The default policy is used without
changing thresholds.

| Condition            | Changed input                     | Effective capacity | Evidence required | Admitted specialist(s) |
| -------------------- | --------------------------------- | -----------------: | ----------------- | ---------------------- |
| Baseline             | none                              |                  2 | no                | unsupported, supported |
| High load            | 2 active / 2 slots                |                  1 | no                | unsupported            |
| High action pressure | 2 pending / 2 slots               |                  1 | no                | unsupported            |
| High surprise        | expected and observed JSON differ |                  2 | yes               | supported              |
| High salience        | caller declares 0.9               |                  2 | yes               | supported              |

All five conditions produced the same capacity, evidence requirement, and
admission result in each of the three repetitions. The executable prints the
regulation and selection event IDs per run. Each selection event includes its
policy reasons and is causally linked to the corresponding regulation and
prediction-error events.

Run the scenario with:

```sh
cd arachne
go run ./examples/regulation-experiment
```

This validates only the deterministic implementation path from explicit
signals to workspace admission. It does not validate whether these signals or
thresholds improve an agent's task performance, and it does not establish
general cognition, causal learning, production reliability, or a biological
interpretation of the values. The prior local active-inference findings remain
bounded to their workloads as summarized in the
[reference inventory](REFERENCE_INVENTORY.md).
