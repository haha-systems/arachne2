//! Candidate preparation, validation, registry admission, and execution pipeline.

use std::collections::{BTreeMap, BTreeSet};
use std::error::Error;
use std::fmt::{Display, Formatter};

use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use silk_ir::{CallTarget, Operation, ProgramIR};
use silk_protocol::{Effect, Grant, HostFunctionDescriptor};
use silk_registry::{
    ARTIFACT_SCHEMA_VERSION, Admission, Origin, ProcedureArtifact, ProcedureContract,
    ProcedureRegistry, Provenance, RegistryError, RegistryStore, RetentionState,
    SemanticDescription, ValidationEvidence, ValidationOutcome, calculate_revision_digest,
};
use silk_runtime::{
    EffectAnalysis, EffectCall, EffectCallTarget, HostFunctionProvider, ProcedureEffects, Session,
    analyze_effects,
};
use silk_syntax::{ParseError, lower};

/// Caller-supplied identity and declared interface for one synthesized candidate.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct CandidateRequest {
    pub procedure_id: String,
    pub name: Option<String>,
    pub semantic_description: SemanticDescription,
    pub contract: ProcedureContract,
    pub provenance: Provenance,
    pub entry_procedure: String,
    pub required_validation_profiles: BTreeSet<String>,
}

/// Checks required before a candidate can enter the ordinary runtime path.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PipelinePolicy {
    pub allowed_origins: BTreeSet<Origin>,
}

impl Default for PipelinePolicy {
    fn default() -> Self {
        Self {
            allowed_origins: [Origin::Generated, Origin::Composed].into_iter().collect(),
        }
    }
}

/// A source-located or policy failure during candidate preparation or execution.
#[derive(Debug)]
pub enum PipelineError {
    Parse(ParseError),
    MissingEntry(String),
    InvalidOrigin(Origin),
    EmptyProcedureId,
    UnsupportedValidationProfile(String),
    Execution(String),
    EffectCeilingMismatch {
        inferred: BTreeSet<String>,
        declared: BTreeSet<String>,
    },
    AuthorityRequirementMismatch {
        inferred: BTreeSet<String>,
        declared: BTreeSet<String>,
    },
    Registry(RegistryError),
}

impl Display for PipelineError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Parse(error) => write!(formatter, "candidate parse/lowering failed: {error}"),
            Self::MissingEntry(entry) => write!(
                formatter,
                "candidate entry procedure does not exist: {entry}"
            ),
            Self::InvalidOrigin(origin) => {
                write!(formatter, "candidate origin is not allowed: {origin:?}")
            }
            Self::EmptyProcedureId => {
                formatter.write_str("candidate procedure_id must not be empty")
            }
            Self::UnsupportedValidationProfile(profile) => write!(
                formatter,
                "pipeline does not produce validation profile `{profile}`"
            ),
            Self::Execution(message) => write!(formatter, "candidate execution failed: {message}"),
            Self::EffectCeilingMismatch { inferred, declared } => write!(
                formatter,
                "inferred effects {inferred:?} exceed or are absent from declared ceiling {declared:?}"
            ),
            Self::AuthorityRequirementMismatch { inferred, declared } => write!(
                formatter,
                "inferred authorities {inferred:?} exceed declared requirements {declared:?}"
            ),
            Self::Registry(error) => Display::fmt(error, formatter),
        }
    }
}

impl Error for PipelineError {}

impl From<RegistryError> for PipelineError {
    fn from(value: RegistryError) -> Self {
        Self::Registry(value)
    }
}

/// A candidate that has passed lowering, effect analysis, and authority declaration checks.
#[derive(Clone, Debug)]
pub struct PreparedCandidate {
    ir: ProgramIR,
    artifact: ProcedureArtifact,
    entry_procedure: String,
    effect_analysis: EffectAnalysis,
}

