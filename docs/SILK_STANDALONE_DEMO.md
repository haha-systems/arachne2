# Standalone Silk demonstration

**Task:** SILK-19  
**Example:** `silk-cli/examples/standalone_parts.rs`

From `silk2/`, run:

```sh
cargo run -p silk-cli --example standalone_parts
```

The example prepares and admits two generated candidates: check local part
availability, then reserve the part if available. It retrieves each candidate
by intent terms and exact input/output schemas, asks the registry to compose a
two-step plan, and executes the pinned revisions through a Silk session. The
application prints each selected revision, matched terms, effect and authority
declarations, structured trace event categories, and final result.

The `inventory.available` and `inventory.reserve` host functions are real
capabilities implemented by a local in-memory application provider. The
example supplies exact host descriptors and session grants; the runtime emits
authorization events before dispatch. It has no network dependency and imports
no Arachne code.

This is a demonstration of the current subset, not a persistent service. The
registry is memory-backed, candidate procedures and schemas are hard-coded,
schema composition requires exact equality, and the inventory resets on each
run. The output explains the chosen procedure revisions and what the session
did, not a proof of semantic safety.
