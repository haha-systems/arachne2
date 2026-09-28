//! Length-prefixed JSON-RPC server for the public Silk Runtime Protocol subset.

use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::io::{self, Read, Write};

use serde::Deserialize;
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use silk_ir::ProgramIR;
use silk_protocol::{Effect, Grant, HostFunctionDescriptor, PROTOCOL_VERSION};
use silk_registry::{
    Admission, MemoryStore as RegistryMemoryStore, ProcedureArtifact, ProcedureRegistry,
    RegistryPolicy, RetentionDecision, RetentionState,
};
use silk_runtime::{HostFunctionProvider, Session};
use silk_syntax::lower;
use silk_synthesis::{CandidateRequest, PipelinePolicy, prepare_candidate};

const MAX_FRAME_BYTES: usize = 16 * 1024 * 1024;

pub fn serve() -> io::Result<()> {
    let stdin = io::stdin();
    let stdout = io::stdout();
    serve_io(stdin.lock(), stdout.lock())
}

fn serve_io(mut input: impl Read, mut output: impl Write) -> io::Result<()> {
    let mut initialized = false;
    let mut sessions = HashMap::<String, WireSession>::new();
    let mut registry = ProcedureRegistry::new(
        RegistryMemoryStore::default(),
        RegistryPolicy {
            required_validation_profiles: [
                "silk.syntax_lowering.v1".to_owned(),
                "silk.effects_authority.v1".to_owned(),
            ]
            .into_iter()
            .collect(),
        },
    );
    let mut registered_programs = HashMap::<(String, String), ProgramIR>::new();
    let mut registered_entries = HashMap::<(String, String), String>::new();
    let mut next_host_id = 1_u64;
    while let Some(frame) = read_frame(&mut input)? {
        let request: Value = match serde_json::from_slice(&frame) {
            Ok(value) => value,
            Err(error) => {
                write_error(
                    &mut output,
                    Value::Null,
                    -32700,
                    "parse error",
                    json!({"detail":error.to_string()}),
                )?;
                continue;
            }
        };
        let id = request.get("id").cloned();
        let Some(id_value) = id else { continue };
        let method = request
            .get("method")
            .and_then(Value::as_str)
            .unwrap_or_default();
        let params = request.get("params").cloned().unwrap_or_else(|| json!({}));
        let result = match method {
            "initialize" => initialize(&params, &mut initialized),
            "session.create" if initialized => create_session(&params, &mut sessions),
            "session.close" if initialized => close_session(&params, &mut sessions),
            "program.load" if initialized => load_program(&params, &mut sessions),
            "candidate.prepare" if initialized => prepare_procedure_candidate(&params, &sessions),
            "registry.admit" if initialized => admit_candidate(
                &params,
                &mut registry,
                &mut registered_programs,
                &mut registered_entries,
            ),
            "registry.retain" if initialized => retain_candidate(&params, &mut registry),
            "registry.run" if initialized => run_retained_procedure(
                &params,
                &mut sessions,
                RetainedProcedureRegistry {
                    registry: &registry,
                    programs: &registered_programs,
                    entries: &registered_entries,
                },
                &mut input,
                &mut output,
                &mut next_host_id,
            )?,
            "procedure.run" if initialized => run_procedure(
                &params,
                &mut sessions,
                &mut input,
                &mut output,
                &mut next_host_id,
            )?,
            _ if !initialized => Err((-32001, "not initialized", json!({}))),
            _ => Err((-32601, "method not found", json!({"method":method}))),
        };
        match result {
            Ok(value) => write_result(&mut output, id_value, value)?,
            Err((code, message, data)) => {
                write_error(&mut output, id_value, code, message, data)?;
                if method == "initialize" && code == -32003 {
                    return Ok(());
                }
            }
        }
    }
    Ok(())
}

#[derive(Deserialize)]
struct HostFunctionInput {
    name: String,
    authority: String,
    effects: BTreeSet<Effect>,
    #[serde(rename = "inputSchema")]
    input_schema: Value,
}

#[derive(Deserialize)]
struct Limits {
    fuel: Option<u64>,
    call_depth: Option<usize>,
    timeout_ms: Option<u64>,
}

