//! Deterministic Phase 1 runtime placeholder.

mod effects;
mod evaluator;
mod replay;

pub use effects::{
    EffectAnalysis, EffectCall, EffectCallTarget, EffectCeilingViolation, ProcedureEffectSummary,
    ProcedureEffects, analyze_effects,
};
pub use evaluator::{ExecutionError, ExecutionResult, HostFunctionProvider, Session};
pub use evaluator::{TraceEvent, TraceEventKind};
pub use replay::{
    REPLAY_SCHEMA_VERSION, RecordingProvider, ReplayEntry, ReplayError, ReplayProvider, ReplayTape,
};

use serde::{Deserialize, Serialize};
use silk_protocol::{Effect, Grant, HostFunctionDescriptor};
use silk_syntax::{Program, lower};
use std::collections::BTreeSet;
use std::error::Error;
use std::fmt::{Display, Formatter};

/// Runtime failure when a host function cannot be safely dispatched.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum HostCallError {
    /// No exact descriptor exists in the current session catalog.
    HostFunctionNotDeclared { function: String },
    /// The descriptor exists, but this session cannot authorize its effects.
    AuthorityDenied {
        function: String,
        authority: String,
        required_effects: BTreeSet<Effect>,
        reason: AuthorityDenialReason,
    },
}

impl Display for HostCallError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::HostFunctionNotDeclared { function } => {
                write!(formatter, "host function is not declared: {function}")
            }
            Self::AuthorityDenied {
                function, reason, ..
            } => write!(formatter, "authority denied for {function}: {reason}"),
        }
    }
}

impl Error for HostCallError {}

/// Why a declared host function was denied before dispatch.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthorityDenialReason {
    /// No session grant names the exact required authority identifier.
    GrantMissing,
    /// A matching grant omits one or more required effects.
    EffectCeilingTooNarrow,
    /// Static unknown effects cannot be safely covered by a session grant.
    UnknownEffects,
}

impl Display for AuthorityDenialReason {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(match self {
            Self::GrantMissing => "grant_missing",
            Self::EffectCeilingTooNarrow => "effect_ceiling_too_narrow",
            Self::UnknownEffects => "unknown_effects",
        })
    }
}

/// Describes an authorization decision without exposing the full grant set.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct HostCallAuthorization {
    /// Exact host function name.
    pub function: String,
    /// Required authority identifier.
    pub authority: String,
    /// Declared effect set for the host function.
    pub required_effects: BTreeSet<Effect>,
}

/// Checks a function descriptor and session grants before host code can run.
///
/// The provided closure is invoked exactly once after successful authorization
/// and never invoked for a missing descriptor or denied call.
pub fn dispatch_host_call<T>(
    function: &str,
    catalog: &[HostFunctionDescriptor],
    grants: &[Grant],
    dispatch: impl FnOnce(&HostCallAuthorization) -> T,
) -> Result<T, HostCallError> {
    let authorization = authorize_host_call(function, catalog, grants)?;
    Ok(dispatch(&authorization))
}

/// Returns the exact authorization decision without invoking host code.
pub fn authorize_host_call(
    function: &str,
    catalog: &[HostFunctionDescriptor],
    grants: &[Grant],
) -> Result<HostCallAuthorization, HostCallError> {
    let Some(descriptor) = catalog
        .iter()
        .find(|descriptor| descriptor.name == function)
    else {
        return Err(HostCallError::HostFunctionNotDeclared {
            function: function.to_owned(),
        });
    };

    let denial = if descriptor.effects.contains(&Effect::Unknown) {
        Some(AuthorityDenialReason::UnknownEffects)
    } else {
        let has_matching_grant = grants
            .iter()
            .any(|grant| grant.authority == descriptor.authority);
        let has_covering_grant = grants.iter().any(|grant| {
            grant.authority == descriptor.authority
                && descriptor.effects.is_subset(&grant.effect_ceiling)
        });
        if has_covering_grant {
            None
        } else if has_matching_grant {
            Some(AuthorityDenialReason::EffectCeilingTooNarrow)
        } else {
            Some(AuthorityDenialReason::GrantMissing)
        }
    };

    if let Some(reason) = denial {
        return Err(HostCallError::AuthorityDenied {
            function: descriptor.name.clone(),
            authority: descriptor.authority.clone(),
            required_effects: descriptor.effects.clone(),
            reason,
        });
    }

    let authorization = HostCallAuthorization {
        function: descriptor.name.clone(),
        authority: descriptor.authority.clone(),
        required_effects: descriptor.effects.clone(),
    };
    Ok(authorization)
}

/// Runtime entry point for compiling and evaluating a loaded source program.
#[derive(Clone, Copy, Debug, Default)]
pub struct Runtime;

impl Runtime {
    /// Compiles and executes the synthetic top-level `main` procedure.
    pub fn execute(&self, program: &Program) -> Result<ExecutionResult, ExecutionError> {
        let ir = lower(program.source()).map_err(|error| ExecutionError {
            kind: "ParseError".into(),
            message: error.message().to_owned().into_boxed_str(),
            procedure: None,
            block: None,
            instruction: None,
            source_span: Some(Box::new(error.span().clone())),
            trace: None,
        })?;
        let mut session = Session::default();
        let mut no_host = NoHostFunctions;
        if !ir.procedures.contains_key("proc:main") {
            return Ok(ExecutionResult {
                value: serde_json::Value::Null,
                output: Vec::new(),
                trace_id: "run-1".to_owned(),
                trace: Vec::new(),
            });
        }
        session.execute(&ir, "main", Vec::new(), &[], &[], &mut no_host)
    }
}

