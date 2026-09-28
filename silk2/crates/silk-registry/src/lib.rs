//! Admission and lookup for versioned semantic procedure artifacts.

use std::collections::{BTreeMap, BTreeSet};
use std::error::Error;
use std::fmt::{Display, Formatter};

use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};

mod composition;
mod retrieval;
pub use composition::{CompositionError, CompositionPlan, CompositionRequest, CompositionStep};
pub use retrieval::{
    RejectedCandidate, RetrievalCandidate, RetrievalQuery, RetrievalResponse, retrieve_artifacts,
};

pub const ARTIFACT_SCHEMA_VERSION: &str = "silk.procedure_artifact.v1";
pub const REVISION_DIGEST_DOMAIN: &str = "silk.procedure_revision.v1";
pub const RETENTION_APPROVAL_PROFILE: &str = "silk.retention_approval.v1";

/// A complete, source-independent procedure revision and its inspectable claims.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ProcedureArtifact {
    pub schema_version: String,
    pub procedure_id: String,
    pub revision_digest: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    pub semantic_description: SemanticDescription,
    pub contract: ProcedureContract,
    pub executable_semantics: Value,
    #[serde(default)]
    pub lineage: Vec<LineageReference>,
    pub provenance: Provenance,
    #[serde(default)]
    pub validation_evidence: Vec<ValidationEvidence>,
}

/// Searchable prose and normalized intent terms, excluded from executable identity.
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct SemanticDescription {
    pub summary: String,
    #[serde(default)]
    pub terms: BTreeSet<String>,
}

/// Machine-readable behavioral interface included in the revision digest.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ProcedureContract {
    pub purpose: String,
    pub input_schema: Value,
    pub output_schema: Value,
    #[serde(default)]
    pub preconditions: Vec<Value>,
    #[serde(default)]
    pub postconditions: Vec<Value>,
    #[serde(default)]
    pub effect_ceiling: BTreeSet<String>,
    #[serde(default)]
    pub required_authorities: BTreeSet<String>,
    #[serde(default)]
    pub dependencies: Vec<DependencyReference>,
}

#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
pub struct DependencyReference {
    pub kind: String,
    pub identity: String,
    pub revision_digest: Option<String>,
    pub interface_version: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct LineageReference {
    pub procedure_id: String,
    pub revision_digest: String,
    pub relation: LineageRelation,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LineageRelation {
    Revises,
    DerivedFrom,
    ComposedFrom,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Provenance {
    pub origin: Origin,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_reference: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_digest: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub generator: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub composition_plan: Option<String>,
    #[serde(default)]
    pub claims: BTreeMap<String, String>,
}

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Origin {
    HumanAuthored,
    LegacyMigration,
    Generated,
    Composed,
}

/// Validator assertion bound to one exact revision and validation profile.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct ValidationEvidence {
    pub validator: String,
    pub validator_version: String,
    pub profile: String,
    pub subject_revision_digest: String,
    pub outcome: ValidationOutcome,
    pub evidence_digest: String,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ValidationOutcome {
    Passed,
    Failed,
}

/// Admission requirements applied consistently by a registry instance.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct RegistryPolicy {
    pub required_validation_profiles: BTreeSet<String>,
}

/// Result of admitting a procedure revision.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Admission {
    Inserted,
    AlreadyPresent,
}

/// Lifecycle state for a generated procedure.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RetentionState {
    Ephemeral,
    Candidate,
    Retained,
}

/// Explicit evidence-bearing transition from one candidate revision to retained knowledge.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct RetentionRecord {
    pub sequence: usize,
    pub procedure_id: String,
    pub revision_digest: String,
    pub from: RetentionState,
    pub to: RetentionState,
    pub actor: String,
    pub reason: String,
    pub evidence: ValidationEvidence,
}

/// Approval for retaining one exact registry candidate.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct RetentionDecision {
    pub actor: String,
    pub reason: String,
    pub evidence: ValidationEvidence,
}

