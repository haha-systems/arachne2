# AR-18 — Add a durable cognitive event store

**Status:** Review  
**Track:** Arachne  
**Depends on:** AR-05, AR-14  
**Source:** Cognitive events contract, development recovery contract

## Outcome

An append-only event log survives process restart and can rebuild the
development routing profile from the original event history.

## Work

Add a standard-library file-backed `EventStore` that preserves the existing
event schema and sequence contract. Persist versioned, organism-bound JSONL
records with checksums, bounded capacity, fsync-before-success, typed recovery
errors, and reported final-line repair. Verify event round trips, sequence
validation, corruption handling, permissions, failed-write rollback, and
routing-profile replay after reopening the store. Document durability limits
and place the card in Review.

## Acceptance criteria

- The `EventStore` interface, event schema, `Spine` API, and default in-memory
  daemon behavior remain unchanged.
- The file store preserves event order and sequence numbers across reopen,
  rejects duplicate and non-monotonic sequences, enforces capacity without
  eviction, and uses 0600 permissions.
- Recovery repairs and reports only an incomplete final line. Mid-file
  corruption is a typed error and leaves the file unchanged.
- Each successful append is synced before return; failed writes do not publish
  an event or advance the accepted sequence.
- Tests rebuild only the development routing profile by replaying the reopened
  history through `Spine.Read` and `NewEngine`.
- Documentation records single-writer, CRC32, capacity, and fresh-spine
  sequence-continuation limits.