struct WireSession {
    runtime: Session,
    program: Option<ProgramIR>,
    descriptors: Vec<HostFunctionDescriptor>,
    schemas: BTreeMap<String, Value>,
    grants: Vec<Grant>,
    limits: Limits,
    input: Option<Value>,
}

fn initialize(params: &Value, initialized: &mut bool) -> Result<Value, RpcFailure> {
    if *initialized {
        return Err(failure(-32002, "already initialized", json!({})));
    }
    let requested = params
        .get("protocol_version")
        .and_then(Value::as_str)
        .unwrap_or_default();
    if requested != PROTOCOL_VERSION {
        return Err(failure(
            -32003,
            "unsupported protocol version",
            json!({"supported":PROTOCOL_VERSION}),
        ));
    }
    *initialized = true;
    Ok(
        json!({"protocol_version":PROTOCOL_VERSION,"features":["session.create","program.load","procedure.run","session.close","host.call","trace.emit","candidate.prepare","registry.admit","registry.retain","registry.run"]}),
    )
}

fn create_session(
    params: &Value,
    sessions: &mut HashMap<String, WireSession>,
) -> Result<Value, RpcFailure> {
    let session_id = required_string(params, "session_id")?;
    if sessions.contains_key(&session_id) {
        return Err(failure(
            -32004,
            "duplicate session ID",
            json!({"session_id":session_id}),
        ));
    }
    let host_functions: Vec<HostFunctionInput> = serde_json::from_value(
        params
            .get("host_functions")
            .cloned()
            .unwrap_or_else(|| json!([])),
    )
    .map_err(|error| {
        failure(
            -32602,
            "invalid host function catalog",
            json!({"detail":error.to_string()}),
        )
    })?;
    let grants: Vec<Grant> =
        serde_json::from_value(params.get("grants").cloned().unwrap_or_else(|| json!([])))
            .map_err(|error| {
                failure(
                    -32602,
                    "invalid grants",
                    json!({"detail":error.to_string()}),
                )
            })?;
    let limits: Limits =
        serde_json::from_value(params.get("limits").cloned().unwrap_or_else(|| json!({})))
            .map_err(|error| {
                failure(
                    -32602,
                    "invalid limits",
                    json!({"detail":error.to_string()}),
                )
            })?;
    let fuel = limits.fuel.unwrap_or(100_000);
    let call_depth = limits.call_depth.unwrap_or(128);
    if fuel == 0 || call_depth == 0 || limits.timeout_ms == Some(0) {
        return Err(failure(-32602, "limits must be positive", json!({})));
    }
    let mut descriptors = Vec::new();
    let mut schemas = BTreeMap::new();
    for host in host_functions {
        if host.name.trim().is_empty()
            || host.authority.trim().is_empty()
            || !host.input_schema.is_object()
        {
            return Err(failure(
                -32602,
                "invalid host function descriptor",
                json!({"function":host.name}),
            ));
        }
        if schemas
            .insert(host.name.clone(), host.input_schema)
            .is_some()
        {
            return Err(failure(
                -32602,
                "duplicate host function name",
                json!({"function":host.name}),
            ));
        }
        descriptors.push(HostFunctionDescriptor {
            name: host.name,
            authority: host.authority,
            effects: host.effects,
        });
    }
    let catalog_digest = digest_json(&json!({"host_functions":descriptors,"grants":grants}));
    let runtime = Session::default().with_limits(fuel, call_depth);
    let input = params
        .get("input")
        .filter(|value| !value.is_null())
        .cloned();
    sessions.insert(
        session_id.clone(),
        WireSession {
            runtime,
            program: None,
            descriptors,
            schemas,
            grants,
            limits,
            input,
        },
    );
    Ok(json!({
        "session_id":session_id,
        "protocol_version":PROTOCOL_VERSION,
        "catalog_digest":catalog_digest,
        "limits":{"fuel":fuel,"call_depth":call_depth,"timeout_ms":sessions[&session_id].limits.timeout_ms.unwrap_or(30_000)}
    }))
}

