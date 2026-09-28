//! Deterministic schema-graph composition over admitted procedure revisions.

use std::collections::{BTreeSet, VecDeque};
use std::error::Error;
use std::fmt::{Display, Formatter};

use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};

use crate::{
    LineageReference, LineageRelation, ProcedureArtifact, ValidationOutcome, validate_digest,
};

pub const COMPOSITION_PLAN_SCHEMA: &str = "silk.composition_plan.v1";
const MAX_SEARCH_STATES: usize = 4096;

/// Exact schema transition and policy constraints to satisfy with registered procedures.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct CompositionRequest {
    pub input_schema: Value,
    pub output_schema: Value,
    #[serde(default)]
    pub allowed_effects: BTreeSet<String>,
    #[serde(default)]
    pub available_authorities: BTreeSet<String>,
    #[serde(default)]
    pub required_validation_profiles: BTreeSet<String>,
    #[serde(default = "default_max_steps")]
    pub max_steps: usize,
}

fn default_max_steps() -> usize {
    16
}

/// One immutable procedure revision in the proposed execution order.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct CompositionStep {
    pub ordinal: usize,
    pub procedure_id: String,
    pub revision_digest: String,
    pub name: Option<String>,
    pub input_schema: Value,
    pub output_schema: Value,
}

/// Inspectable composition proposal. It is not itself executable or admitted.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct CompositionPlan {
    pub schema_version: String,
    pub plan_digest: String,
    pub input_schema: Value,
    pub output_schema: Value,
    pub steps: Vec<CompositionStep>,
    pub effects: BTreeSet<String>,
    pub required_authorities: BTreeSet<String>,
    pub lineage: Vec<LineageReference>,
    pub validation: Vec<String>,
}

/// Why a composition request could not produce a validated plan.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CompositionError {
    InvalidStepBound(usize),
    SearchLimitExceeded(usize),
    NoCompatiblePlan(Vec<String>),
}

impl Display for CompositionError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::InvalidStepBound(bound) => write!(
                formatter,
                "composition max_steps must be 1..=64, got {bound}"
            ),
            Self::SearchLimitExceeded(limit) => write!(
                formatter,
                "composition search exceeded {limit} states; narrow the query or reduce max_steps"
            ),
            Self::NoCompatiblePlan(reasons) => {
                formatter.write_str("no compatible composition plan")?;
                for reason in reasons {
                    write!(formatter, "; {reason}")?;
                }
                Ok(())
            }
        }
    }
}

impl Error for CompositionError {}

struct SearchNode {
    schema: Value,
    steps: Vec<CompositionStep>,
    effects: BTreeSet<String>,
    authorities: BTreeSet<String>,
    visited_schemas: Vec<Value>,
}

pub(super) fn compose_artifacts(
    artifacts: Vec<&ProcedureArtifact>,
    request: &CompositionRequest,
) -> Result<CompositionPlan, CompositionError> {
    if !(1..=64).contains(&request.max_steps) {
        return Err(CompositionError::InvalidStepBound(request.max_steps));
    }
    let mut queue = VecDeque::from([SearchNode {
        schema: request.input_schema.clone(),
        steps: Vec::new(),
        effects: BTreeSet::new(),
        authorities: BTreeSet::new(),
        visited_schemas: vec![request.input_schema.clone()],
    }]);
    let mut explored = 0;
    let mut rejection_reasons = BTreeSet::new();

    while let Some(node) = queue.pop_front() {
        explored += 1;
        if explored > MAX_SEARCH_STATES {
            return Err(CompositionError::SearchLimitExceeded(MAX_SEARCH_STATES));
        }
        if node.schema == request.output_schema && !node.steps.is_empty() {
            return Ok(make_plan(request, node));
        }
        if node.steps.len() == request.max_steps {
            continue;
        }

        for artifact in &artifacts {
            if artifact.contract.input_schema != node.schema {
                continue;
            }
            let reasons = eligibility_reasons(
                artifact,
                request,
                &node.effects,
                &node.authorities,
                &artifacts,
            );
            if !reasons.is_empty() {
                rejection_reasons.extend(reasons.into_iter().map(|reason| {
                    format!(
                        "{}@{}: {reason}",
                        artifact.procedure_id, artifact.revision_digest
                    )
                }));
                continue;
            }
            let next_schema = artifact.contract.output_schema.clone();
            if node.visited_schemas.contains(&next_schema) {
                continue;
            }
            let mut steps = node.steps.clone();
            steps.push(CompositionStep {
                ordinal: steps.len(),
                procedure_id: artifact.procedure_id.clone(),
                revision_digest: artifact.revision_digest.clone(),
                name: artifact.name.clone(),
                input_schema: artifact.contract.input_schema.clone(),
                output_schema: next_schema.clone(),
            });
            let mut effects = node.effects.clone();
            effects.extend(artifact.contract.effect_ceiling.iter().cloned());
            let mut authorities = node.authorities.clone();
            authorities.extend(artifact.contract.required_authorities.iter().cloned());
            let mut visited_schemas = node.visited_schemas.clone();
            visited_schemas.push(next_schema.clone());
            queue.push_back(SearchNode {
                schema: next_schema,
                steps,
                effects,
                authorities,
                visited_schemas,
            });
        }
    }

    if rejection_reasons.is_empty() {
        rejection_reasons.insert(format!(
            "no registered schema path from input to output within {} steps",
            request.max_steps
        ));
    }
    Err(CompositionError::NoCompatiblePlan(
        rejection_reasons.into_iter().take(32).collect(),
    ))
}

