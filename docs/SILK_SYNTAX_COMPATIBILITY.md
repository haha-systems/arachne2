# Legacy syntax disposition and migration

**Task:** SILK-04  
**Evidence:** [Stage 0 builtin and syntax inventory](../silk2/reference/silk/stage0/BUILTIN_INVENTORY.md), [corpus archive](../silk2/reference/silk/README.md), [language semantics](SILK_LANGUAGE_SEMANTICS.md), and [effects and authority](SILK_EFFECTS_AUTHORITY.md).

The Stage 0 inventory analyzes the 200-file corpus and records core forms, host-call names, use counts, and dispositions. The compatibility archive separately records each program's class and host-dependence status. A legacy token's presence establishes migration evidence; it does not make that spelling or behavior part of Silk 2.

## Declaration and authority forms

| Legacy form                               | Silk 2 disposition                                                                                         | Migration rule                                                                                                                                                                   |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `state name = value`                      | Retained with new semantics                                                                                | State is session-local and isolated. It is not durable memory and does not authorize a host write.                                                                               |
| `policy name(...) { ... }`                | Retained as an agent lifecycle declaration                                                                 | The runtime invokes the policy through the lifecycle API. It returns a decision; it does not dispatch work itself.                                                               |
| `learn name(...) { ... }`                 | Retained as the learning lifecycle spelling                                                                | A `learn` procedure can orchestrate explicitly declared host functions. Changes beyond session-local state require an authorized host call.                                      |
| `act name(...) { ... }`                   | Compatibility input only; migrate to an ordinary procedure with an inferred/declared effect classification | The action concept remains; a separate action execution engine or authority-bearing keyword does not. The exact effect annotation syntax is reserved for the parser/IR contract. |
| `tool name using provider.method`         | Compatibility input only; replace with a host function descriptor supplied when creating the session       | Host descriptors declare callable name and schema. Calls still require a matching session grant and pre-dispatch authority check.                                                |
| `capability ...`                          | Deprecated Arachne-specific syntax; remove from executable Silk                                            | A capability declaration cannot grant its own authority. The host supplies session grants separately from program source.                                                        |
| `learning ...`                            | Deprecated spelling; use `learn`                                                                           | No `learning` declaration occurs in the corpus; `learn` occurs seven times in six canonical files.                                                                               |
| `memory_policy` or memory-layer selectors | Host policy metadata, not executable language authority                                                    | Move access policy into the host's descriptor/session policy. Silk does not own Arachne memory or decide its authorization.                                                      |

## Host calls and core syntax

Calls to language-core functions remain ordinary Silk calls. The inventory classifies `len`, `str`, `append`, `contains`, string helpers, JSON helpers, and other core forms separately from generic host functions and Arachne-specific host functions. Only language-core behavior belongs in the evaluator.

Legacy generic calls such as MCP tools, clock, and delay are declared by the session host and invoked through the host protocol. Arachne-specific calls for active inference, memory, governance, workspace, contract-net, or SWR also stay host-owned. Neither call category becomes a builtin merely because it appears frequently in legacy programs.

For example, migrate authority-bearing source declarations out of Silk source:

```silk
# Legacy shape; illustrates syntax only.
tool lookup using catalog.search
capability catalog.read
act find_item(query) {
  return lookup(query)
}
```

into an ordinary Silk procedure whose `catalog.search` function is declared by the host and granted for the session:

```silk
fn find_item(query) {
  return catalog.search({"query": query})
}
```

The function descriptor and `catalog.read` grant are passed through the session creation contract, not embedded in this source. The exact wire encoding is specified in [the host protocol](SILK_HOST_PROTOCOL.md); effect inference and authority checks are specified in [the effects contract](SILK_EFFECTS_AUTHORITY.md).

## Syntax inventory coverage and unresolved parser details

The archived inventory covers all 200 corpus program files for observed builtin and declaration usage and calls out canonical lifecycle examples separately from duplicate worktree copies. The core grammar and runtime meanings are in [SILK_LANGUAGE_SEMANTICS.md](SILK_LANGUAGE_SEMANTICS.md). It covers the source forms needed for the corpus: declarations, bindings, literals, calls, member/index access, assignment, conditions, loops, imports, and control transfer.

The initial tokenization, precedence, comment forms, declaration handling, and source-span diagnostics are defined in [the SILK-08 parser contract](SILK_PARSER_LOWERING.md). Import/export syntax, module resolution, and effect annotations still need implementation contracts. Legacy dynamic import fallback and global symbol extraction are not defaults; safe normalized module roots and explicit exports are the target semantics.

## Decisions and gotchas

- Corpus-wide counts include duplicate `.claude/worktrees` snapshots; lifecycle counts deduplicate those copies.
- The archive contains 33 `required`, 60 `preferred`, 20 `legacy`, and 87 `abandoned` records. Host dependence is a separate axis.
- Seven canonical `learn` declarations use a repeated four-call active-inference sequence. Their mutation authority comes from host calls and grants, not from the `learn` keyword.
- Do not treat a removed declaration or builtin as an implicit host permission. Host declarations and grants remain separate inputs.
