# SILK-06 — Model effects, grants, and authority checks

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-05  
**Source:** PRD §7, Silk Phase 3

## Outcome

Silk can determine which external effects a program may perform and whether a session authorizes them.

## Work

Define grants and effect categories, implement conservative dynamic checks, and add static effect inference where available. Specify denial and provider-failure behavior.

## Acceptance criteria

- Unauthorized calls are rejected before the host performs them.
- Static findings and runtime decisions are inspectable and have documented semantics.

## Progress

- Added [the effects and authority model](../docs/SILK_EFFECTS_AUTHORITY.md), including the effect vocabulary, static inference, session grants, pre-dispatch check order, denial traces, and conformance cases.
- Added protocol `Effect`, `HostFunctionDescriptor`, and `Grant` types plus a runtime `dispatch_host_call` gate. It requires an exact function descriptor, exact authority grant, and a grant effect ceiling covering all declared effects. Unknown effects fail closed. Tests prove denied and undeclared calls never invoke the dispatch closure, and an authorized call invokes it exactly once.
- Added the public `analyze_effects` graph analyzer. It computes direct/transitive sets, dependencies, required authorities, unknown call sites, recursive fixpoints, and ceiling violations. Static summaries and dynamic decision types serialize for later inspection and trace use.
- Parser/IR adaptation is intentionally not part of this task; SILK-07/08 will build the graph input and SILK-09 will call the dispatch gate while evaluating host calls. Argument-schema and live session-state checks also belong to that integration.
- Verified with Rust 1.95.0: `cargo fmt --all --check`, all workspace tests, `cargo build --workspace`, and strict Clippy pass. Tests cover grant matching, pre-dispatch denial, unknown effects, graph transitivity/recursion, unresolved calls, and ceiling violations.

## Gotchas and CES record

- **Observation:** Cargo reported a duplicate `serde.workspace` key after adding the protocol types. **Hypothesis:** the dependency was already present in the protocol manifest. **Action:** removed the duplicate key. **Outcome:** Cargo metadata loaded and compilation advanced.
- **Observation:** The protocol test could not resolve `Effect`. **Hypothesis:** the test module's restricted `super` import omitted the new enum. **Action:** imported `Effect` into the test module. **Outcome:** protocol serialization and all workspace tests passed.
- **Review finding:** when multiple grants name one authority, the first matching grant may be too narrow while a later grant covers the call. The gate checks whether any exact-authority grant covers the full effect set; it does not stop at the first match.
- **Observation:** the first static-analysis compile could not find Serde in `silk-runtime`. **Hypothesis:** serializable effect summaries introduced a direct dependency not implied transitively by `silk-protocol`. **Action:** declared the workspace Serde dependency on `silk-runtime`. **Outcome:** workspace tests and strict Clippy pass.
