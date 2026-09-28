# Silk procedure composition

**Task:** SILK-16  
**API:** `silk_registry::ProcedureRegistry::compose`  
**Plan schema:** `silk.composition_plan.v1`

The composer searches admitted procedure revisions as a directed schema graph.
An edge is eligible when its input schema exactly equals the current schema;
its output schema becomes the next node. Breadth-first search returns the
shortest path, with stable registry ordering breaking ties. The request bounds
path length to 1–64 steps and the search to 4,096 explored states.

Every proposed step pins both procedure ID and revision digest. The returned
plan also carries the schema chain, aggregate effects and authority labels,
lineage references, a deterministic plan digest, and the validations that were
performed. It is an inspectable proposal, not an executable procedure and not
an admitted registry artifact.

The composer rejects candidates whose cumulative effect set exceeds the
request's `allowed_effects`, whose declared authority requirements are absent
from `available_authorities`, or whose revision lacks a requested passing
validation profile. It also requires each declared `procedure` dependency to
resolve to the exact ID/digest in the registry. `NoCompatiblePlan` contains
bounded, candidate-specific reasons when policy or dependency checks remove
otherwise connected edges.

This initial composer supports linear chains and exact JSON schema equality.
It does not reason about pre/postcondition expressions, schema subtyping,
branches, parallel execution, or value transformations. Authority labels are
compatibility constraints only; the Silk runtime must still check the active
session grant before dispatch. Validation evidence remains an assertion until a
separate trust policy verifies the validator.