impl PreparedCandidate {
    /// Reports that a prepared candidate is temporary until it is admitted to a registry.
    #[must_use]
    pub const fn retention_state(&self) -> RetentionState {
        RetentionState::Ephemeral
    }

    #[must_use]
    pub const fn artifact(&self) -> &ProcedureArtifact {
        &self.artifact
    }

    #[must_use]
    pub const fn effect_analysis(&self) -> &EffectAnalysis {
        &self.effect_analysis
    }

    #[must_use]
    pub fn entry_procedure(&self) -> &str {
        &self.entry_procedure
    }

    /// Admits the candidate first, then executes the pinned candidate entry in the supplied session.
    pub fn admit_and_execute<S: RegistryStore>(
        self,
        registry: &mut ProcedureRegistry<S>,
        session: &mut Session,
        arguments: Vec<Value>,
        catalog: &[HostFunctionDescriptor],
        grants: &[Grant],
        host: &mut dyn HostFunctionProvider,
    ) -> Result<(Admission, silk_runtime::ExecutionResult), PipelineError> {
        let admission = registry.admit(self.artifact)?;
        let execution = session
            .execute(
                &self.ir,
                &self.entry_procedure,
                arguments,
                catalog,
                grants,
                host,
            )
            .map_err(|error| PipelineError::Execution(error.to_string()))?;
        Ok((admission, execution))
    }
}

/// Lowers, analyzes, validates, and packages a candidate without executing it.
pub fn prepare_candidate(
    source: &str,
    mut request: CandidateRequest,
    catalog: &[HostFunctionDescriptor],
    policy: &PipelinePolicy,
) -> Result<PreparedCandidate, PipelineError> {
    if request.procedure_id.trim().is_empty() {
        return Err(PipelineError::EmptyProcedureId);
    }
    if !policy.allowed_origins.contains(&request.provenance.origin) {
        return Err(PipelineError::InvalidOrigin(request.provenance.origin));
    }
    for profile in &request.required_validation_profiles {
        if profile != "silk.syntax_lowering.v1" && profile != "silk.effects_authority.v1" {
            return Err(PipelineError::UnsupportedValidationProfile(profile.clone()));
        }
    }
    let mut ir = lower(source).map_err(PipelineError::Parse)?;
    let entry_id = if request.entry_procedure.starts_with("proc:") {
        request.entry_procedure.clone()
    } else {
        format!("proc:{}", request.entry_procedure)
    };
    if !ir.procedures.contains_key(&entry_id) {
        return Err(PipelineError::MissingEntry(request.entry_procedure));
    }

    let analysis = analyze_candidate(&ir, catalog);
    let summary = analysis
        .procedures
        .get(&entry_id)
        .expect("effect analysis emits a summary for every procedure");
    let inferred_effects = summary
        .transitive_effects
        .iter()
        .map(effect_label)
        .collect::<BTreeSet<_>>();
    if !inferred_effects.is_subset(&request.contract.effect_ceiling) {
        return Err(PipelineError::EffectCeilingMismatch {
            inferred: inferred_effects,
            declared: request.contract.effect_ceiling,
        });
    }
    let inferred_authorities = summary.required_authorities.clone();
    if !inferred_authorities.is_subset(&request.contract.required_authorities) {
        return Err(PipelineError::AuthorityRequirementMismatch {
            inferred: inferred_authorities,
            declared: request.contract.required_authorities,
        });
    }

    // The serialized payload excludes source text and source spans. The current IR's
    // provisional internal procedure names remain until SILK-13's IR migration.
    ir.program_id = "silk.candidate.v1".to_owned();
    ir.source_map.clear();
    let executable_semantics = serde_json::to_value(&ir).expect("semantic IR serializes");
    let source_digest = format!("sha256:{:x}", Sha256::digest(source.as_bytes()));
    request.provenance.source_digest = Some(source_digest);
    let mut artifact = ProcedureArtifact {
        schema_version: ARTIFACT_SCHEMA_VERSION.to_owned(),
        procedure_id: request.procedure_id,
        revision_digest: format!("sha256:{}", "0".repeat(64)),
        name: request.name,
        semantic_description: request.semantic_description,
        contract: request.contract,
        executable_semantics,
        lineage: Vec::new(),
        provenance: request.provenance,
        validation_evidence: Vec::new(),
    };
    artifact.revision_digest = calculate_revision_digest(&artifact)?;
    artifact.validation_evidence = request
        .required_validation_profiles
        .iter()
        .map(|profile| {
            let evidence = json!({
                "profile": profile,
                "subject_revision_digest": artifact.revision_digest,
                "source_digest": artifact.provenance.source_digest,
                "effect_analysis": summary,
            });
            ValidationEvidence {
                validator: if profile == "silk.syntax_lowering.v1" {
                    "silk-syntax::lower".to_owned()
                } else {
                    "silk-runtime::analyze_effects".to_owned()
                },
                validator_version: env!("CARGO_PKG_VERSION").to_owned(),
                profile: profile.clone(),
                subject_revision_digest: artifact.revision_digest.clone(),
                outcome: ValidationOutcome::Passed,
                evidence_digest: format!(
                    "sha256:{:x}",
                    Sha256::digest(serde_json::to_vec(&evidence).expect("evidence serializes"))
                ),
            }
        })
        .collect();

    Ok(PreparedCandidate {
        ir,
        artifact,
        entry_procedure: entry_id,
        effect_analysis: analysis,
    })
}

