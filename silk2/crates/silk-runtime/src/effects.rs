//! Conservative effect inference over a procedure/call graph.

use std::collections::{BTreeMap, BTreeSet};

use serde::{Deserialize, Serialize};
use silk_protocol::{Effect, HostFunctionDescriptor};

/// Effect and call-site facts supplied by a parser or semantic IR.
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct ProcedureEffects {
    /// Effects caused directly by local operations such as state writes or output.
    pub local_effects: BTreeSet<Effect>,
    /// Calls made directly by this procedure.
    pub calls: Vec<EffectCall>,
    /// Optional upper bound declared for this procedure.
    pub effect_ceiling: Option<BTreeSet<Effect>>,
}

/// One statically observed call site.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct EffectCall {
    /// Stable source or IR location identifier.
    pub site_id: String,
    /// Statically known or conservatively approximated call target.
    pub target: EffectCallTarget,
}

/// The statically known target of a procedure call.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub enum EffectCallTarget {
    /// Call to another procedure in the analyzed program.
    Procedure(String),
    /// Call to a function declared in the active session catalog.
    HostFunction(String),
    /// Dynamic call with a known candidate set and conservative effect bound.
    Dynamic {
        /// Procedure names this dynamic target may resolve to.
        procedure_candidates: BTreeSet<String>,
        /// Host-function names this dynamic target may resolve to.
        host_function_candidates: BTreeSet<String>,
        /// Effects possible in addition to the candidate summaries.
        possible_effects: BTreeSet<Effect>,
    },
}

/// Inferred effects and dependencies for one procedure.
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct ProcedureEffectSummary {
    /// Effects caused directly by the procedure body and its direct host calls.
    pub direct_effects: BTreeSet<Effect>,
    /// Direct and transitive effects reachable from this procedure.
    pub transitive_effects: BTreeSet<Effect>,
    /// Host-function names reachable from this procedure.
    pub host_dependencies: BTreeSet<String>,
    /// Authority identifiers required by reachable host functions.
    pub required_authorities: BTreeSet<String>,
    /// Dynamic or unresolved call-site IDs reachable from this procedure.
    pub unknown_call_sites: BTreeSet<String>,
    /// Whether an explicit effect ceiling contains the inferred set.
    pub effect_ceiling_satisfied: Option<bool>,
}

/// Result of effect inference for all procedures in a program.
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct EffectAnalysis {
    /// Procedure summaries keyed by stable procedure name.
    pub procedures: BTreeMap<String, ProcedureEffectSummary>,
    /// Procedures whose declared ceiling is smaller than their inferred set.
    pub ceiling_violations: Vec<EffectCeilingViolation>,
}

/// An effect ceiling that does not cover the inferred effect set.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct EffectCeilingViolation {
    /// Procedure that declared the insufficient ceiling.
    pub procedure: String,
    /// Effects inferred from direct and transitive calls.
    pub inferred_effects: BTreeSet<Effect>,
    /// Effects permitted by the declaration.
    pub effect_ceiling: BTreeSet<Effect>,
}

/// Computes direct and transitive procedure effects using finite-set union.
///
/// Recursive call components converge because effect sets, dependencies, and
/// unknown-call sites only grow and the input graph is finite. Host functions
/// are resolved by exact descriptor name. An unresolved or dynamic call always
/// includes `unknown`; a dynamic call also includes all known candidate effects.
pub fn analyze_effects(
    procedures: &BTreeMap<String, ProcedureEffects>,
    host_functions: &BTreeMap<String, HostFunctionDescriptor>,
) -> EffectAnalysis {
    let mut summaries: BTreeMap<_, _> = procedures
        .keys()
        .map(|name| (name.clone(), ProcedureEffectSummary::default()))
        .collect();

    loop {
        let previous = summaries.clone();
        let mut changed = false;
        for (name, procedure) in procedures {
            let summary = summarize_procedure(procedure, &previous, procedures, host_functions);
            if previous.get(name) != Some(&summary) {
                summaries.insert(name.clone(), summary);
                changed = true;
            }
        }
        if !changed {
            break;
        }
    }

    let mut ceiling_violations = Vec::new();
    for (name, procedure) in procedures {
        let Some(ceiling) = &procedure.effect_ceiling else {
            continue;
        };
        let summary = summaries
            .get_mut(name)
            .expect("every input procedure has a summary");
        let satisfied = summary.transitive_effects.is_subset(ceiling);
        summary.effect_ceiling_satisfied = Some(satisfied);
        if !satisfied {
            ceiling_violations.push(EffectCeilingViolation {
                procedure: name.clone(),
                inferred_effects: summary.transitive_effects.clone(),
                effect_ceiling: ceiling.clone(),
            });
        }
    }

    EffectAnalysis {
        procedures: summaries,
        ceiling_violations,
    }
}