/// Policy or storage failure encountered during admission.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RegistryError {
    UnsupportedSchema(String),
    MissingField(&'static str),
    InvalidDigest(String),
    RevisionDigestMismatch {
        supplied: String,
        calculated: String,
    },
    MissingValidationProfile(String),
    ConflictingRevision {
        procedure_id: String,
        revision_digest: String,
    },
    NotRetainable {
        procedure_id: String,
        revision_digest: String,
    },
    InvalidRetentionEvidence(String),
}

impl Display for RegistryError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::UnsupportedSchema(schema) => {
                write!(formatter, "unsupported artifact schema: {schema}")
            }
            Self::MissingField(field) => {
                write!(formatter, "required artifact field is empty: {field}")
            }
            Self::InvalidDigest(digest) => write!(formatter, "invalid SHA-256 digest: {digest}"),
            Self::RevisionDigestMismatch {
                supplied,
                calculated,
            } => write!(
                formatter,
                "revision digest mismatch: supplied {supplied}, calculated {calculated}"
            ),
            Self::MissingValidationProfile(profile) => write!(
                formatter,
                "missing passing validation evidence for profile {profile}"
            ),
            Self::ConflictingRevision {
                procedure_id,
                revision_digest,
            } => write!(
                formatter,
                "conflicting immutable revision {procedure_id}@{revision_digest}"
            ),
            Self::NotRetainable {
                procedure_id,
                revision_digest,
            } => write!(
                formatter,
                "procedure revision is not an unretained candidate: {procedure_id}@{revision_digest}"
            ),
            Self::InvalidRetentionEvidence(message) => {
                write!(formatter, "invalid retention evidence: {message}")
            }
        }
    }
}

impl Error for RegistryError {}

/// Minimal storage boundary. Persistent implementations can replace the volatile backend.
pub trait RegistryStore {
    fn get(&self, procedure_id: &str, revision_digest: &str) -> Option<&ProcedureArtifact>;
    fn insert(&mut self, artifact: ProcedureArtifact);
    fn artifacts(&self) -> Vec<&ProcedureArtifact>;
    fn add_alias(&mut self, procedure_id: &str, revision_digest: &str, alias: String);
    fn aliases(&self, procedure_id: &str, revision_digest: &str) -> Vec<&str>;
    fn retention_state(&self, procedure_id: &str, revision_digest: &str) -> Option<RetentionState>;
    fn retention_history(&self, procedure_id: &str, revision_digest: &str)
    -> Vec<&RetentionRecord>;
    fn retain_candidate(&mut self, record: RetentionRecord) -> bool;
}

/// Volatile map-backed storage for a standalone Silk process.
#[derive(Clone, Debug, Default)]
pub struct MemoryStore {
    artifacts: BTreeMap<(String, String), ProcedureArtifact>,
    aliases: BTreeMap<(String, String), BTreeSet<String>>,
    retention_states: BTreeMap<(String, String), RetentionState>,
    retention_history: BTreeMap<(String, String), Vec<RetentionRecord>>,
}

impl RegistryStore for MemoryStore {
    fn get(&self, procedure_id: &str, revision_digest: &str) -> Option<&ProcedureArtifact> {
        self.artifacts
            .get(&(procedure_id.to_owned(), revision_digest.to_owned()))
    }

    fn insert(&mut self, artifact: ProcedureArtifact) {
        let key = (
            artifact.procedure_id.clone(),
            artifact.revision_digest.clone(),
        );
        self.artifacts.insert(key.clone(), artifact);
        self.retention_states.insert(key, RetentionState::Candidate);
    }

    fn artifacts(&self) -> Vec<&ProcedureArtifact> {
        self.artifacts.values().collect()
    }

    fn add_alias(&mut self, procedure_id: &str, revision_digest: &str, alias: String) {
        self.aliases
            .entry((procedure_id.to_owned(), revision_digest.to_owned()))
            .or_default()
            .insert(alias);
    }

    fn aliases(&self, procedure_id: &str, revision_digest: &str) -> Vec<&str> {
        self.aliases
            .get(&(procedure_id.to_owned(), revision_digest.to_owned()))
            .into_iter()
            .flat_map(BTreeSet::iter)
            .map(String::as_str)
            .collect()
    }