fn analyze_candidate(ir: &ProgramIR, catalog: &[HostFunctionDescriptor]) -> EffectAnalysis {
    let host_functions = catalog
        .iter()
        .cloned()
        .map(|descriptor| (descriptor.name.clone(), descriptor))
        .collect::<BTreeMap<_, _>>();
    let procedures = ir
        .procedures
        .iter()
        .map(|(procedure_id, procedure)| {
            let mut facts = ProcedureEffects::default();
            for block in procedure.blocks.values() {
                for instruction in &block.instructions {
                    match &instruction.op {
                        Operation::LoadState { .. } => {
                            facts.local_effects.insert(Effect::SessionStateRead);
                        }
                        Operation::InitState { .. } | Operation::StoreState { .. } => {
                            facts.local_effects.insert(Effect::SessionStateWrite);
                        }
                        Operation::Call { callee, target, .. } => {
                            let target = match target {
                                CallTarget::Procedure => {
                                    EffectCallTarget::Procedure(format!("proc:{callee}"))
                                }
                                CallTarget::HostFunction => {
                                    EffectCallTarget::HostFunction(callee.clone())
                                }
                                CallTarget::Dynamic => EffectCallTarget::Dynamic {
                                    procedure_candidates: BTreeSet::new(),
                                    host_function_candidates: BTreeSet::new(),
                                    possible_effects: [Effect::Unknown].into_iter().collect(),
                                },
                                CallTarget::Core => match callee.as_str() {
                                    "print" => {
                                        facts.local_effects.insert(Effect::ProcessOutput);
                                        continue;
                                    }
                                    "now" => {
                                        facts.local_effects.insert(Effect::Clock);
                                        continue;
                                    }
                                    "sleep" => {
                                        facts.local_effects.insert(Effect::Delay);
                                        continue;
                                    }
                                    _ => {
                                        facts.local_effects.insert(Effect::Unknown);
                                        continue;
                                    }
                                },
                            };
                            facts.calls.push(EffectCall {
                                site_id: instruction.id.clone(),
                                target,
                            });
                        }
                        _ => {}
                    }
                }
            }
            (procedure_id.clone(), facts)
        })
        .collect();
    analyze_effects(&procedures, &host_functions)
}

fn effect_label(effect: &Effect) -> String {
    serde_json::to_value(effect)
        .expect("effect serializes")
        .as_str()
        .expect("effect label is a string")
        .to_owned()
}
