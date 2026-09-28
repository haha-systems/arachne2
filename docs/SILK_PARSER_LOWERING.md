# Silk parser and lowering contract

**Task:** SILK-08  
**IR schema:** `silk.ir.v1` in [`silk-ir`](../silk2/crates/silk-ir/src/lib.rs).  
**Implementation:** [`silk-syntax`](../silk2/crates/silk-syntax/src/lib.rs).

## Supported source subset

The parser currently accepts UTF-8 source with whitespace, `#` line comments, `//` line comments, single- or double-quoted strings, signed 64-bit integer literals, finite decimal floating-point literals, and these declarations/statements:

- `fn name(parameters) { ... }`, plus `policy` and `learn` declarations lowered as ordinary procedures;
- top-level executable statements, lowered into a synthetic `main` procedure;
- `let` immutable locals and `state` session-state bindings, with assignment to `state` or existing local bindings;
- `return`, `yield`, `if`/`else`, `while`, `for name in expression`, `break`, and `continue`;
- `null`, booleans, names, arrays, string-keyed object literals, unary `-`/`!`, calls, member/index access, and binary operators.

Binary precedence from lowest to highest is `or`/`||`, `and`/`&&`, equality, ordering, addition/subtraction, then multiplication/division/remainder. Operators associate left to right. Calls and member/index access bind more tightly. The lexer accepts `==`, `!=`, `<`, `>`, `<=`, `>=`, `+`, `-`, `*`, `/`, `%`, `!`, `=`, `&&`, `||`, and `->`; `and` and `or` are word operators. A trailing comma is not accepted in parameter, call, array, or object lists.

## Lowering

Each named declaration becomes `proc:<name>` with an entry block. Top-level statements become `proc:main`; declaring a named `main` alongside top-level statements is an error. Source declaration keywords `fn`, `policy`, and `learn` share the same procedure representation in this phase. The IR records blocks, instructions, explicit control-flow terminators, call target category, and byte-offset/line/column source spans. Source order and deterministic IDs are preserved by vectors and ordered maps.

Calls with dotted names are marked as host-function targets for later descriptor resolution. The initial recognized core names are `len`, `str`, `append`, `contains`, `print`, `sleep`, and `now`. Other simple names are static procedure targets and must resolve during lowering; non-name dynamic call targets are rejected. This classification does not implement or authorize any host operation.

The source-derived `program_id` uses a deterministic non-cryptographic fingerprint only as a provisional compilation key. It is not a trusted digest or stable procedure identity; SILK-13 owns identity and lineage rules.

## Diagnostics and known limits

Errors include UTF-8 byte start/end offsets and one-based line/column. Duplicate procedures, parameters, local/state declarations, and object keys are rejected. Assignment to an undeclared binding, top-level `return`/`yield`, loop control outside a loop, unsupported declarations, malformed literals, and non-name dynamic callees are errors. Legacy `act`, `tool`, `capability`, `learning`, `import`, and `export` declarations are rejected with a source location; they cannot add authority or silently become executable names.

The parser currently does not implement imports/modules, exported declarations, explicit effect annotations, nested assignment targets, static type checking, contract declarations, recursive block scopes, Unicode escape sequences, or the full historical corpus grammar. These are either awaiting a separate contract or later work. Corpus recordings remain compatibility evidence; passing parse/lowering does not imply runtime equivalence. The IR evaluator and full corpus comparison remain SILK-09 and SILK-10.
