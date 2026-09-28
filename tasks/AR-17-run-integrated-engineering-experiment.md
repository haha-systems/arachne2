# AR-17 — Run an integrated engineering experiment

**Status:** Done  
**Track:** Arachne  
**Depends on:** AR-04, AR-05, AR-07, AR-09, AR-11, AR-12, AR-13, AR-14, AR-15, AR-16  
**Source:** PRD §8, Arachne Phase 13

## Outcome

A sustained run exercises Arachne as an integrated organism and produces an intelligible developmental history.

## Work

Design an experiment requiring agents, Silk, memory, workspace, regulation, replay, governance, and development. Capture the complete run and analyze both success and failure paths.

## Acceptance criteria

- A reader can trace perception, memory, reasoning, action, learning, and change from structured evidence.
- The experiment evaluates integration and observability without claiming a broader cognitive result than the evidence supports.

## Completion

- Added a runnable integrated engineering experiment that connects agent
  proposals, workspace selection, attributed episodic memory and consolidation,
  regulation, replay, Silk candidate validation and retained execution,
  governance, and routing development.
- Captured a successful 31-event run and a controlled failed 29-event run in
  [the success record](../docs/evidence/AR-17-integrated-success.json) and
  [the failure record](../docs/evidence/AR-17-integrated-failure.json). Both
  include the full cognitive event sequence and inspection timeline.
- The successful path returns `17.0` and applies a governed routing update. The
  failed path returns `8.0`, records the wrong action and failure event, and
  leaves the routing profile unchanged. Both runs also capture a rejected
  malformed candidate before the retained procedure path.
- Documented the setup, observations, and evidence limits in
  [the experiment report](../docs/ARACHNE_INTEGRATED_EXPERIMENT.md).
- Gotchas: specialists and procedure source are deterministic fixtures; the
  approval identity is synthetic; the arithmetic task is narrow; event history
  and the Silk registry are process-local. The captures demonstrate integration
  and observability, not general reasoning, learning quality, or restart
  recovery.
