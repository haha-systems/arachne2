# SILK-09 — Implement the IR evaluator

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-07  
**Source:** PRD §7, Silk Phase 5

## Outcome

Silk can execute semantic IR with a tree- or graph-walking evaluator.

## Work

Implement procedure evaluation, control flow, session state, host calls, and structured runtime errors. Keep execution behavior explicit and testable before considering bytecode.

## Acceptance criteria

- The evaluator executes trivial independent programs.
- Host calls pass through the provider contract and authority checks.
- Evaluation produces enough context to diagnose failures.

## Completion notes

- Added a graph-walking evaluator for the implemented IR subset, including calls, values, short-circuit booleans, structured output, session state, control flow, and resource limits.
- Added a reusable `Session` API with host provider, descriptor, and grant inputs. Each host invocation is gated immediately before dispatch.
- Added structured errors with procedure, block, instruction, and source context; basic IR schema/control-flow validation; integer overflow detection.
- Wired `silk run <path>` to compile, evaluate `main`, and return one stable JSON object containing output and result.
- Checks: `cargo fmt --all --check`, `cargo test --workspace`, and `cargo clippy --workspace --all-targets -- -D warnings`.
- Gotchas/CES: integer literals initially passed through `f64`, losing exact integer/overflow behavior, so the IR/parser now preserve signed 64-bit integers separately. Error helpers initially lacked procedure context; the evaluator enriches operation errors before unwinding. Short-circuit operators lower to CFG branches and phi values so the skipped side cannot invoke a host function.
- Remaining runtime boundaries are captured in [the evaluator notes](../docs/SILK_EVALUATOR.md), including replay-aware `now`/`sleep`, host schemas, traces, and full corpus compatibility.