struct NoHostFunctions;
impl HostFunctionProvider for NoHostFunctions {
    fn call(
        &mut self,
        function: &str,
        _arguments: &[serde_json::Value],
    ) -> Result<serde_json::Value, String> {
        Err(format!("no host provider is configured for `{function}`"))
    }
}

#[cfg(test)]
mod tests {
    use std::cell::Cell;
    use std::collections::BTreeSet;

    use silk_protocol::{Effect, Grant, HostFunctionDescriptor};

    use silk_syntax::parse;

    use super::{AuthorityDenialReason, HostCallError, Runtime, dispatch_host_call};

    fn host_descriptor(effects: impl IntoIterator<Item = Effect>) -> HostFunctionDescriptor {
        HostFunctionDescriptor {
            name: "language.summarize".to_owned(),
            authority: "language.summarize".to_owned(),
            effects: effects.into_iter().collect(),
        }
    }

    fn host_grant(effects: impl IntoIterator<Item = Effect>) -> Grant {
        Grant {
            authority: "language.summarize".to_owned(),
            effect_ceiling: effects.into_iter().collect(),
        }
    }

    #[test]
    fn exact_matching_grant_dispatches_once() {
        let catalog = [host_descriptor([Effect::HostRead])];
        let grants = [host_grant([Effect::HostRead])];
        let dispatch_count = Cell::new(0);

        let result = dispatch_host_call("language.summarize", &catalog, &grants, |_| {
            dispatch_count.set(dispatch_count.get() + 1);
            "ok"
        })
        .expect("authorized call");

        assert_eq!(result, "ok");
        assert_eq!(dispatch_count.get(), 1);
    }

    #[test]
    fn missing_descriptor_fails_before_dispatch() {
        let dispatch_count = Cell::new(0);

        let result = dispatch_host_call("language.summarize", &[], &[], |_| {
            dispatch_count.set(dispatch_count.get() + 1);
        });

        assert_eq!(
            result,
            Err(HostCallError::HostFunctionNotDeclared {
                function: "language.summarize".to_owned()
            })
        );
        assert_eq!(dispatch_count.get(), 0);
    }

    #[test]
    fn missing_grant_fails_before_dispatch() {
        let catalog = [host_descriptor([Effect::HostRead])];
        let dispatch_count = Cell::new(0);

        let result = dispatch_host_call("language.summarize", &catalog, &[], |_| {
            dispatch_count.set(dispatch_count.get() + 1);
        });

        assert!(matches!(
            result,
            Err(HostCallError::AuthorityDenied {
                reason: AuthorityDenialReason::GrantMissing,
                ..
            })
        ));
        assert_eq!(dispatch_count.get(), 0);
    }

    #[test]
    fn narrow_effect_ceiling_fails_before_dispatch() {
        let catalog = [host_descriptor([Effect::HostRead, Effect::HostWrite])];
        let grants = [host_grant([Effect::HostRead])];
        let dispatch_count = Cell::new(0);

        let result = dispatch_host_call("language.summarize", &catalog, &grants, |_| {
            dispatch_count.set(dispatch_count.get() + 1);
        });

        assert!(matches!(
            result,
            Err(HostCallError::AuthorityDenied {
                reason: AuthorityDenialReason::EffectCeilingTooNarrow,
                ..
            })
        ));
        assert_eq!(dispatch_count.get(), 0);
    }

    #[test]
    fn any_matching_grant_may_cover_the_effect_set() {
        let catalog = [host_descriptor([Effect::HostRead, Effect::HostWrite])];
        let grants = [
            host_grant([Effect::HostRead]),
            host_grant([Effect::HostRead, Effect::HostWrite]),
        ];
        let dispatch_count = Cell::new(0);

        dispatch_host_call("language.summarize", &catalog, &grants, |_| {
            dispatch_count.set(dispatch_count.get() + 1);
        })
        .expect("one exact authority grant covers the effects");

        assert_eq!(dispatch_count.get(), 1);
    }

    #[test]
    fn unknown_effects_fail_closed() {
        let catalog = [host_descriptor([Effect::Unknown])];
        let grants = [host_grant(BTreeSet::from([Effect::Unknown]))];
        let dispatch_count = Cell::new(0);

        let result = dispatch_host_call("language.summarize", &catalog, &grants, |_| {
            dispatch_count.set(dispatch_count.get() + 1);
        });

        assert!(matches!(
            result,
            Err(HostCallError::AuthorityDenied {
                reason: AuthorityDenialReason::UnknownEffects,
                ..
            })
        ));
        assert_eq!(dispatch_count.get(), 0);
    }

    #[test]
    fn execution_result_is_deterministic_and_explicit() {
        let program = parse("print(1)").expect("source wrapper");
        let runtime = Runtime;

        assert_eq!(
            runtime.execute(&program).expect("execution"),
            runtime.execute(&program).expect("execution")
        );
        assert_eq!(
            runtime.execute(&program).expect("execution").value,
            serde_json::Value::Null
        );
    }
}
