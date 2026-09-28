# Silk2 compatibility matrix

**PRD:** docs/silk2/SILK_PRD.md §10 Stage 0, item 6; §7.2.
**Status:** complete — 200/200 corpus programs classified.
**Source:** docs/silk2/corpus/MANIFEST.json and docs/silk2/corpus/SHIM_CLASSIFICATION.json.

## Purpose

This matrix assigns exactly one compatibility class to every recorded Silk
program. Phase 4 measures its exit criteria against these classes: **100% of
required** programs must match the reference recording, and **at least 90%
of preferred** programs must match. legacy and abandoned programs remain
recorded for auditability but do not set the Phase 4 gate.

The matrix uses the manifest's exact program paths as identities. Worktree
snapshots therefore remain visible as rows even though they are not independent
source targets.

## Class definitions

| Class | Meaning |
|---|---|
| required | Current runtime, safety, or language fixture needed for the near-term proof and the Phase 4 gate. |
| preferred | Canonical evaluation, example, or lifecycle fixture that should remain compatible when practical; measured at ≥90% in Phase 4. |
| legacy | Historical or externally generated input retained for compatibility evidence, but not a blocking target. |
| abandoned | Non-canonical generated snapshot with no independent compatibility target; retained only to make the inventory complete and auditable. |

Host status is copied from Stage 0.5 and is intentionally separate from
compatibility class. A host-dependent program may still be required or
preferred when its live host behavior is part of Arachne integration.

## Summary

| Class | Programs |
|---|---:|
| required | 33 |
| preferred | 60 |
| legacy | 20 |
| abandoned | 87 |
| **Total** | **200** |

## Matrix

