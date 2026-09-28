# Silk and Arachne repository and release strategy

**Task:** INT-03  
**Related contracts:** [Arachne 2 architecture](ARACHNE2_ARCHITECTURE.md), [Silk/Arachne boundary](SILK_ARACHNE_BOUNDARY.md), [reference freeze](REFERENCE_FREEZE.md).

## Project identities and source ownership

Silk 2 and Arachne 2 are separate products with separate source ownership and releases. The target repository identities are:

| Project   | Target repository                  | Language | Local staging root                 |
| --------- | ---------------------------------- | -------- | ---------------------------------- |
| Silk 2    | `github.com/haha-systems/silk2`    | Rust     | [`silk2/`](../silk2/README.md)     |
| Arachne 2 | `github.com/haha-systems/arachne2` | Go       | [`arachne/`](../arachne/README.md) |

The target names follow the existing `haha-systems/arachne` project namespace and distinguish the next-generation source from the Zig reference. Confirm name availability and owner settings when the repositories are created; this planning workspace has no configured remote and does not publish them. The roots are already independent language projects: Cargo resolves only within `silk2/`, and the Go module resolves only within `arachne/`.

This `arachne2` workspace is the coordination and migration staging repository. It contains the PRD, specifications, task board, and the two project roots until they are extracted. No runtime package may import source from the other project or from the coordination root. After extraction, cross-project work uses released artifacts and the public protocol.

## Version boundaries

- Silk crate and executable releases use independent SemVer. A change to the Rust API or CLI does not by itself change the runtime protocol version.
- SRP protocol versioning is independent: major versions can change incompatible method or security meaning; compatible optional fields/features use minor evolution as specified in SILK-05.
- Arachne releases use their own SemVer and cadence. An Arachne release records the exact Silk runtime release, artifact digest, negotiated SRP version, and enabled protocol features used by its integration tests.
- Arachne consumes Silk as a subprocess distribution. It does not link Rust source or a shared library into Go. Any future embedded library proposal requires evidence that the protocol transport blocks a concrete use case.
- SRP schemas and normative method behavior are owned by Silk. Arachne may maintain its own Go client and tests; generated or copied client code is versioned in Arachne and does not create a shared source module.

An Arachne release must pin a tested Silk release, not a moving branch or unversioned `latest`. Arachne may support a declared range of compatible SRP minors, but production deployments should record the exact runtime artifact and negotiated version for reproducibility. Upgrading Silk is an explicit dependency update with black-box interoperability tests and authority-denial checks.

## Preserving the Zig reference

The existing Zig project remains a separate historical reference at `haha-systems/arachne`. The verified runnable source pin is the immutable tag `arachne2-start-v1` at commit `ab8d9bdadebd48cb58cfeecaa100a951c47e442c`, documented with source hashes and reproduction commands in [REFERENCE_FREEZE.md](REFERENCE_FREEZE.md). The historical `zig-reference-v1` reference written by Stage 0 was absent from fetched refs; it is not the chosen runnable pin.

The Zig reference is not an upstream for new Silk/Arachne implementation changes. If another reference snapshot is needed, create a new immutable version and preserve its commit, toolchain, hashes, and test results. The current GitHub plan does not support formal branch protection on the private repository; preserve the exact commit/tag and verify it during release rather than making the reference public.

## Release gates and migration

1. Keep both projects independently buildable from their staging roots.
2. Extract or mirror each root into its target repository with its own CI, owner files, release tags, and source history as practical.
3. Publish a Silk runtime version that passes its standalone corpus/integration gates and declares the SRP version/features it implements.
4. Pin that runtime artifact in Arachne and run black-box session, procedure, host-grant, denial-before-dispatch, trace, and clean-close tests.
5. Publish Arachne under its own version only after its tests pass against the recorded Silk artifact.
6. Preserve the reference tag and test evidence alongside the compatibility comparison; never move a reference tag to make a comparison pass.

## Open operations

- Repository creation, access controls, signing keys, package registries, and release credentials must be configured by the repository owner when extraction is scheduled.
- The exact binary distribution mechanism for Silk (release archive, platform package, or another signed artifact) can be chosen with its first release pipeline. All supported options preserve subprocess isolation and exact artifact pinning.
- If target repository names are unavailable, resolve an owner-approved alias before creating either repository and update module/repository metadata together.
