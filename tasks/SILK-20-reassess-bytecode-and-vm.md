# SILK-20 — Reassess the bytecode and VM decision

**Status:** Deferred  
**Track:** Silk  
**Depends on:** SILK-09, SILK-10  
**Source:** PRD §7, Deferred Work

## Outcome

Any move from semantic IR evaluation to bytecode is justified by concrete runtime needs.

## Work

Revisit bytecode only when measured needs include performance, portable artifacts, hard resource accounting, isolation, deterministic low-level replay, multiple engines, or another runtime target. Compare options while keeping semantic IR canonical.

## Acceptance criteria

- The decision records evidence, tradeoffs, and a go/no-go outcome.
- No bytecode implementation is started without a demonstrated requirement.