    fn retention_state(&self, procedure_id: &str, revision_digest: &str) -> Option<RetentionState> {
        self.retention_states
            .get(&(procedure_id.to_owned(), revision_digest.to_owned()))
            .copied()
    }

    fn retention_history(
        &self,
        procedure_id: &str,
        revision_digest: &str,
    ) -> Vec<&RetentionRecord> {
        self.retention_history
            .get(&(procedure_id.to_owned(), revision_digest.to_owned()))
            .into_iter()
            .flatten()
            .collect()
    }

    fn retain_candidate(&mut self, record: RetentionRecord) -> bool {
        let key = (record.procedure_id.clone(), record.revision_digest.clone());
        if self.retention_state(&key.0, &key.1) != Some(RetentionState::Candidate)
            || record.from != RetentionState::Candidate
            || record.to != RetentionState::Retained
        {
            return false;
        }
        self.retention_states
            .insert(key.clone(), RetentionState::Retained);
        self.retention_history.entry(key).or_default().push(record);
        true
    }
}

/// Registry policy and query interface over an injected storage backend.
#[derive(Clone, Debug)]
pub struct ProcedureRegistry<S = MemoryStore> {
    store: S,
    policy: RegistryPolicy,
}

impl Default for ProcedureRegistry<MemoryStore> {
    fn default() -> Self {
        Self::new(MemoryStore::default(), RegistryPolicy::default())
    }
}

impl<S: RegistryStore> ProcedureRegistry<S> {
    #[must_use]
    pub const fn new(store: S, policy: RegistryPolicy) -> Self {
        Self { store, policy }
    }

    /// Validates metadata, content digest, and profile evidence before insertion.
    pub fn admit(&mut self, artifact: ProcedureArtifact) -> Result<Admission, RegistryError> {
        validate_artifact(&artifact, &self.policy)?;
        if let Some(existing) = self
            .store
            .get(&artifact.procedure_id, &artifact.revision_digest)
        {
            if existing == &artifact {
                return Ok(Admission::AlreadyPresent);
            }
            return Err(RegistryError::ConflictingRevision {
                procedure_id: artifact.procedure_id,
                revision_digest: artifact.revision_digest,
            });
        }
        self.store.insert(artifact);
        Ok(Admission::Inserted)
    }

    /// Resolves an exact immutable revision. No implicit latest-version lookup exists.
    pub fn get(&self, procedure_id: &str, revision_digest: &str) -> Option<&ProcedureArtifact> {
        self.store.get(procedure_id, revision_digest)
    }

    /// Lists all admitted revisions in deterministic backend order.
    #[must_use]
    pub fn list(&self) -> Vec<&ProcedureArtifact> {
        self.store.artifacts()
    }

    /// Returns all revisions using a display name; names are not unique identities.
    #[must_use]
    pub fn find_by_name(&self, name: &str) -> Vec<&ProcedureArtifact> {
        self.store
            .artifacts()
            .into_iter()
            .filter(|item| {
                item.name.as_deref() == Some(name)
                    || self
                        .store
                        .aliases(&item.procedure_id, &item.revision_digest)
                        .contains(&name)
            })
            .collect()
    }

    /// Adds a display alias without changing the immutable artifact or revision digest.
    pub fn add_alias(
        &mut self,
        procedure_id: &str,
        revision_digest: &str,
        alias: String,
    ) -> Result<(), RegistryError> {
        if alias.trim().is_empty() {
            return Err(RegistryError::MissingField("alias"));
        }
        if self.store.get(procedure_id, revision_digest).is_none() {
            return Err(RegistryError::MissingField("exact procedure revision"));
        }
        self.store.add_alias(procedure_id, revision_digest, alias);
        Ok(())
    }

    /// Returns lifecycle state for one exact revision.
    #[must_use]
    pub fn retention_state(
        &self,
        procedure_id: &str,
        revision_digest: &str,
    ) -> Option<RetentionState> {
        self.store.retention_state(procedure_id, revision_digest)
    }