| # | Program | Compatibility class | Stage 0.5 host status | Rationale |
|---:|---|---|---|---|
| 1 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/active_inference_adaptive_routing_organism.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 2 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/brain_manager.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 3 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/cnp_leader.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 4 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/cnp_worker.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 5 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/codegen.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 6 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/critic.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 7 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/designer.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 8 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/episodic_loop_smoke.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 9 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/leader.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 10 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/manager.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 11 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_disabled_baseline.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 12 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_disabled_governance_actor.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 13 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_disabled_handoff.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 14 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_disabled_planner.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 15 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_disabled_reviewer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 16 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_governance_actor.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 17 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_handoff.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 18 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_planner.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 19 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/memory_reviewer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 20 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/researcher.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 21 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/summarizer.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 22 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/tester.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 23 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/zmq_leader.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 24 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/zmq_worker.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 25 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/zmq_worker_fast.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 26 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/zmq_worker_medium.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 27 | .claude/worktrees/nostalgic-blackwell-b181bf/agents/zmq_worker_slow.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 28 | .claude/worktrees/nostalgic-blackwell-b181bf/examples/active_inference_episodic_substrate.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 29 | .claude/worktrees/nostalgic-blackwell-b181bf/examples/active_inference_minimal.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 30 | .claude/worktrees/nostalgic-blackwell-b181bf/examples/active_inference_semantic_candidate.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 31 | .claude/worktrees/nostalgic-blackwell-b181bf/examples/demo.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 32 | .claude/worktrees/nostalgic-blackwell-b181bf/examples/mcp_agent.silk | **abandoned** | generic host / generic_host_unavailable | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 33 | .claude/worktrees/nostalgic-blackwell-b181bf/examples/swr_signal_lane_minimal.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 34 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/001_cross_agent_protected_change/governance.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 35 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/001_cross_agent_protected_change/observer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 36 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/001_cross_agent_protected_change/proposer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 37 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/001_cross_agent_protected_change/target.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 38 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/002_constrained_policy_adaptation/governance.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 39 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/002_constrained_policy_adaptation/observer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 40 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/002_constrained_policy_adaptation/proposer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 41 | .claude/worktrees/nostalgic-blackwell-b181bf/experiments/002_constrained_policy_adaptation/target.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 42 | .claude/worktrees/nostalgic-blackwell-b181bf/test_modules/math.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 43 | .claude/worktrees/nostalgic-blackwell-b181bf/test_modules/utils.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 44 | .claude/worktrees/organism-arc/agents/active_inference_adaptive_routing_organism.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 45 | .claude/worktrees/organism-arc/agents/brain_manager.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 46 | .claude/worktrees/organism-arc/agents/cnp_leader.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 47 | .claude/worktrees/organism-arc/agents/cnp_worker.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 48 | .claude/worktrees/organism-arc/agents/codegen.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 49 | .claude/worktrees/organism-arc/agents/critic.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 50 | .claude/worktrees/organism-arc/agents/designer.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 51 | .claude/worktrees/organism-arc/agents/episodic_loop_smoke.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 52 | .claude/worktrees/organism-arc/agents/first_real_usage_researcher.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 53 | .claude/worktrees/organism-arc/agents/leader.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 54 | .claude/worktrees/organism-arc/agents/manager.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 55 | .claude/worktrees/organism-arc/agents/memory_disabled_baseline.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 56 | .claude/worktrees/organism-arc/agents/memory_disabled_governance_actor.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 57 | .claude/worktrees/organism-arc/agents/memory_disabled_handoff.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 58 | .claude/worktrees/organism-arc/agents/memory_disabled_planner.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 59 | .claude/worktrees/organism-arc/agents/memory_disabled_reviewer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 60 | .claude/worktrees/organism-arc/agents/memory_governance_actor.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 61 | .claude/worktrees/organism-arc/agents/memory_handoff.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 62 | .claude/worktrees/organism-arc/agents/memory_planner.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 63 | .claude/worktrees/organism-arc/agents/memory_reviewer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 64 | .claude/worktrees/organism-arc/agents/researcher.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 65 | .claude/worktrees/organism-arc/agents/summarizer.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 66 | .claude/worktrees/organism-arc/agents/tester.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 67 | .claude/worktrees/organism-arc/agents/zmq_leader.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 68 | .claude/worktrees/organism-arc/agents/zmq_worker.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 69 | .claude/worktrees/organism-arc/agents/zmq_worker_fast.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 70 | .claude/worktrees/organism-arc/agents/zmq_worker_medium.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 71 | .claude/worktrees/organism-arc/agents/zmq_worker_slow.silk | **abandoned** | organism / dispatch_timeout | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 72 | .claude/worktrees/organism-arc/examples/active_inference_episodic_substrate.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 73 | .claude/worktrees/organism-arc/examples/active_inference_minimal.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 74 | .claude/worktrees/organism-arc/examples/active_inference_semantic_candidate.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 75 | .claude/worktrees/organism-arc/examples/demo.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 76 | .claude/worktrees/organism-arc/examples/mcp_agent.silk | **abandoned** | generic host / generic_host_unavailable | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 77 | .claude/worktrees/organism-arc/examples/swr_signal_lane_minimal.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 78 | .claude/worktrees/organism-arc/experiments/001_cross_agent_protected_change/governance.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 79 | .claude/worktrees/organism-arc/experiments/001_cross_agent_protected_change/observer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 80 | .claude/worktrees/organism-arc/experiments/001_cross_agent_protected_change/proposer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 81 | .claude/worktrees/organism-arc/experiments/001_cross_agent_protected_change/target.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 82 | .claude/worktrees/organism-arc/experiments/002_constrained_policy_adaptation/governance.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 83 | .claude/worktrees/organism-arc/experiments/002_constrained_policy_adaptation/observer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 84 | .claude/worktrees/organism-arc/experiments/002_constrained_policy_adaptation/proposer.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 85 | .claude/worktrees/organism-arc/experiments/002_constrained_policy_adaptation/target.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 86 | .claude/worktrees/organism-arc/test_modules/math.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 87 | .claude/worktrees/organism-arc/test_modules/utils.silk | **abandoned** | stub-sufficient | Generated worktree snapshot; duplicate of canonical source and not an active compatibility target. |
| 88 | agents/active_inference_adaptive_routing_organism.silk | **preferred** | stub-sufficient | Reference active-inference routing program; preferred as a canonical host-integration and evaluation fixture. |
| 89 | agents/agw_1/b2/set_resolver.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 90 | agents/agw_1/b2/set_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 91 | agents/agw_1/br1/belief_revision_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 92 | agents/agw_1/common/ablation_response.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 93 | agents/agw_1/common/direct_control.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 94 | agents/agw_1/common/environment_observer.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 95 | agents/agw_1/common/identity_set_resolver.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 96 | agents/agw_1/common/intervention_producer.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 97 | agents/agw_1/common/stimulus_interpreter.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 98 | agents/agw_1/common/workspace_probe.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 99 | agents/agw_1/mk1/candidate_stub.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 100 | agents/agw_1/mk1/consumer_stub.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 101 | agents/agw_1/mk1/response_stub.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 102 | agents/agw_1/non_oracle_reasoning/set_dual_sign_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 103 | agents/agw_1/non_oracle_reasoning/set_reasoning_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 104 | agents/agw_1/sensitivity/set_aggregator_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 105 | agents/agw_1/sensitivity/set_budget_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 106 | agents/agw_1/sensitivity/set_recovery_response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 107 | agents/agw_1/tf1/associator.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 108 | agents/agw_1/tf1/property_resolver.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 109 | agents/agw_1/tf1/response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 110 | agents/agw_1/tf1/taxonomy_resolver.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 111 | agents/agw_1/tf2/associator.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 112 | agents/agw_1/tf2/attribute_resolver.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 113 | agents/agw_1/tf2/response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 114 | agents/agw_1/tf3/candidate_producer.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 115 | agents/agw_1/tf3/resolver.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 116 | agents/agw_1/tf3/response_actor.silk | **preferred** | organism / task_input_unbound | Frozen AGW-1 evaluation role; preferred for compatibility coverage, but outside the initial Silk gate. |
| 117 | agents/brain_manager.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 118 | agents/cnp_leader.silk | **preferred** | stub-sufficient | Standalone Contract-Net role; preferred as a canonical coordination compatibility fixture. |
| 119 | agents/cnp_worker.silk | **preferred** | stub-sufficient | Standalone Contract-Net role; preferred as a canonical coordination compatibility fixture. |
| 120 | agents/codegen.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 121 | agents/critic.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 122 | agents/designer.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 123 | agents/episodic_loop_smoke.silk | **required** | stub-sufficient | Active-inference lifecycle smoke fixture; required for the current episode, prediction, and learning compatibility slice. |
| 124 | agents/jl_1/associator.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 125 | agents/jl_1/property_resolver.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 126 | agents/jl_1/response_actor.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 127 | agents/jl_1/secondary_resolver.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 128 | agents/leader.silk | **legacy** | stub-sufficient | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 129 | agents/manager.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 130 | agents/mb_1/memory_evidence_associator.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 131 | agents/mc_1/memory_associator.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 132 | agents/mc_1/memory_disabled_associator.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 133 | agents/mc_1/memory_property_resolver.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 134 | agents/mc_1/memory_secondary_resolver.silk | **preferred** | organism / task_input_unbound | Evaluation-suite role fixture; preferred for measurable corpus coverage, but not part of the initial runtime proof. |
| 135 | agents/memory_disabled_baseline.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 136 | agents/memory_disabled_governance_actor.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 137 | agents/memory_disabled_handoff.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 138 | agents/memory_disabled_planner.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 139 | agents/memory_disabled_reviewer.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 140 | agents/memory_governance_actor.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 141 | agents/memory_handoff.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 142 | agents/memory_planner.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 143 | agents/memory_reviewer.silk | **preferred** | stub-sufficient | Governed memory-cluster role fixture; preferred for memory protocol compatibility, but not part of the initial runtime proof. |
| 144 | agents/organism/adaptive_router.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 145 | agents/organism/author_create.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 146 | agents/organism/author_patch.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 147 | agents/organism/cnp_coordinator.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 148 | agents/organism/cnp_worker_fast.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 149 | agents/organism/cnp_worker_slow.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 150 | agents/organism/keystone_worker.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 151 | agents/organism/load_flood.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 152 | agents/organism/reasoning_coordinator.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 153 | agents/organism/reasoning_keystone.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 154 | agents/organism/route_worker_cheap.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 155 | agents/organism/route_worker_fast.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 156 | agents/organism/route_worker_steady.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 157 | agents/organism/sim_memory.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 158 | agents/organism/sim_seeder.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 159 | agents/organism/sim_worker.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 160 | agents/organism/task_capability.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 161 | agents/organism/task_counter.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 162 | agents/organism/worker_seed.silk | **required** | stub-sufficient | Resident organism worker used by current host coordination, routing, or self-regulation paths; required for the near-term runtime proof. |
| 163 | agents/researcher.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 164 | agents/summarizer.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 165 | agents/tester.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 166 | agents/zmq_leader.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 167 | agents/zmq_worker.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 168 | agents/zmq_worker_fast.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 169 | agents/zmq_worker_medium.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 170 | agents/zmq_worker_slow.silk | **legacy** | organism / dispatch_timeout | Historical root-level agent retained for compatibility evidence; it is not a current near-term proof target. |
| 171 | examples/active_inference_episodic_substrate.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 172 | examples/active_inference_minimal.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 173 | examples/active_inference_semantic_candidate.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 174 | examples/demo.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 175 | examples/mcp_agent.silk | **preferred** | generic host / generic_host_unavailable | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 176 | examples/swr_signal_lane_minimal.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 177 | examples/workspace_consumer.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 178 | examples/workspace_producer.silk | **preferred** | stub-sufficient | Published example; preferred as a language and host compatibility fixture, but not a blocking runtime target. |
| 179 | experiments/001_cross_agent_protected_change/governance.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 180 | experiments/001_cross_agent_protected_change/observer.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 181 | experiments/001_cross_agent_protected_change/proposer.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 182 | experiments/001_cross_agent_protected_change/target.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 183 | experiments/002_constrained_policy_adaptation/governance.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 184 | experiments/002_constrained_policy_adaptation/observer.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 185 | experiments/002_constrained_policy_adaptation/proposer.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 186 | experiments/002_constrained_policy_adaptation/target.silk | **required** | stub-sufficient | Safety or governance regression fixture; required to preserve governed cross-agent behavior. |
| 187 | packets/agw_1/tf3/public_candidate_producer.silk | **legacy** | organism / task_input_unbound | Blinded AGW packet copy generated for external authoring; retained as legacy input, not canonical source. |
| 188 | packets/agw_1/tf3/public_set_consumer.silk | **legacy** | organism / task_input_unbound | Blinded AGW packet copy generated for external authoring; retained as legacy input, not canonical source. |
| 189 | packets/agw_1/tf3/public_set_resolver.silk | **legacy** | organism / task_input_unbound | Blinded AGW packet copy generated for external authoring; retained as legacy input, not canonical source. |
| 190 | packets/agw_1/tf3/public_set_response_actor.silk | **legacy** | organism / task_input_unbound | Blinded AGW packet copy generated for external authoring; retained as legacy input, not canonical source. |
| 191 | runs/organism/long_horizon_loop/agent_final.silk | **legacy** | stub-sufficient | Historical long-horizon loop input explicitly excluded from the PRD proof target; retained as legacy compatibility evidence. |
| 192 | runs/organism/long_horizon_loop/agent_v1.silk | **legacy** | stub-sufficient | Historical long-horizon loop input explicitly excluded from the PRD proof target; retained as legacy compatibility evidence. |
| 193 | runs/organism/self_recursive_loop_llm/author_llm.silk | **preferred** | stub-sufficient | Self-recursive authoring-loop input; preferred for lifecycle compatibility and future rewrite proof, but not the initial Silk gate. |
| 194 | runs/organism/self_recursive_loop_llm_escalate/author_llm.silk | **preferred** | stub-sufficient | Self-recursive authoring-loop input; preferred for lifecycle compatibility and future rewrite proof, but not the initial Silk gate. |
| 195 | runs/organism/self_recursive_loop_llm_theatre/author_llm.silk | **preferred** | stub-sufficient | Self-recursive authoring-loop input; preferred for lifecycle compatibility and future rewrite proof, but not the initial Silk gate. |
| 196 | test_modules/math.silk | **required** | stub-sufficient | Language or runtime fixture used by executable tests; required to keep evaluator semantics and host boundaries covered. |
| 197 | test_modules/self_recursive_create_escalate.silk | **required** | stub-sufficient | Language or runtime fixture used by executable tests; required to keep evaluator semantics and host boundaries covered. |
| 198 | test_modules/self_recursive_escalate.silk | **required** | stub-sufficient | Language or runtime fixture used by executable tests; required to keep evaluator semantics and host boundaries covered. |
| 199 | test_modules/self_recursive_theatre.silk | **required** | stub-sufficient | Language or runtime fixture used by executable tests; required to keep evaluator semantics and host boundaries covered. |
| 200 | test_modules/utils.silk | **required** | stub-sufficient | Language or runtime fixture used by executable tests; required to keep evaluator semantics and host boundaries covered. |

