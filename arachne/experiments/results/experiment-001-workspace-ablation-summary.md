# Experiment 001 initial validation

Run date: 2026-09-29. Configuration: [workspace-ablation.json](../workspace-ablation.json). Machine-readable trial records: [experiment-001-workspace-ablation.jsonl](experiment-001-workspace-ablation.jsonl).

This 20-seed paired batch (40 trials) validates runner behavior only. It does not support a scientific claim about workspace utility.

| Condition | Successes | Success rate | Average steps | Average events | Average runtime | Invalid |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Workspace enabled | 7/20 | 35% | 6.00 | 15.00 | 0.395 ms | 0 |
| Workspace disabled | 0/20 | 0% | 5.00 | 1.00 | 0.112 ms | 0 |

Enabled trials recorded five workspace publications, one broadcast, and two workspace reads (the broadcast's internal inspection plus the runner's final inspection). Disabled trials recorded zero publications, broadcasts, messages, or workspace reads. Every paired seed had the same generated task and expected answer.

| Seed | Enabled | Disabled |
| ---: | --- | --- |
| 1001 | success | failure |
| 1002 | failure | failure |
| 1003 | success | failure |
| 1004 | failure | failure |
| 1005 | success | failure |
| 1006 | success | failure |
| 1007 | failure | failure |
| 1008 | success | failure |
| 1009 | failure | failure |
| 1010 | failure | failure |
| 1011 | success | failure |
| 1012 | failure | failure |
| 1013 | failure | failure |
| 1014 | failure | failure |
| 1015 | failure | failure |
| 1016 | failure | failure |
| 1017 | failure | failure |
| 1018 | success | failure |
| 1019 | failure | failure |
| 1020 | failure | failure |

The run shows both successful and unsuccessful enabled outcomes, and the disabled execution completes normally without workspace behavior. The observed rates are smoke results for this synthetic task only.