fn summarize_procedure(
    procedure: &ProcedureEffects,
    previous: &BTreeMap<String, ProcedureEffectSummary>,
    procedures: &BTreeMap<String, ProcedureEffects>,
    host_functions: &BTreeMap<String, HostFunctionDescriptor>,
) -> ProcedureEffectSummary {
    let mut summary = ProcedureEffectSummary {
        direct_effects: procedure.local_effects.clone(),
        transitive_effects: procedure.local_effects.clone(),
        ..ProcedureEffectSummary::default()
    };

    for call in &procedure.calls {
        match &call.target {
            EffectCallTarget::Procedure(name) => {
                merge_procedure_target(name, &call.site_id, previous, procedures, &mut summary);
            }
            EffectCallTarget::HostFunction(name) => {
                merge_host_target(name, &call.site_id, host_functions, &mut summary);
            }
            EffectCallTarget::Dynamic {
                procedure_candidates,
                host_function_candidates,
                possible_effects,
            } => {
                summary.unknown_call_sites.insert(call.site_id.clone());
                summary
                    .direct_effects
                    .extend(possible_effects.iter().copied());
                summary
                    .transitive_effects
                    .extend(possible_effects.iter().copied());
                summary.direct_effects.insert(Effect::Unknown);
                summary.transitive_effects.insert(Effect::Unknown);
                for name in procedure_candidates {
                    merge_procedure_target(name, &call.site_id, previous, procedures, &mut summary);
                }
                for name in host_function_candidates {
                    merge_host_target(name, &call.site_id, host_functions, &mut summary);
                }
            }
        }
    }

    summary
}

fn merge_procedure_target(
    name: &str,
    site_id: &str,
    previous: &BTreeMap<String, ProcedureEffectSummary>,
    procedures: &BTreeMap<String, ProcedureEffects>,
    summary: &mut ProcedureEffectSummary,
) {
    if !procedures.contains_key(name) {
        summary.direct_effects.insert(Effect::Unknown);
        summary.transitive_effects.insert(Effect::Unknown);
        summary.unknown_call_sites.insert(site_id.to_owned());
        return;
    }
    if let Some(callee) = previous.get(name) {
        summary
            .transitive_effects
            .extend(callee.transitive_effects.iter().copied());
        summary
            .host_dependencies
            .extend(callee.host_dependencies.iter().cloned());
        summary
            .required_authorities
            .extend(callee.required_authorities.iter().cloned());
        summary
            .unknown_call_sites
            .extend(callee.unknown_call_sites.iter().cloned());
    }
}

fn merge_host_target(
    name: &str,
    site_id: &str,
    host_functions: &BTreeMap<String, HostFunctionDescriptor>,
    summary: &mut ProcedureEffectSummary,
) {
    summary.host_dependencies.insert(name.to_owned());
    match host_functions.get(name) {
        Some(descriptor) => {
            summary
                .direct_effects
                .extend(descriptor.effects.iter().copied());
            summary
                .transitive_effects
                .extend(descriptor.effects.iter().copied());
            summary
                .required_authorities
                .insert(descriptor.authority.clone());
            if descriptor.effects.contains(&Effect::Unknown) {
                summary.unknown_call_sites.insert(site_id.to_owned());
            }
        }
        None => {
            summary.direct_effects.insert(Effect::Unknown);
            summary.transitive_effects.insert(Effect::Unknown);
            summary.unknown_call_sites.insert(site_id.to_owned());
        }
    }
}

#[cfg(test)]
mod tests {
    use std::collections::{BTreeMap, BTreeSet};

    use silk_protocol::{Effect, HostFunctionDescriptor};

    use super::{EffectCall, EffectCallTarget, ProcedureEffects, analyze_effects};

    fn effects(values: impl IntoIterator<Item = Effect>) -> BTreeSet<Effect> {
        values.into_iter().collect()
    }

    fn host(
        name: &str,
        authority: &str,
        values: impl IntoIterator<Item = Effect>,
    ) -> HostFunctionDescriptor {
        HostFunctionDescriptor {
            name: name.to_owned(),
            authority: authority.to_owned(),
            effects: effects(values),
        }
    }

    fn call(site_id: &str, target: EffectCallTarget) -> EffectCall {
        EffectCall {
            site_id: site_id.to_owned(),
            target,
        }
    }