    /// Returns append-only retention decisions for one exact revision.
    #[must_use]
    pub fn retention_history(
        &self,
        procedure_id: &str,
        revision_digest: &str,
    ) -> Vec<&RetentionRecord> {
        self.store.retention_history(procedure_id, revision_digest)
    }

    /// Explicitly retains a candidate only with passing revision-bound approval evidence.
    pub fn retain(
        &mut self,
        procedure_id: &str,
        revision_digest: &str,
        decision: RetentionDecision,
    ) -> Result<(), RegistryError> {
        if self.store.get(procedure_id, revision_digest).is_none() {
            return Err(RegistryError::NotRetainable {
                procedure_id: procedure_id.to_owned(),
                revision_digest: revision_digest.to_owned(),
            });
        }
        if decision.actor.trim().is_empty() || decision.reason.trim().is_empty() {
            return Err(RegistryError::InvalidRetentionEvidence(
                "actor and reason are required".to_owned(),
            ));
        }
        let evidence = &decision.evidence;
        if evidence.profile != RETENTION_APPROVAL_PROFILE
            || evidence.subject_revision_digest != revision_digest
            || evidence.outcome != ValidationOutcome::Passed
            || evidence.validator.trim().is_empty()
            || evidence.validator_version.trim().is_empty()
            || validate_digest(&evidence.evidence_digest).is_err()
        {
            return Err(RegistryError::InvalidRetentionEvidence(
                "expected a passing, digest-bound silk.retention_approval.v1 record".to_owned(),
            ));
        }
        let record = RetentionRecord {
            sequence: self
                .store
                .retention_history(procedure_id, revision_digest)
                .len(),
            procedure_id: procedure_id.to_owned(),
            revision_digest: revision_digest.to_owned(),
            from: RetentionState::Candidate,
            to: RetentionState::Retained,
            actor: decision.actor,
            reason: decision.reason,
            evidence: decision.evidence,
        };
        if !self.store.retain_candidate(record) {
            return Err(RegistryError::NotRetainable {
                procedure_id: procedure_id.to_owned(),
                revision_digest: revision_digest.to_owned(),
            });
        }
        Ok(())
    }

    /// Returns ranked candidates that satisfy the query's compatibility filters.
    #[must_use]
    pub fn retrieve(&self, query: &RetrievalQuery) -> RetrievalResponse {
        retrieve_artifacts(self.store.artifacts(), query)
    }

    /// Finds and validates the shortest deterministic linear composition for a schema transition.
    pub fn compose(
        &self,
        request: &CompositionRequest,
    ) -> Result<CompositionPlan, CompositionError> {
        composition::compose_artifacts(self.store.artifacts(), request)
    }

    #[must_use]
    pub const fn store(&self) -> &S {
        &self.store
    }
}

/// Computes the domain-separated digest over executable semantics and behavioral contract.
pub fn calculate_revision_digest(artifact: &ProcedureArtifact) -> Result<String, RegistryError> {
    let projection = revision_projection(artifact);
    let canonical = canonical_json(&projection);
    let mut hash = Sha256::new();
    hash.update(REVISION_DIGEST_DOMAIN.as_bytes());
    hash.update([0]);
    hash.update(canonical);
    Ok(format!("sha256:{:x}", hash.finalize()))
}