fn eligibility_reasons(
    artifact: &ProcedureArtifact,
    request: &CompositionRequest,
    accumulated_effects: &BTreeSet<String>,
    accumulated_authorities: &BTreeSet<String>,
    artifacts: &[&ProcedureArtifact],
) -> Vec<String> {
    let mut reasons = Vec::new();
    let mut effects = accumulated_effects.clone();
    effects.extend(artifact.contract.effect_ceiling.iter().cloned());
    if !effects.is_subset(&request.allowed_effects) {
        reasons.push("composition effects exceed the allowed effect set".to_owned());
    }
    let mut authorities = accumulated_authorities.clone();
    authorities.extend(artifact.contract.required_authorities.iter().cloned());
    if !authorities.is_subset(&request.available_authorities) {
        reasons.push("composition requires unavailable authority labels".to_owned());
    }
    for profile in &request.required_validation_profiles {
        if !artifact.validation_evidence.iter().any(|evidence| {
            evidence.profile == *profile
                && evidence.subject_revision_digest == artifact.revision_digest
                && evidence.outcome == ValidationOutcome::Passed
                && validate_digest(&evidence.evidence_digest).is_ok()
        }) {
            reasons.push(format!("missing passing validation profile `{profile}`"));
        }
    }
    for dependency in &artifact.contract.dependencies {
        if dependency.kind == "procedure" {
            let present = dependency.revision_digest.as_ref().is_some_and(|digest| {
                artifacts.iter().any(|candidate| {
                    candidate.procedure_id == dependency.identity
                        && candidate.revision_digest == *digest
                })
            });
            if !present {
                reasons.push(format!(
                    "unresolved procedure dependency `{}@{}`",
                    dependency.identity,
                    dependency.revision_digest.as_deref().unwrap_or("unpinned")
                ));
            }
        }
    }
    reasons
}

fn make_plan(request: &CompositionRequest, node: SearchNode) -> CompositionPlan {
    let lineage = node
        .steps
        .iter()
        .map(|step| LineageReference {
            procedure_id: step.procedure_id.clone(),
            revision_digest: step.revision_digest.clone(),
            relation: LineageRelation::ComposedFrom,
        })
        .collect::<Vec<_>>();
    let validation = vec![
        "exact input/output schema chain".to_owned(),
        "aggregate effect ceiling".to_owned(),
        "available authority labels".to_owned(),
        "revision-bound validation profiles".to_owned(),
        "pinned procedure dependencies".to_owned(),
    ];
    let digest_input = serde_json::json!({
        "schema_version": COMPOSITION_PLAN_SCHEMA,
        "input_schema": request.input_schema,
        "output_schema": request.output_schema,
        "steps": node.steps,
        "effects": node.effects,
        "required_authorities": node.authorities,
    });
    let encoded = serde_json::to_vec(&digest_input).expect("plan digest input serializes");
    let plan_digest = format!("sha256:{:x}", Sha256::digest(encoded));
    CompositionPlan {
        schema_version: COMPOSITION_PLAN_SCHEMA.to_owned(),
        plan_digest,
        input_schema: request.input_schema.clone(),
        output_schema: request.output_schema.clone(),
        steps: node.steps,
        effects: node.effects,
        required_authorities: node.authorities,
        lineage,
        validation,
    }
}