fn load_program(
    params: &Value,
    sessions: &mut HashMap<String, WireSession>,
) -> Result<Value, RpcFailure> {
    let session_id = required_string(params, "session_id")?;
    let source = required_string(params, "source")?;
    let session = sessions
        .get_mut(&session_id)
        .ok_or_else(|| failure(-32005, "unknown session", json!({"session_id":session_id})))?;
    let ir = lower(&source).map_err(|error| {
        failure(
            -32010,
            "program load failed",
            json!({"kind":"ParseError","message":error.to_string()}),
        )
    })?;
    let program_id = ir.program_id.clone();
    let fuel = session.limits.fuel.unwrap_or(100_000);
    let call_depth = session.limits.call_depth.unwrap_or(128);
    session.runtime = Session::default().with_limits(fuel, call_depth);
    session.program = Some(ir);
    Ok(json!({"program_id":program_id,"diagnostics":[]}))
}

fn prepare_procedure_candidate(
    params: &Value,
    sessions: &HashMap<String, WireSession>,
) -> Result<Value, RpcFailure> {
    let session_id = required_string(params, "session_id")?;
    let source = required_string(params, "source")?;
    let request: CandidateRequest = serde_json::from_value(
        params
            .get("candidate_request")
            .cloned()
            .unwrap_or(Value::Null),
    )
    .map_err(|error| {
        failure(
            -32602,
            "invalid candidate request",
            json!({"detail":error.to_string()}),
        )
    })?;
    let session = sessions
        .get(&session_id)
        .ok_or_else(|| failure(-32005, "unknown session", json!({"session_id":session_id})))?;
    let prepared = prepare_candidate(
        &source,
        request,
        &session.descriptors,
        &PipelinePolicy::default(),
    )
    .map_err(|error| {
        failure(
            -32030,
            "candidate preparation failed",
            json!({"detail":error.to_string()}),
        )
    })?;
    Ok(json!({
        "artifact":prepared.artifact(),
        "entry_procedure":prepared.entry_procedure(),
        "retention_state":prepared.retention_state(),
    }))
}

fn admit_candidate(
    params: &Value,
    registry: &mut ProcedureRegistry<RegistryMemoryStore>,
    registered_programs: &mut HashMap<(String, String), ProgramIR>,
    registered_entries: &mut HashMap<(String, String), String>,
) -> Result<Value, RpcFailure> {
    let artifact: ProcedureArtifact = serde_json::from_value(
        params.get("artifact").cloned().unwrap_or(Value::Null),
    )
    .map_err(|error| {
        failure(
            -32602,
            "invalid procedure artifact",
            json!({"detail":error.to_string()}),
        )
    })?;
    let entry_procedure = required_string(params, "entry_procedure")?;
    let program: ProgramIR = serde_json::from_value(artifact.executable_semantics.clone())
        .map_err(|error| {
            failure(
                -32602,
                "artifact executable semantics are invalid",
                json!({"detail":error.to_string()}),
            )
        })?;
    if !program.procedures.contains_key(&entry_procedure) {
        return Err(failure(
            -32602,
            "candidate entry procedure is missing",
            json!({"entry_procedure":entry_procedure}),
        ));
    }
    let key = (
        artifact.procedure_id.clone(),
        artifact.revision_digest.clone(),
    );
    if registered_entries
        .get(&key)
        .is_some_and(|existing| existing != &entry_procedure)
    {
        return Err(failure(
            -32031,
            "candidate entry binding conflicts",
            json!({}),
        ));
    }
    let admission = registry.admit(artifact).map_err(|error| {
        failure(
            -32031,
            "candidate admission failed",
            json!({"detail":error.to_string()}),
        )
    })?;
    let artifact = registry
        .get(&key.0, &key.1)
        .expect("admitted artifact exists");
    let entry = registered_entries
        .entry(key.clone())
        .or_insert(entry_procedure);
    registered_programs.entry(key).or_insert(program);
    let admission = match admission {
        Admission::Inserted => "inserted",
        Admission::AlreadyPresent => "already_present",
    };
    Ok(json!({
        "admission":admission,
        "procedure_id":artifact.procedure_id,
        "revision_digest":artifact.revision_digest,
        "entry_procedure":entry,
        "retention_state":registry.retention_state(&artifact.procedure_id,&artifact.revision_digest),
    }))
}