    #[test]
    fn propagates_effects_dependencies_and_authorities_to_entry_points() {
        let procedures = BTreeMap::from([
            (
                "entry".to_owned(),
                ProcedureEffects {
                    calls: vec![call(
                        "entry:1",
                        EffectCallTarget::Procedure("worker".to_owned()),
                    )],
                    ..ProcedureEffects::default()
                },
            ),
            (
                "worker".to_owned(),
                ProcedureEffects {
                    local_effects: effects([Effect::SessionStateWrite]),
                    calls: vec![call(
                        "worker:1",
                        EffectCallTarget::HostFunction("store.write".to_owned()),
                    )],
                    effect_ceiling: Some(effects([Effect::SessionStateWrite, Effect::HostWrite])),
                },
            ),
        ]);
        let host_functions = BTreeMap::from([(
            "store.write".to_owned(),
            host("store.write", "memory.write", [Effect::HostWrite]),
        )]);

        let analysis = analyze_effects(&procedures, &host_functions);
        let entry = &analysis.procedures["entry"];
        assert_eq!(
            entry.transitive_effects,
            effects([Effect::SessionStateWrite, Effect::HostWrite])
        );
        assert_eq!(
            entry.host_dependencies,
            BTreeSet::from(["store.write".to_owned()])
        );
        assert_eq!(
            entry.required_authorities,
            BTreeSet::from(["memory.write".to_owned()])
        );
        assert_eq!(
            analysis.procedures["worker"].effect_ceiling_satisfied,
            Some(true)
        );
        assert!(analysis.ceiling_violations.is_empty());
    }

    #[test]
    fn recursive_call_components_converge() {
        let procedures = BTreeMap::from([
            (
                "first".to_owned(),
                ProcedureEffects {
                    local_effects: effects([Effect::SessionStateRead]),
                    calls: vec![call(
                        "first:1",
                        EffectCallTarget::Procedure("second".to_owned()),
                    )],
                    ..ProcedureEffects::default()
                },
            ),
            (
                "second".to_owned(),
                ProcedureEffects {
                    local_effects: effects([Effect::HostRead]),
                    calls: vec![call(
                        "second:1",
                        EffectCallTarget::Procedure("first".to_owned()),
                    )],
                    ..ProcedureEffects::default()
                },
            ),
        ]);

        let analysis = analyze_effects(&procedures, &BTreeMap::new());
        let expected = effects([Effect::SessionStateRead, Effect::HostRead]);
        assert_eq!(analysis.procedures["first"].transitive_effects, expected);
        assert_eq!(analysis.procedures["second"].transitive_effects, expected);
    }

    #[test]
    fn dynamic_and_missing_calls_include_unknown_and_report_ceilings() {
        let procedures = BTreeMap::from([(
            "entry".to_owned(),
            ProcedureEffects {
                calls: vec![
                    call(
                        "entry:dynamic",
                        EffectCallTarget::Dynamic {
                            procedure_candidates: BTreeSet::new(),
                            host_function_candidates: BTreeSet::from(["maybe.write".to_owned()]),
                            possible_effects: effects([Effect::HostWrite]),
                        },
                    ),
                    call(
                        "entry:missing",
                        EffectCallTarget::Procedure("absent".to_owned()),
                    ),
                ],
                effect_ceiling: Some(effects([Effect::HostWrite])),
                ..ProcedureEffects::default()
            },
        )]);
        let host_functions = BTreeMap::from([(
            "maybe.write".to_owned(),
            host("maybe.write", "store.write", [Effect::HostWrite]),
        )]);

        let analysis = analyze_effects(&procedures, &host_functions);
        let entry = &analysis.procedures["entry"];
        assert!(entry.transitive_effects.contains(&Effect::Unknown));
        assert!(entry.transitive_effects.contains(&Effect::HostWrite));
        assert_eq!(
            entry.unknown_call_sites,
            BTreeSet::from(["entry:dynamic".to_owned(), "entry:missing".to_owned()])
        );
        assert_eq!(entry.effect_ceiling_satisfied, Some(false));
        assert_eq!(analysis.ceiling_violations.len(), 1);
    }

    #[test]
    fn undeclared_host_function_is_an_unknown_call() {
        let procedures = BTreeMap::from([(
            "entry".to_owned(),
            ProcedureEffects {
                calls: vec![call(
                    "entry:1",
                    EffectCallTarget::HostFunction("missing.call".to_owned()),
                )],
                ..ProcedureEffects::default()
            },
        )]);

        let analysis = analyze_effects(&procedures, &BTreeMap::new());
        let entry = &analysis.procedures["entry"];
        assert!(entry.transitive_effects.contains(&Effect::Unknown));
        assert!(entry.host_dependencies.contains("missing.call"));
        assert!(entry.unknown_call_sites.contains("entry:1"));
    }
}