fn validate_artifact(
    artifact: &ProcedureArtifact,
    policy: &RegistryPolicy,
) -> Result<(), RegistryError> {
    if artifact.schema_version != ARTIFACT_SCHEMA_VERSION {
        return Err(RegistryError::UnsupportedSchema(
            artifact.schema_version.clone(),
        ));
    }
    for (field, value) in [
        ("procedure_id", artifact.procedure_id.as_str()),
        (
            "semantic_description.summary",
            artifact.semantic_description.summary.as_str(),
        ),
        ("contract.purpose", artifact.contract.purpose.as_str()),
    ] {
        if value.trim().is_empty() {
            return Err(RegistryError::MissingField(field));
        }
    }
    if artifact.executable_semantics.is_null()
        || artifact.contract.input_schema.is_null()
        || artifact.contract.output_schema.is_null()
    {
        return Err(RegistryError::MissingField(
            "executable_semantics/contract schemas",
        ));
    }
    validate_digest(&artifact.revision_digest)?;
    let calculated = calculate_revision_digest(artifact)?;
    if artifact.revision_digest != calculated {
        return Err(RegistryError::RevisionDigestMismatch {
            supplied: artifact.revision_digest.clone(),
            calculated,
        });
    }
    for profile in &policy.required_validation_profiles {
        let has_pass = artifact.validation_evidence.iter().any(|evidence| {
            evidence.profile == *profile
                && evidence.subject_revision_digest == artifact.revision_digest
                && evidence.outcome == ValidationOutcome::Passed
                && !evidence.validator.trim().is_empty()
                && !evidence.validator_version.trim().is_empty()
                && validate_digest(&evidence.evidence_digest).is_ok()
        });
        if !has_pass {
            return Err(RegistryError::MissingValidationProfile(profile.clone()));
        }
    }
    for dependency in &artifact.contract.dependencies {
        if dependency.kind.trim().is_empty() || dependency.identity.trim().is_empty() {
            return Err(RegistryError::MissingField(
                "contract.dependencies.kind/identity",
            ));
        }
        if dependency.revision_digest.is_none() && dependency.interface_version.is_none() {
            return Err(RegistryError::MissingField(
                "contract.dependencies.revision_digest/interface_version",
            ));
        }
        if let Some(digest) = &dependency.revision_digest {
            validate_digest(digest)?;
        }
    }
    for parent in &artifact.lineage {
        if parent.procedure_id.trim().is_empty() {
            return Err(RegistryError::MissingField("lineage.procedure_id"));
        }
        validate_digest(&parent.revision_digest)?;
    }
    Ok(())
}

fn validate_digest(digest: &str) -> Result<(), RegistryError> {
    let hex = digest
        .strip_prefix("sha256:")
        .ok_or_else(|| RegistryError::InvalidDigest(digest.to_owned()))?;
    if hex.len() != 64
        || !hex
            .bytes()
            .all(|byte| byte.is_ascii_hexdigit() && !byte.is_ascii_uppercase())
    {
        return Err(RegistryError::InvalidDigest(digest.to_owned()));
    }
    Ok(())
}

fn revision_projection(artifact: &ProcedureArtifact) -> Value {
    serde_json::json!({
        "executable_semantics": artifact.executable_semantics,
        "contract": {
            "input_schema": artifact.contract.input_schema,
            "output_schema": artifact.contract.output_schema,
            "preconditions": artifact.contract.preconditions,
            "postconditions": artifact.contract.postconditions,
            "effect_ceiling": artifact.contract.effect_ceiling,
            "required_authorities": artifact.contract.required_authorities,
            "dependencies": artifact.contract.dependencies,
        }
    })
}

fn canonical_json(value: &Value) -> Vec<u8> {
    let mut bytes = Vec::new();
    write_canonical(value, &mut bytes);
    bytes
}

fn write_canonical(value: &Value, output: &mut Vec<u8>) {
    match value {
        Value::Null => output.extend_from_slice(b"null"),
        Value::Bool(value) => output.extend_from_slice(if *value { b"true" } else { b"false" }),
        Value::Number(value) => output.extend_from_slice(value.to_string().as_bytes()),
        Value::String(value) => output.extend_from_slice(
            serde_json::to_string(value)
                .expect("string serializes")
                .as_bytes(),
        ),
        Value::Array(values) => {
            output.push(b'[');
            for (index, value) in values.iter().enumerate() {
                if index > 0 {
                    output.push(b',');
                }
                write_canonical(value, output);
            }
            output.push(b']');
        }
        Value::Object(values) => {
            let sorted = values.iter().collect::<BTreeMap<_, _>>();
            output.push(b'{');
            for (index, (key, value)) in sorted.iter().enumerate() {
                if index > 0 {
                    output.push(b',');
                }
                output.extend_from_slice(
                    serde_json::to_string(key)
                        .expect("key serializes")
                        .as_bytes(),
                );
                output.push(b':');
                write_canonical(value, output);
            }
            output.push(b'}');
        }
    }
}