fn retain_candidate(
    params: &Value,
    registry: &mut ProcedureRegistry<RegistryMemoryStore>,
) -> Result<Value, RpcFailure> {
    let procedure_id = required_string(params, "procedure_id")?;
    let revision_digest = required_string(params, "revision_digest")?;
    let decision: RetentionDecision = serde_json::from_value(
        params.get("decision").cloned().unwrap_or(Value::Null),
    )
    .map_err(|error| {
        failure(
            -32602,
            "invalid retention decision",
            json!({"detail":error.to_string()}),
        )
    })?;
    registry
        .retain(&procedure_id, &revision_digest, decision)
        .map_err(|error| {
            failure(
                -32032,
                "candidate retention failed",
                json!({"detail":error.to_string()}),
            )
        })?;
    Ok(json!({
        "procedure_id":procedure_id,
        "revision_digest":revision_digest,
        "retention_state":registry.retention_state(&procedure_id,&revision_digest),
    }))
}

struct RetainedProcedureRegistry<'a> {
    registry: &'a ProcedureRegistry<RegistryMemoryStore>,
    programs: &'a HashMap<(String, String), ProgramIR>,
    entries: &'a HashMap<(String, String), String>,
}

fn run_retained_procedure<R: Read, W: Write>(
    params: &Value,
    sessions: &mut HashMap<String, WireSession>,
    retained: RetainedProcedureRegistry<'_>,
    input: &mut R,
    output: &mut W,
    next_host_id: &mut u64,
) -> io::Result<Result<Value, RpcFailure>> {
    let prepared = (|| {
        let session_id = required_string(params, "session_id")?;
        let procedure_id = required_string(params, "procedure_id")?;
        let revision_digest = required_string(params, "revision_digest")?;
        let procedure = required_string(params, "procedure")?;
        if retained
            .registry
            .retention_state(&procedure_id, &revision_digest)
            != Some(RetentionState::Retained)
        {
            return Err(failure(
                -32032,
                "procedure revision is not retained",
                json!({"procedure_id":procedure_id,"revision_digest":revision_digest}),
            ));
        }
        let key = (procedure_id, revision_digest);
        if retained.entries.get(&key) != Some(&procedure) {
            return Err(failure(
                -32032,
                "procedure does not match the retained entry",
                json!({"procedure":procedure}),
            ));
        }
        let Some(program) = retained.programs.get(&key).cloned() else {
            return Err(failure(
                -32032,
                "retained executable is unavailable",
                json!({}),
            ));
        };
        Ok((session_id, program))
    })();
    let (session_id, program) = match prepared {
        Ok(value) => value,
        Err(error) => return Ok(Err(error)),
    };
    let Some(session) = sessions.get_mut(&session_id) else {
        return Ok(Err(failure(
            -32005,
            "unknown session",
            json!({"session_id":session_id}),
        )));
    };
    let previous_program = session.program.replace(program);
    let result = run_procedure(params, sessions, input, output, next_host_id);
    if let Some(session) = sessions.get_mut(&session_id) {
        session.program = previous_program;
    }
    result
}

fn close_session(
    params: &Value,
    sessions: &mut HashMap<String, WireSession>,
) -> Result<Value, RpcFailure> {
    let session_id = required_string(params, "session_id")?;
    sessions
        .remove(&session_id)
        .ok_or_else(|| failure(-32005, "unknown session", json!({"session_id":session_id})))?;
    Ok(json!({"session_id":session_id,"closed":true}))
}

