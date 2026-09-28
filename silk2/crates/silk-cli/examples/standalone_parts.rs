use std::collections::{BTreeMap, BTreeSet};

use serde_json::{Value, json};
use silk_ir::ProgramIR;
use silk_protocol::{Effect, Grant, HostFunctionDescriptor};
use silk_registry::{
    MemoryStore, Origin, ProcedureContract, ProcedureRegistry, Provenance, RegistryPolicy,
    SemanticDescription,
};
use silk_runtime::{HostFunctionProvider, Session};
use silk_synthesis::{CandidateRequest, PipelinePolicy, prepare_candidate};

const CHECK_SOURCE: &str = r#"
fn main(request) {
    let available = inventory.available(request.part);
    return {"part": request.part, "available": available};
}
"#;

const RESERVE_SOURCE: &str = r#"
fn main(check) {
    if check.available {
        return inventory.reserve(check.part);
    }
    return {"part": check.part, "reserved": false};
}
"#;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let input_schema = json!({
        "type": "object",
        "properties": {"part": {"type": "string"}},
        "required": ["part"]
    });
    let availability_schema = json!({
        "type": "object",
        "properties": {
            "part": {"type": "string"},
            "available": {"type": "boolean"}
        },
        "required": ["part", "available"]
    });
    let reservation_schema = json!({
        "type": "object",
        "properties": {
            "part": {"type": "string"},
            "reserved": {"type": "boolean"}
        },
        "required": ["part", "reserved"]
    });
    let catalog = vec![
        HostFunctionDescriptor {
            name: "inventory.available".to_owned(),
            authority: "inventory.read".to_owned(),
            effects: [Effect::HostRead].into_iter().collect(),
        },
        HostFunctionDescriptor {
            name: "inventory.reserve".to_owned(),
            authority: "inventory.reserve".to_owned(),
            effects: [Effect::HostWrite].into_iter().collect(),
        },
    ];
    let grants = vec![
        Grant {
            authority: "inventory.read".to_owned(),
            effect_ceiling: [Effect::HostRead].into_iter().collect(),
        },
        Grant {
            authority: "inventory.reserve".to_owned(),
            effect_ceiling: [Effect::HostWrite].into_iter().collect(),
        },
    ];
    let profiles = [
        "silk.syntax_lowering.v1".to_owned(),
        "silk.effects_authority.v1".to_owned(),
    ]
    .into_iter()
    .collect::<BTreeSet<_>>();
    let mut registry = ProcedureRegistry::new(
        MemoryStore::default(),
        RegistryPolicy {
            required_validation_profiles: profiles.clone(),
        },
    );

    let check_contract = ProcedureContract {
        purpose: "Check whether a requested part is in stock".to_owned(),
        input_schema: input_schema.clone(),
        output_schema: availability_schema.clone(),
        preconditions: Vec::new(),
        postconditions: Vec::new(),
        effect_ceiling: ["host_read".to_owned()].into_iter().collect(),
        required_authorities: ["inventory.read".to_owned()].into_iter().collect(),
        dependencies: Vec::new(),
    };
    let reserve_contract = ProcedureContract {
        purpose: "Reserve a requested part when it is available".to_owned(),
        input_schema: availability_schema.clone(),
        output_schema: reservation_schema.clone(),
        preconditions: Vec::new(),
        postconditions: Vec::new(),
        effect_ceiling: ["host_write".to_owned()].into_iter().collect(),
        required_authorities: ["inventory.reserve".to_owned()].into_iter().collect(),
        dependencies: Vec::new(),
    };

    for (id, name, source, contract, terms) in [
        (
            "urn:silk:demo:part-check",
            "part availability",
            CHECK_SOURCE,
            check_contract,
            ["inventory", "availability"]
                .into_iter()
                .map(str::to_owned)
                .collect(),
        ),
        (
            "urn:silk:demo:part-reserve",
            "part reservation",
            RESERVE_SOURCE,
            reserve_contract,
            ["inventory", "reservation"]
                .into_iter()
                .map(str::to_owned)
                .collect(),
        ),
    ] {
        let request = CandidateRequest {
            procedure_id: id.to_owned(),
            name: Some(name.to_owned()),
            semantic_description: SemanticDescription {
                summary: contract.purpose.clone(),
                terms,
            },
            contract,
            provenance: Provenance {
                origin: Origin::Generated,
                source_reference: Some("silk-cli/examples/standalone_parts.rs".to_owned()),
                source_digest: None,
                generator: Some("standalone-parts-demo".to_owned()),
                composition_plan: None,
                claims: BTreeMap::new(),
            },
            entry_procedure: "main".to_owned(),
            required_validation_profiles: profiles.clone(),
        };
        let prepared = prepare_candidate(source, request, &catalog, &PipelinePolicy::default())?;
        registry.admit(prepared.artifact().clone())?;
    }

    let effect_filter: BTreeSet<String> = ["host_read".to_owned(), "host_write".to_owned()]
        .into_iter()
        .collect();
    let authority_filter: BTreeSet<String> =
        ["inventory.read".to_owned(), "inventory.reserve".to_owned()]
            .into_iter()
            .collect();
    for (input, output, preferred) in [
        (&input_schema, &availability_schema, "availability"),
        (&availability_schema, &reservation_schema, "reservation"),
    ] {
        let matches = registry.retrieve(&silk_registry::RetrievalQuery {
            intent: preferred.to_owned(),
            preferred_terms: [preferred.to_owned()].into_iter().collect(),
            input_schema: Some(input.clone()),
            output_schema: Some(output.clone()),
            allowed_effects: Some(effect_filter.clone()),
            available_authorities: Some(authority_filter.clone()),
            required_validation_profiles: profiles.clone(),
            ..Default::default()
        });
        let Some(candidate) = matches.candidates.first() else {
            return Err(format!("no retrieval match for {preferred}").into());
        };
        println!(
            "retrieved {}@{}: {} (score {}, {})",
            candidate.artifact.procedure_id,
            candidate.artifact.revision_digest,
            candidate.artifact.contract.purpose,
            candidate.score,
            candidate.explanation.join("; ")
        );
    }

    let request = silk_registry::CompositionRequest {
        input_schema: input_schema.clone(),
        output_schema: reservation_schema,
        allowed_effects: effect_filter,
        available_authorities: authority_filter,
        required_validation_profiles: profiles,
        max_steps: 4,
    };
    let plan = registry.compose(&request)?;
    println!(
        "composition {} has {} steps",
        plan.plan_digest,
        plan.steps.len()
    );

    let mut session = Session::default();
    let mut inventory = DemoInventory {
        stock: [("gear-42".to_owned(), 1)].into_iter().collect(),
    };
    let mut value = json!({"part": "gear-42"});
    for step in &plan.steps {
        let artifact = registry
            .get(&step.procedure_id, &step.revision_digest)
            .expect("composition pins registered revisions");
        let program: ProgramIR = serde_json::from_value(artifact.executable_semantics.clone())?;
        let result = session.execute(
            &program,
            "main",
            vec![value],
            &catalog,
            &grants,
            &mut inventory,
        )?;
        println!(
            "executed {}: effects={:?}, authorities={:?}, result={}",
            artifact.name.as_deref().unwrap_or(&artifact.procedure_id),
            artifact.contract.effect_ceiling,
            artifact.contract.required_authorities,
            result.value
        );
        println!(
            "  trace events: {:?}",
            result
                .trace
                .iter()
                .map(|event| event.event)
                .collect::<Vec<_>>()
        );
        value = result.value;
    }
    println!("outcome: {value}");
    Ok(())
}

struct DemoInventory {
    stock: BTreeMap<String, u64>,
}

impl HostFunctionProvider for DemoInventory {
    fn call(&mut self, function: &str, arguments: &[Value]) -> Result<Value, String> {
        let part = arguments
            .first()
            .and_then(Value::as_str)
            .ok_or_else(|| "expected part number string".to_owned())?;
        match function {
            "inventory.available" => {
                Ok(Value::Bool(self.stock.get(part).copied().unwrap_or(0) > 0))
            }
            "inventory.reserve" => {
                let count = self.stock.entry(part.to_owned()).or_default();
                if *count == 0 {
                    return Err(format!("no stock for {part}"));
                }
                *count -= 1;
                Ok(json!({"part": part, "reserved": true}))
            }
            _ => Err(format!("unknown demo host function {function}")),
        }
    }
}
