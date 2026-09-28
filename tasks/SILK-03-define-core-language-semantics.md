# SILK-03 — Define core language semantics

**Status:** Done  
**Track:** Silk  
**Depends on:** SILK-01  
**Source:** PRD §7, Silk Phase 2

## Outcome

Silk’s core concepts have documented meanings independent of the legacy interpreter.

## Work

Specify values, bindings, procedures, control flow, inputs and outputs, errors, runtime sessions, and host calls. Include representative examples and edge cases.

## Acceptance criteria

- The semantic description is sufficient to guide an implementation and compatibility review.
- Every core concept can be explained without reference to Arachne internals.

## Completion notes

- Added [the core language semantics](../docs/SILK_LANGUAGE_SEMANTICS.md), including value and binding rules, procedures, control flow, lifecycle, session isolation, error behavior, imports, and host-call boundaries.
- Kept the PRD's later host-schema and authority decisions in SILK-05 and SILK-06, and called out the remaining syntax dispositions for SILK-04.
- Cross-checked host and language boundaries against the frozen builtin inventory and the REF-02 research inventory.