fn run_procedure<R: Read, W: Write>(
    params: &Value,
    sessions: &mut HashMap<String, WireSession>,
    input: &mut R,
    output: &mut W,
    next_host_id: &mut u64,
) -> io::Result<Result<Value, RpcFailure>> {
    let parsed = (|| {
        let session_id = required_string(params, "session_id")?;
        let procedure = required_string(params, "procedure")?;
        let session = sessions
            .get_mut(&session_id)
            .ok_or_else(|| failure(-32005, "unknown session", json!({"session_id":session_id})))?;
        let program = session.program.as_ref().ok_or_else(|| {
            failure(
                -32006,
                "no program loaded",
                json!({"session_id":session_id}),
            )
        })?;
        let arguments: Vec<Value> = if let Some(arguments) = params.get("arguments") {
            serde_json::from_value(arguments.clone()).map_err(|error| {
                failure(
                    -32602,
                    "arguments must be an array",
                    json!({"detail":error.to_string()}),
                )
            })?
        } else {
            session.input.clone().into_iter().collect()
        };
        let mut host = WireHost {
            input,
            output,
            next_host_id,
            session_id: &session_id,
            schemas: &session.schemas,
            timeout_ms: session.limits.timeout_ms.unwrap_or(30_000),
        };
        let result = session.runtime.execute(
            program,
            &procedure,
            arguments,
            &session.descriptors,
            &session.grants,
            &mut host,
        );
        match result {
            Ok(result) => {
                for event in &result.trace {
                    write_notification(
                        host.output,
                        "trace.emit",
                        json!({"session_id":session_id,"event":event}),
                    )
                    .map_err(|error| {
                        failure(
                            -32020,
                            "trace delivery failed",
                            json!({"detail":error.to_string()}),
                        )
                    })?;
                }
                Ok(
                    json!({"value":result.value,"output":result.output,"trace_id":result.trace_id,"trace":result.trace}),
                )
            }
            Err(error) => {
                let trace = error.trace.as_deref().cloned().unwrap_or_default();
                for event in &trace {
                    write_notification(
                        host.output,
                        "trace.emit",
                        json!({"session_id":session_id,"event":event}),
                    )
                    .map_err(|io_error| {
                        failure(
                            -32020,
                            "trace delivery failed",
                            json!({"detail":io_error.to_string()}),
                        )
                    })?;
                }
                Err(failure(
                    -32011,
                    "procedure execution failed",
                    json!({
                        "kind":error.kind,"message":error.message,"procedure":error.procedure,
                        "block":error.block,"instruction":error.instruction,"source_span":error.source_span,
                        "trace":trace
                    }),
                ))
            }
        }
    })();
    Ok(parsed)
}

struct WireHost<'a, R, W> {
    input: &'a mut R,
    output: &'a mut W,
    next_host_id: &'a mut u64,
    session_id: &'a str,
    schemas: &'a BTreeMap<String, Value>,
    timeout_ms: u64,
}

impl<R: Read, W: Write> HostFunctionProvider for WireHost<'_, R, W> {
    fn call(&mut self, function: &str, arguments: &[Value]) -> Result<Value, String> {
        let schema = self
            .schemas
            .get(function)
            .ok_or_else(|| format!("missing input schema for {function}"))?;
        let arguments = map_arguments(schema, arguments)?;
        let id = *self.next_host_id;
        *self.next_host_id += 1;
        write_frame(self.output, &json!({
            "jsonrpc":"2.0","id":format!("host-{id}"),"method":"host.call",
            "params":{"session_id":self.session_id,"call_id":format!("host-{id}"),"function":function,
            "arguments":arguments,"deadline_ms":self.timeout_ms}
        })).map_err(|error| error.to_string())?;
        let response = read_frame(self.input)
            .map_err(|error| error.to_string())?
            .ok_or_else(|| "host closed protocol stream during call".to_owned())?;
        let response: Value =
            serde_json::from_slice(&response).map_err(|error| error.to_string())?;
        if response.get("id") != Some(&json!(format!("host-{id}"))) {
            return Err("host response ID did not match pending call".to_owned());
        }
        if let Some(error) = response.get("error") {
            return Err(error
                .get("message")
                .and_then(Value::as_str)
                .unwrap_or("host call failed")
                .to_owned());
        }
        response
            .get("result")
            .cloned()
            .ok_or_else(|| "host response omitted result".to_owned())
    }
}

