//! Explainable lexical ranking and compatibility filtering for procedure artifacts.

use std::collections::BTreeSet;

use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::{DependencyReference, ProcedureArtifact, ValidationOutcome};

/// Query terms rank matches; optional interface and policy fields filter them.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
pub struct RetrievalQuery {
    #[serde(default)]
    pub intent: String,
    #[serde(default)]
    pub preferred_terms: BTreeSet<String>,
    #[serde(default)]
    pub required_terms: BTreeSet<String>,
    #[serde(default)]
    pub input_schema: Option<Value>,
    #[serde(default)]
    pub output_schema: Option<Value>,
    #[serde(default)]
    pub allowed_effects: Option<BTreeSet<String>>,
    #[serde(default)]
    pub available_authorities: Option<BTreeSet<String>>,
    #[serde(default)]
    pub required_validation_profiles: BTreeSet<String>,
    #[serde(default)]
    pub required_dependencies: BTreeSet<DependencyReference>,
    #[serde(default)]
    pub max_results: Option<usize>,
    #[serde(default)]
    pub include_rejected: bool,
}

/// Candidate with the full inspectable contract and a deterministic score explanation.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct RetrievalCandidate {
    pub artifact: ProcedureArtifact,
    pub score: u32,
    pub matched_preferred_terms: Vec<String>,
    pub matched_intent_terms: Vec<String>,
    pub explanation: Vec<String>,
}

/// A filtered artifact and the constraints it did not satisfy.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct RejectedCandidate {
    pub procedure_id: String,
    pub revision_digest: String,
    pub name: Option<String>,
    pub reasons: Vec<String>,
}

/// Ranked matches plus optional explainable filter rejections.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
pub struct RetrievalResponse {
    pub candidates: Vec<RetrievalCandidate>,
    pub rejected: Vec<RejectedCandidate>,
}

/// Filters candidates, scores textual relevance, and applies stable tie-breaking.
pub fn retrieve_artifacts<'a>(
    artifacts: impl IntoIterator<Item = &'a ProcedureArtifact>,
    query: &RetrievalQuery,
) -> RetrievalResponse {
    let intent_terms = tokens(&query.intent);
    let preferred_terms = query
        .preferred_terms
        .iter()
        .flat_map(|term| tokens(term))
        .collect::<BTreeSet<_>>();
    let mut response = RetrievalResponse::default();

    for artifact in artifacts {
        let reasons = filter_reasons(artifact, query);
        if !reasons.is_empty() {
            if query.include_rejected {
                response.rejected.push(RejectedCandidate {
                    procedure_id: artifact.procedure_id.clone(),
                    revision_digest: artifact.revision_digest.clone(),
                    name: artifact.name.clone(),
                    reasons,
                });
            }
            continue;
        }

        let description_terms = artifact
            .semantic_description
            .terms
            .iter()
            .flat_map(|term| tokens(term))
            .collect::<BTreeSet<_>>();
        let summary_terms = tokens(&artifact.semantic_description.summary);
        let purpose_terms = tokens(&artifact.contract.purpose);
        let matched_preferred_terms = preferred_terms
            .intersection(&description_terms)
            .cloned()
            .collect::<Vec<_>>();
        let searchable_terms = description_terms.union(&summary_terms).cloned().collect();
        let matched_intent_terms = intent_terms
            .intersection(&searchable_terms)
            .cloned()
            .collect::<Vec<_>>();
        let purpose_matches = intent_terms.intersection(&purpose_terms).count() as u32;
        let score = (matched_preferred_terms.len() as u32 * 10)
            + (matched_intent_terms.len() as u32 * 3)
            + purpose_matches;
        let mut explanation = Vec::new();
        if !matched_preferred_terms.is_empty() {
            explanation.push(format!(
                "matched preferred terms: {}",
                matched_preferred_terms.join(", ")
            ));
        }
        if !matched_intent_terms.is_empty() {
            explanation.push(format!(
                "matched intent terms: {}",
                matched_intent_terms.join(", ")
            ));
        }
        if purpose_matches > 0 {
            explanation.push(format!("matched {} purpose terms", purpose_matches));
        }
        if explanation.is_empty() {
            explanation.push("eligible by filters; no lexical match".to_owned());
        }
        response.candidates.push(RetrievalCandidate {
            artifact: artifact.clone(),
            score,
            matched_preferred_terms,
            matched_intent_terms,
            explanation,
        });
    }

    response.candidates.sort_by(|left, right| {
        right
            .score
            .cmp(&left.score)
            .then_with(|| left.artifact.procedure_id.cmp(&right.artifact.procedure_id))
            .then_with(|| {
                left.artifact
                    .revision_digest
                    .cmp(&right.artifact.revision_digest)
            })
    });
    if let Some(limit) = query.max_results {
        response.candidates.truncate(limit);
    }
    response.rejected.sort_by(|left, right| {
        left.procedure_id
            .cmp(&right.procedure_id)
            .then_with(|| left.revision_digest.cmp(&right.revision_digest))
    });
    response
}

fn filter_reasons(artifact: &ProcedureArtifact, query: &RetrievalQuery) -> Vec<String> {
    let mut reasons = Vec::new();
    let terms = artifact
        .semantic_description
        .terms
        .iter()
        .flat_map(|term| tokens(term))
        .chain(tokens(&artifact.semantic_description.summary))
        .collect::<BTreeSet<_>>();
    for required in query.required_terms.iter().flat_map(|term| tokens(term)) {
        if !terms.contains(&required) {
            reasons.push(format!("missing required term `{required}`"));
        }
    }
    if query
        .input_schema
        .as_ref()
        .is_some_and(|schema| schema != &artifact.contract.input_schema)
    {
        reasons.push("input schema is not an exact match".to_owned());
    }
    if query
        .output_schema
        .as_ref()
        .is_some_and(|schema| schema != &artifact.contract.output_schema)
    {
        reasons.push("output schema is not an exact match".to_owned());
    }
    if let Some(allowed) = &query.allowed_effects {
        if !artifact.contract.effect_ceiling.is_subset(allowed) {
            reasons.push("effect ceiling exceeds allowed effects".to_owned());
        }
    }
    if let Some(available) = &query.available_authorities {
        if !artifact.contract.required_authorities.is_subset(available) {
            reasons.push("required authorities are unavailable to the caller".to_owned());
        }
    }
    for profile in &query.required_validation_profiles {
        let passed = artifact.validation_evidence.iter().any(|evidence| {
            evidence.profile == *profile
                && evidence.subject_revision_digest == artifact.revision_digest
                && evidence.outcome == ValidationOutcome::Passed
        });
        if !passed {
            reasons.push(format!("missing passing validation profile `{profile}`"));
        }
    }
    for dependency in &query.required_dependencies {
        if !artifact.contract.dependencies.contains(dependency) {
            reasons.push(format!(
                "missing dependency `{}:{}`",
                dependency.kind, dependency.identity
            ));
        }
    }
    reasons
}

fn tokens(value: &str) -> BTreeSet<String> {
    let normalized = value.to_lowercase();
    let mut result = BTreeSet::new();
    let mut token = String::new();
    for character in normalized.chars() {
        if character.is_alphanumeric() {
            token.push(character);
        } else if !token.is_empty() {
            result.insert(std::mem::take(&mut token));
        }
    }
    if !token.is_empty() {
        result.insert(token);
    }
    result
}
