# INT-03 — Establish independent repository and release identities

**Status:** Done  
**Track:** Integration  
**Depends on:** SILK-01, AR-01  
**Source:** PRD §11

## Outcome

Silk and Arachne have clear source ownership, repository placement, and release/version boundaries.

## Work

Choose the new Silk repository and the location for new Go Arachne. Define how Arachne consumes stable Silk releases or protocol versions and how the preserved Zig implementation remains archived.

## Acceptance criteria

- The projects have independent identities and do not share source modules.
- Repository and release decisions are documented before cross-project implementation depends on them.

## Completion notes

- Selected separate repositories `haha-systems/silk2` and `haha-systems/arachne2`, with the existing independent project roots `silk2/` and `arachne/` as staging roots in this coordination workspace.
- Defined separate source ownership, SemVer release cadence, SRP protocol evolution, exact Silk artifact pinning for Arachne, and black-box upgrade gates in [the release strategy](../docs/RELEASE_STRATEGY.md).
- Preserved the runnable Zig reference by its immutable tag/commit and documented the private-repository branch-protection limitation. Repository creation and release credentials remain owner operations at extraction time.