fn map_arguments(schema: &Value, arguments: &[Value]) -> Result<Value, String> {
    if arguments.len() == 1 && arguments[0].is_object() {
        validate_schema(schema, &arguments[0])?;
        return Ok(arguments[0].clone());
    }
    let properties = schema
        .get("properties")
        .and_then(Value::as_object)
        .ok_or_else(|| "input schema requires object properties".to_owned())?;
    let required = schema
        .get("required")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    let mut names = required
        .iter()
        .filter_map(Value::as_str)
        .map(str::to_owned)
        .collect::<Vec<_>>();
    let mut optional = properties
        .keys()
        .filter(|name| !names.contains(name))
        .cloned()
        .collect::<Vec<_>>();
    optional.sort();
    names.extend(optional);
    if arguments.len() < required.len() || arguments.len() > names.len() {
        return Err(format!(
            "host function expects {}..={} positional arguments, got {}",
            required.len(),
            names.len(),
            arguments.len()
        ));
    }
    let object = names
        .iter()
        .take(arguments.len())
        .zip(arguments)
        .map(|(key, value)| (key.clone(), value.clone()))
        .collect::<serde_json::Map<_, _>>();
    let value = Value::Object(object);
    validate_schema(schema, &value)?;
    Ok(value)
}

fn validate_schema(schema: &Value, value: &Value) -> Result<(), String> {
    let expected = schema
        .get("type")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let valid = match expected {
        "object" => value.is_object(),
        "array" => value.is_array(),
        "string" => value.is_string(),
        "boolean" => value.is_boolean(),
        "number" => value.is_number(),
        "integer" => value.as_i64().is_some() || value.as_u64().is_some(),
        "null" => value.is_null(),
        _ => false,
    };
    if !valid {
        return Err(format!(
            "host arguments do not match input schema type `{expected}`"
        ));
    }
    if let (Some(required), Some(object)) = (
        schema.get("required").and_then(Value::as_array),
        value.as_object(),
    ) {
        for name in required.iter().filter_map(Value::as_str) {
            if !object.contains_key(name) {
                return Err(format!("host arguments missing required property `{name}`"));
            }
        }
    }
    if let (Some(properties), Some(object)) = (
        schema.get("properties").and_then(Value::as_object),
        value.as_object(),
    ) {
        if schema.get("additionalProperties") == Some(&Value::Bool(false))
            && object.keys().any(|key| !properties.contains_key(key))
        {
            return Err("host arguments contain an undeclared property".to_owned());
        }
        for (name, property_schema) in properties {
            if let Some(property_value) = object.get(name) {
                validate_schema(property_schema, property_value)
                    .map_err(|error| format!("property `{name}`: {error}"))?;
            }
        }
    }
    Ok(())
}

type RpcFailure = (i64, &'static str, Value);

fn failure(code: i64, message: &'static str, data: Value) -> RpcFailure {
    (code, message, data)
}

fn required_string(params: &Value, name: &'static str) -> Result<String, RpcFailure> {
    params
        .get(name)
        .and_then(Value::as_str)
        .filter(|value| !value.trim().is_empty())
        .map(str::to_owned)
        .ok_or_else(|| {
            failure(
                -32602,
                "missing required string parameter",
                json!({"parameter":name}),
            )
        })
}

fn digest_json(value: &Value) -> String {
    format!(
        "sha256:{:x}",
        Sha256::digest(serde_json::to_vec(value).expect("JSON value serializes"))
    )
}

fn read_frame(input: &mut impl Read) -> io::Result<Option<Vec<u8>>> {
    let mut prefix = [0_u8; 4];
    let mut read = 0;
    while read < prefix.len() {
        match input.read(&mut prefix[read..])? {
            0 if read == 0 => return Ok(None),
            0 => {
                return Err(io::Error::new(
                    io::ErrorKind::UnexpectedEof,
                    "truncated frame prefix",
                ));
            }
            count => read += count,
        }
    }
    let length = u32::from_be_bytes(prefix) as usize;
    if length == 0 || length > MAX_FRAME_BYTES {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "invalid frame length",
        ));
    }
    let mut frame = vec![0; length];
    input.read_exact(&mut frame)?;
    Ok(Some(frame))
}

fn write_frame(output: &mut impl Write, value: &Value) -> io::Result<()> {
    let bytes = serde_json::to_vec(value).map_err(io::Error::other)?;
    if bytes.is_empty() || bytes.len() > MAX_FRAME_BYTES {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "frame size out of bounds",
        ));
    }
    output.write_all(&(bytes.len() as u32).to_be_bytes())?;
    output.write_all(&bytes)?;
    output.flush()
}

