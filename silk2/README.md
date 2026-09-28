# Silk 2 Rust workspace

This directory is an independently buildable Silk project. It has no path
dependency on Arachne or the Zig reference. The `reference/silk/` directory
contains versioned Stage 0 recordings and the source snapshots that remain
available with verified hashes.

## Requirements

- Rust stable with the 2024 edition (1.85 or newer).
- No Zig installation, Arachne source checkout, network service, or credentials
  are needed for the workspace checks.

## Build and run

From this directory:

```sh
cargo fmt --all --check
cargo clippy --workspace --all-targets -- -D warnings
cargo build --workspace
cargo test --workspace -- --nocapture
cargo run -p silk-cli -- run examples/hello.silk
cargo run -p silk-cli -- inspect examples/hello.silk
cargo run -p silk-cli -- serve
cargo run -p silk-cli --example standalone_parts
```

The CLI can execute the supported source subset, inspect static semantic IR,
inspect structured execution snapshots, capture deterministic runs, and compare
versioned snapshots. See [semantic inspection](../docs/SILK_INSPECTION.md) and
[compatibility status](../docs/SILK_COMPATIBILITY_STATUS.md) for current scope
and limitations.

## Boundaries

- `silk-syntax` owns parsing and lowering for the documented Silk subset.
- `silk-runtime` owns effect analysis, session evaluation, host authorization,
  traces, and deterministic replay.
- `silk-protocol` owns versioned JSON-RPC data structures only.
- `silk-cli` owns the public `silk run`, `silk inspect`, `silk capture`, and
  `silk compare` commands.
- `silk-corpus` owns the archived reference envelope and execution snapshot
  comparison types used by the CLI.
- `silk-registry` owns source-independent artifact admission and exact revision
  lookup. Its default backend is volatile memory; see
  [registry contract](../docs/SILK_PROCEDURE_REGISTRY.md).
- `silk-synthesis` prepares externally generated source through parsing,
  static analysis, registry admission, and the normal runtime path; see
  [synthesis pipeline](../docs/SILK_SYNTHESIS.md).

The standalone inventory workflow demonstrates retrieval, composition, host
authorization, execution, and trace explanation without Arachne. See
[standalone demo](../docs/SILK_STANDALONE_DEMO.md).

See [`PHASES.md`](PHASES.md) for the scope of the first milestone and the
decisions reserved for later language and host specifications.
