# Zig reference freeze for Arachne 2 / Silk 2

**Task:** REF-01  
**Reference repository:** `https://github.com/haha-systems/arachne`  
**Pinned source:** tag `arachne2-start-v1` and branch `reference/arachne2-start-2026-09`, both at commit `ab8d9bdadebd48cb58cfeecaa100a951c47e442c`  
**Toolchain:** Zig `0.16.0`, selected by the repository's `mise.toml`

## Why this pin exists

The sibling checkout's `docs/silk2/STAGE0_FREEZE.md` records an earlier tag,
`zig-reference-v1` at `8a16b6a76008ed423d60f5e4e86469b89c659b04`. That tag and
commit were not present in the sibling checkout's tag refs or in a fresh Jujutsu
clone's fetched refs. The remote `main` and `silk2` branches did not contain that
commit. The local Stage 0 documents therefore described a reference that was not
currently retrievable by its recorded identity.

The six source files that the Stage 0 document says must remain frozen have
identical SHA-256 contents in the sibling checkout and the clean remote `main`
revision pinned above:

| Frozen source file | SHA-256 |
|---|---|
| `src/Lexer.zig` | `fe86916f59e6652b9cd6418b8c48eda7cd2516af506956a954fc28e2f12e8b81` |
| `src/Ast.zig` | `95655b8a2bed1674aa06fb2a1f1f926806518af26a1057798b3b4c656807699f` |
| `src/Interpreter.zig` | `5cadcf585857d95c1423927226bbb312cb68a598465cadc0efaadc9d427409b6` |
| `src/ModuleLoader.zig` | `106d16b0c861182bdba93e932e2386bbee5240356cccf1782110ce07ba83a5e5` |
| `src/AstSerializer.zig` | `cbcb9037d06640fd119a7b1a8dda4a91488cb87c560f7fa4b66a299c65c57c2e` |
| `src/Value.zig` | `471b8e69dc0b8454d924627e12970ea399889487045fb80d41cb0152f46c78e1` |

The branch points at the clean remote `main` commit that passed the checks
below. It is a new practical reference pin; it does not recreate the missing
historical tag or claim that the Stage 0 corpus documents are present on that
commit. Those documents and recordings remain separately identified by the
[reference inventory](REFERENCE_INVENTORY.md).

## Reproduction

Start from a clean checkout of the pinned branch. `mise.toml` selects the Zig
version; `mise` must be installed. No LLM, MCP service, credentials, or external
agent host is needed for the build, tests, or the simple demo run.

```sh
mise exec -- zig build
mise exec -- zig build test
mise exec -- zig build run -- examples/demo.silk
```

The documented clean clone was checked out through Jujutsu at commit
`ab8d9bdadebd48cb58cfeecaa100a951c47e442c`. Results:

| Command | Result |
|---|---|
| `mise exec -- zig build` | Exit 0 |
| `mise exec -- zig build test` | Exit 0 |
| `mise exec -- zig build run -- examples/demo.silk` | Exit 0; `Program executed successfully` |

The test command emits parser errors and `failed command` diagnostics while
exercising expected negative fixtures; its final process status is zero. Judge
the suite by the exit status, not by those expected diagnostic lines alone.

The upstream [README](../../arachne/README.md) also documents Zig and
`zig build test`; this freeze file pins the toolchain more precisely and adds
the clean demo invocation.

## Keeping the reference stable

Do not move the `arachne2-start-v1` tag or amend/force-update
`reference/arachne2-start-2026-09`. All compatibility reports should name the
tag and full commit ID. An attempt to read branch protection through GitHub's
API returned HTTP 403: the private repository's plan does not support this
feature. Formal branch protection is unavailable; treat the tag and full
content-addressed commit as the stable boundary, and do not move the tag.

The original `zig-reference-v1` tag discrepancy should be reconciled in the
legacy repository's Stage 0 documentation when that repository can be updated
under its own workflow. This Arachne 2 checkout records the verified pin without
changing the reference checkout's files.