fn write_result(output: &mut impl Write, id: Value, result: Value) -> io::Result<()> {
    write_frame(output, &json!({"jsonrpc":"2.0","id":id,"result":result}))
}

fn write_error(
    output: &mut impl Write,
    id: Value,
    code: i64,
    message: &str,
    data: Value,
) -> io::Result<()> {
    write_frame(
        output,
        &json!({"jsonrpc":"2.0","id":id,"error":{"code":code,"message":message,"data":data}}),
    )
}

fn write_notification(output: &mut impl Write, method: &str, params: Value) -> io::Result<()> {
    write_frame(
        output,
        &json!({"jsonrpc":"2.0","method":method,"params":params}),
    )
}

#[cfg(test)]
mod tests {
    use std::io::Cursor;

    use serde_json::{Value, json};

    use super::{PROTOCOL_VERSION, read_frame, serve_io};

    #[test]
    fn host_call_reaches_wire_only_after_declaration_and_grant_checks() {
        let cases = [
            ("undeclared", false, false, Some("UnknownHostFunction")),
            ("ungranted", true, false, Some("AuthorityDenied")),
            ("granted", true, true, None),
        ];

        for (name, declared, granted, expected_error) in cases {
            let frames = run_host_call_case(declared, granted);
            let host_calls = frames
                .iter()
                .filter(|frame| frame.get("method") == Some(&json!("host.call")))
                .count();
            assert_eq!(host_calls, usize::from(granted), "{name}: host.call count");

            let response = frames
                .iter()
                .find(|frame| frame.get("id") == Some(&json!(4)))
                .expect("procedure response");
            if let Some(kind) = expected_error {
                assert_eq!(
                    response["error"]["data"]["kind"], kind,
                    "{name}: error kind"
                );
            } else {
                assert_eq!(response["result"]["value"], 42, "{name}: host result");
            }
        }
    }

    fn run_host_call_case(declared: bool, granted: bool) -> Vec<Value> {
        let mut input = Vec::new();
        append_request(
            &mut input,
            json!({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocol_version":PROTOCOL_VERSION}}),
        );
        let mut descriptors = Vec::new();
        if declared {
            descriptors.push(json!({
                "name":"catalog.read","authority":"catalog.read","effects":["host_read"],
                "inputSchema":{"type":"object","properties":{"value":{"type":"integer"}},"required":["value"]}
            }));
        }
        let grants = if granted {
            json!([{"authority":"catalog.read","effect_ceiling":["host_read"]}])
        } else {
            json!([])
        };
        append_request(
            &mut input,
            json!({"jsonrpc":"2.0","id":2,"method":"session.create","params":{
                "session_id":"session-1","host_functions":descriptors,"grants":grants
            }}),
        );
        append_request(
            &mut input,
            json!({"jsonrpc":"2.0","id":3,"method":"program.load","params":{
                "session_id":"session-1","source":"fn run() { return catalog.read(7); }"
            }}),
        );
        append_request(
            &mut input,
            json!({"jsonrpc":"2.0","id":4,"method":"procedure.run","params":{
                "session_id":"session-1","procedure":"run","arguments":[]
            }}),
        );
        if granted {
            append_request(
                &mut input,
                json!({"jsonrpc":"2.0","id":"host-1","result":42}),
            );
        }

        let mut output = Vec::new();
        serve_io(Cursor::new(input), &mut output).expect("serve test protocol stream");
        let mut cursor = Cursor::new(output);
        let mut frames = Vec::new();
        while let Some(frame) = read_frame(&mut cursor).expect("read output frame") {
            frames.push(serde_json::from_slice(&frame).expect("valid output JSON"));
        }
        frames
    }

    fn append_request(stream: &mut Vec<u8>, value: Value) {
        let bytes = serde_json::to_vec(&value).expect("serialize test request");
        stream.extend_from_slice(&(bytes.len() as u32).to_be_bytes());
        stream.extend_from_slice(&bytes);
    }
}
