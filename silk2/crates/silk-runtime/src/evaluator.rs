//! Graph-walking evaluator for the executable `silk.ir.v1` subset.

use std::collections::BTreeMap;
use std::error::Error;
use std::fmt::{Display, Formatter};

use serde::{Deserialize, Serialize};
use serde_json::{Map, Number, Value, json};
use silk_ir::{
    CallTarget, Instruction, Literal, Operation, Procedure, ProgramIR, SourceSpan, Terminator,
};
use silk_protocol::{Grant, HostFunctionDescriptor};

use crate::{AuthorityDenialReason, HostCallError, authorize_host_call};

const DEFAULT_FUEL: u64 = 100_000;
const DEFAULT_MAX_DEPTH: usize = 128;

/// Synchronous bridge for a host function implementation.
pub trait HostFunctionProvider {
    /// Invokes an already declared and authorized function.
    fn call(&mut self, function: &str, arguments: &[Value]) -> Result<Value, String>;
}

/// A reusable execution session with isolated mutable Silk state.
pub struct Session {
    state: BTreeMap<String, Value>,
    fuel_limit: u64,
    max_call_depth: usize,
    next_run_id: u64,
}

impl Default for Session {
    fn default() -> Self {
        Self {
            state: BTreeMap::new(),
            fuel_limit: DEFAULT_FUEL,
            max_call_depth: DEFAULT_MAX_DEPTH,
            next_run_id: 1,
        }
    }
}

impl Session {
    /// Sets instruction fuel and maximum procedure call depth for each run.
    #[must_use]
    pub fn with_limits(mut self, fuel: u64, max_call_depth: usize) -> Self {
        self.fuel_limit = fuel;
        self.max_call_depth = max_call_depth;
        self
    }

    /// Executes a named procedure in this session.
    pub fn execute(
        &mut self,
        program: &ProgramIR,
        procedure: &str,
        arguments: Vec<Value>,
        catalog: &[HostFunctionDescriptor],
        grants: &[Grant],
        host: &mut dyn HostFunctionProvider,
    ) -> Result<ExecutionResult, ExecutionError> {
        validate_program(program)?;
        let trace_id = format!("run-{}", self.next_run_id);
        self.next_run_id += 1;
        let mut machine = Machine {
            program,
            state: &mut self.state,
            catalog,
            grants,
            host,
            remaining_fuel: self.fuel_limit,
            max_call_depth: self.max_call_depth,
            stack: Vec::new(),
            call_ids: Vec::new(),
            next_call_id: 0,
            trace_id,
            next_sequence: 0,
            trace: Vec::new(),
            output: Vec::new(),
            block: None,
            instruction: None,
        };
        let value = match machine.invoke(procedure, arguments) {
            Ok(value) => value,
            Err(mut error) => {
                error.trace = Some(Box::new(machine.trace));
                return Err(error);
            }
        };
        Ok(ExecutionResult {
            value,
            output: machine.output,
            trace_id: machine.trace_id,
            trace: machine.trace,
        })
    }
}

/// Successful value and output events from one execution.
#[derive(Clone, Debug, PartialEq)]
pub struct ExecutionResult {
    /// Procedure return value.
    pub value: Value,
    /// Lines emitted by the `print` core function.
    pub output: Vec<String>,
    /// Stable identifier correlating every event from this run.
    pub trace_id: String,
    /// Structured execution events in emission order.
    pub trace: Vec<TraceEvent>,
}

impl ExecutionResult {
    /// Encodes output and return value as one stable JSON object.
    pub fn to_json(&self) -> String {
        json!({"output": self.output, "value": self.value, "trace_id": self.trace_id, "trace": self.trace}).to_string()
    }
}

/// Stable event categories emitted by the graph evaluator.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TraceEventKind {
    ProcedureEnter,
    ProcedureExit,
    Branch,
    EffectExecuted,
    HostCallAttempt,
    HostCallAuthorized,
    HostCallDenied,
    HostCallCompleted,
    Error,
}

/// One structured event with sequence and nested-call correlation context.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct TraceEvent {
    pub sequence: u64,
    pub trace_id: String,
    pub correlation_id: String,
    pub parent_correlation_id: Option<String>,
    pub procedure: String,
    pub block: Option<String>,
    pub instruction: Option<String>,
    pub event: TraceEventKind,
    pub data: Value,
}

/// Structured evaluator failure with the active procedure and source location.
#[derive(Clone, Debug, PartialEq)]
pub struct ExecutionError {
    /// Stable error kind.
    pub kind: Box<str>,
    /// Human-readable detail.
    pub message: Box<str>,
    /// Active procedure, when available.
    pub procedure: Option<String>,
    /// Active block, when available.
    pub block: Option<String>,
    /// Active instruction, when available.
    pub instruction: Option<String>,
    /// Source location mapped from the active IR object.
    pub source_span: Option<Box<SourceSpan>>,
    /// Events emitted before this execution failed.
    pub trace: Option<Box<Vec<TraceEvent>>>,
}

impl Display for ExecutionError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        write!(formatter, "{}: {}", self.kind, self.message)?;
        if let Some(span) = &self.source_span {
            write!(formatter, " ({}:{})", span.line, span.column)?;
        }
        Ok(())
    }
}
impl Error for ExecutionError {}

struct Machine<'a> {
    program: &'a ProgramIR,
    state: &'a mut BTreeMap<String, Value>,
    catalog: &'a [HostFunctionDescriptor],
    grants: &'a [Grant],
    host: &'a mut dyn HostFunctionProvider,
    remaining_fuel: u64,
    max_call_depth: usize,
    stack: Vec<String>,
    call_ids: Vec<String>,
    next_call_id: u64,
    trace_id: String,
    next_sequence: u64,
    trace: Vec<TraceEvent>,
    output: Vec<String>,
    block: Option<String>,
    instruction: Option<String>,
}

impl Machine<'_> {
    fn invoke(&mut self, name: &str, arguments: Vec<Value>) -> Result<Value, ExecutionError> {
        if self.stack.len() >= self.max_call_depth {
            return Err(self.error("RecursionLimit", "maximum procedure call depth exceeded"));
        }
        let id = if name.starts_with("proc:") {
            name.to_owned()
        } else {
            format!("proc:{name}")
        };
        let Some(procedure) = self.program.procedures.get(&id) else {
            return Err(self.error("UnknownName", format!("unknown procedure `{name}`")));
        };
        if arguments.len() != procedure.parameters.len() {
            return Err(self.error(
                "ArityError",
                format!(
                    "procedure `{name}` expects {} arguments, received {}",
                    procedure.parameters.len(),
                    arguments.len()
                ),
            ));
        }
        let mut locals: BTreeMap<_, _> = procedure
            .parameters
            .iter()
            .cloned()
            .zip(arguments)
            .collect();
        let mut iterators: BTreeMap<String, IteratorState> = BTreeMap::new();
        let call_id = format!("call-{}", self.next_call_id);
        self.next_call_id += 1;
        self.stack.push(procedure.name.clone());
        self.call_ids.push(call_id);
        self.emit(
            TraceEventKind::ProcedureEnter,
            json!({"arguments": locals.values().collect::<Vec<_>>() }),
        );
        let result = self.run_procedure(procedure, &mut locals, &mut iterators);
        if let Err(error) = &result {
            self.emit(
                TraceEventKind::Error,
                json!({"kind": error.kind, "message": error.message}),
            );
        }
        match &result {
            Ok(value) => self.emit(
                TraceEventKind::ProcedureExit,
                json!({"status":"returned","value":value}),
            ),
            Err(error) => self.emit(
                TraceEventKind::ProcedureExit,
                json!({"status":"error","kind":error.kind}),
            ),
        }
        self.call_ids.pop();
        self.stack.pop();
        result
    }

    fn run_procedure(
        &mut self,
        procedure: &Procedure,
        locals: &mut BTreeMap<String, Value>,
        iterators: &mut BTreeMap<String, IteratorState>,
    ) -> Result<Value, ExecutionError> {
        let mut current = procedure.entry_block.clone();
        let mut predecessor: Option<String> = None;
        loop {
            self.block = Some(current.clone());
            let Some(block) = procedure.blocks.get(&current) else {
                return Err(self.error("InvalidIR", format!("missing block `{current}`")));
            };
            for instruction in &block.instructions {
                self.instruction = Some(instruction.id.clone());
                self.tick()?;
                let execution = self.execute_instruction(
                    instruction,
                    locals,
                    iterators,
                    predecessor.as_deref(),
                );
                let value = match execution {
                    Ok(value) => value,
                    Err(mut error) => {
                        error.procedure = error.procedure.or_else(|| self.stack.last().cloned());
                        error.block = error.block.or_else(|| self.block.clone());
                        error.instruction = error.instruction.or_else(|| self.instruction.clone());
                        error.source_span = error
                            .source_span
                            .or_else(|| self.current_span().map(Box::new));
                        return Err(error);
                    }
                };
                if let Some(value) = value {
                    let Some(result) = &instruction.result else {
                        return Err(
                            self.error("InvalidIR", "value-producing operation has no result ID")
                        );
                    };
                    locals.insert(result.clone(), value);
                }
            }
            self.instruction = None;
            match &block.terminator {
                Terminator::Jump { target }
                | Terminator::Break { target }
                | Terminator::Continue { target } => {
                    predecessor = Some(current);
                    current = target.clone();
                }
                Terminator::Branch {
                    condition,
                    then_target,
                    else_target,
                } => {
                    let value = locals.get(condition).ok_or_else(|| {
                        self.error("InvalidIR", format!("undefined value `{condition}`"))
                    })?;
                    let next = if value.as_bool() == Some(true) {
                        then_target.clone()
                    } else if value.as_bool() == Some(false) {
                        else_target.clone()
                    } else {
                        return Err(self.error("TypeError", "branch condition must be boolean"));
                    };
                    self.emit(
                        TraceEventKind::Branch,
                        json!({"condition":value,"taken":next}),
                    );
                    predecessor = Some(current);
                    current = next;
                }
                Terminator::Return { value } | Terminator::Yield { value } => {
                    return self.value_of(locals, value.as_deref());
                }
                Terminator::End => return Ok(Value::Null),
            }
        }
    }

    fn execute_instruction(
        &mut self,
        instruction: &Instruction,
        locals: &mut BTreeMap<String, Value>,
        iterators: &mut BTreeMap<String, IteratorState>,
        predecessor: Option<&str>,
    ) -> Result<Option<Value>, ExecutionError> {
        let value = |id: &str| {
            locals
                .get(id)
                .cloned()
                .ok_or_else(|| self.error("InvalidIR", format!("undefined value `{id}`")))
        };
        let span = self.current_span();
        match &instruction.op {
            Operation::Constant { value: literal } => Ok(Some(literal_to_json(literal))),
            Operation::Load { name } => locals
                .get(name)
                .cloned()
                .map(Some)
                .ok_or_else(|| self.error("UnknownName", format!("unknown binding `{name}`"))),
            Operation::LoadState { name } => self
                .state
                .get(name)
                .cloned()
                .map(Some)
                .ok_or_else(|| self.error("UnknownName", format!("unknown state `{name}`"))),
            Operation::Bind { name, value: id } => {
                let v = value(id)?;
                locals.insert(name.clone(), v);
                Ok(None)
            }
            Operation::InitState { name, value: id } => {
                let v = value(id)?;
                self.state.entry(name.clone()).or_insert(v);
                self.emit(
                    TraceEventKind::EffectExecuted,
                    json!({"effect":"session_state_write","slot":name}),
                );
                Ok(None)
            }
            Operation::StoreState { name, value: id } => {
                let v = value(id)?;
                let Some(slot) = self.state.get_mut(name) else {
                    return Err(self.error("UnknownName", format!("unknown state `{name}`")));
                };
                *slot = v;
                self.emit(
                    TraceEventKind::EffectExecuted,
                    json!({"effect":"session_state_write","slot":name}),
                );
                Ok(None)
            }
            Operation::Assign { target, value: id } => {
                let v = value(id)?;
                let Some(slot) = locals.get_mut(target) else {
                    return Err(self.error(
                        "ImmutableBinding",
                        format!("unknown local binding `{target}`"),
                    ));
                };
                *slot = v;
                Ok(None)
            }
            Operation::Unary { operator, operand } => Ok(Some(unary(operator, value(operand)?)?)),
            Operation::Binary {
                operator,
                left,
                right,
            } => Ok(Some(binary(operator, value(left)?, value(right)?)?)),
            Operation::Phi { incoming } => incoming
                .iter()
                .find(|(block, _)| Some(block.as_str()) == predecessor)
                .map(|(_, id)| value(id))
                .transpose()?
                .map(Some)
                .ok_or_else(|| self.error("InvalidIR", "phi has no value for predecessor block")),
            Operation::Array { elements } => Ok(Some(Value::Array(
                elements
                    .iter()
                    .map(|id| value(id))
                    .collect::<Result<_, _>>()?,
            ))),
            Operation::Object { entries } => {
                let mut object = Map::new();
                for (key, id) in entries {
                    object.insert(key.clone(), value(id)?);
                }
                Ok(Some(Value::Object(object)))
            }
            Operation::Member { object, key } => {
                let object = value(object)?;
                Ok(Some(object.get(key).cloned().ok_or_else(|| {
                    self.error("MissingKey", format!("missing object key `{key}`"))
                })?))
            }
            Operation::Index { object, index } => index_value(value(object)?, value(index)?)
                .map(Some)
                .map_err(|(kind, message)| self.error(kind, message)),
            Operation::IteratorInit { collection } => {
                let collection = value(collection)?;
                let items = collection
                    .as_array()
                    .cloned()
                    .ok_or_else(|| self.error("TypeError", "for loop requires an array"))?;
                iterators.insert(
                    instruction.result.clone().unwrap_or_default(),
                    IteratorState {
                        items,
                        index: 0,
                        current: None,
                    },
                );
                Ok(Some(Value::Null))
            }
            Operation::IteratorNext { iterator } => {
                let mut state = iterators
                    .get(iterator)
                    .cloned()
                    .ok_or_else(|| self.error("InvalidIR", "unknown iterator"))?;
                state.current = state.items.get(state.index).cloned();
                let has = state.current.is_some();
                if has {
                    state.index += 1;
                }
                iterators.insert(iterator.clone(), state);
                Ok(Some(Value::Bool(has)))
            }
            Operation::IteratorItem { iterator } => iterators
                .get(iterator)
                .and_then(|state| state.current.clone())
                .map(Some)
                .ok_or_else(|| self.error("InvalidIR", "iterator has no current item")),
            Operation::Discard { value: id } => {
                let _ = value(id)?;
                Ok(None)
            }
            Operation::Call {
                callee,
                target,
                arguments,
            } => {
                let args = arguments
                    .iter()
                    .map(|id| value(id))
                    .collect::<Result<Vec<_>, _>>()?;
                let result = match target {
                    CallTarget::Core => self.call_core(callee, &args)?,
                    CallTarget::Procedure => self.invoke(callee, args)?,
                    CallTarget::HostFunction => {
                        let function = callee.clone();
                        self.emit(
                            TraceEventKind::HostCallAttempt,
                            json!({"function":function,"arguments":args}),
                        );
                        let authorization =
                            match authorize_host_call(&function, self.catalog, self.grants) {
                                Ok(authorization) => authorization,
                                Err(error) => {
                                    self.emit(
                                        TraceEventKind::HostCallDenied,
                                        json!({"function":function,"error":error.to_string()}),
                                    );
                                    return Err(self.host_error(error, span.clone()));
                                }
                            };
                        self.emit(TraceEventKind::HostCallAuthorized,json!({"function":authorization.function,"authority":authorization.authority,"effects":authorization.required_effects}));
                        match self.host.call(&function, &args) {
                            Ok(value) => {
                                self.emit(
                                    TraceEventKind::HostCallCompleted,
                                    json!({"function":function,"status":"success","result":value}),
                                );
                                value
                            }
                            Err(message) => {
                                self.emit(
                                    TraceEventKind::HostCallCompleted,
                                    json!({"function":function,"status":"error","message":message}),
                                );
                                return Err(self.error("HostError", message));
                            }
                        }
                    }
                    CallTarget::Dynamic => {
                        return Err(self.error(
                            "NotCallable",
                            "dynamic calls are not supported by this IR evaluator",
                        ));
                    }
                };
                Ok(Some(result))
            }
        }
    }

    fn call_core(&mut self, name: &str, args: &[Value]) -> Result<Value, ExecutionError> {
        match name {
            "print" => {
                self.output
                    .push(args.iter().map(display_value).collect::<Vec<_>>().join(" "));
                self.emit(
                    TraceEventKind::EffectExecuted,
                    json!({"effect":"process_output"}),
                );
                Ok(Value::Null)
            }
            "len" if args.len() == 1 => match &args[0] {
                Value::Array(values) => Ok(json!(values.len())),
                Value::Object(values) => Ok(json!(values.len())),
                Value::String(value) => Ok(json!(value.chars().count())),
                _ => Err(self.error("TypeError", "len expects an array, object, or string")),
            },
            "str" if args.len() == 1 => Ok(Value::String(display_value(&args[0]))),
            "contains" if args.len() == 2 => match &args[0] {
                Value::Array(values) => Ok(Value::Bool(values.contains(&args[1]))),
                Value::String(value) => {
                    if let Some(search) = args[1].as_str() {
                        Ok(Value::Bool(value.contains(search)))
                    } else {
                        Err(self.error("TypeError", "string contains expects a string"))
                    }
                }
                _ => Err(self.error("TypeError", "contains expects an array or string")),
            },
            "append" if args.len() == 2 => {
                let mut values = args[0]
                    .as_array()
                    .cloned()
                    .ok_or_else(|| self.error("TypeError", "append expects an array"))?;
                values.push(args[1].clone());
                Ok(Value::Array(values))
            }
            "sleep" | "now" => Err(self.error(
                "UnavailableCoreFunction",
                format!("core function `{name}` requires the replay-aware runtime provider"),
            )),
            _ => Err(self.error(
                "ArityError",
                format!("unknown core function or invalid arity for `{name}`"),
            )),
        }
    }

    fn value_of(
        &self,
        locals: &BTreeMap<String, Value>,
        id: Option<&str>,
    ) -> Result<Value, ExecutionError> {
        id.map(|id| {
            locals
                .get(id)
                .cloned()
                .ok_or_else(|| self.error("InvalidIR", format!("undefined return value `{id}`")))
        })
        .unwrap_or(Ok(Value::Null))
    }
    fn tick(&mut self) -> Result<(), ExecutionError> {
        if self.remaining_fuel == 0 {
            return Err(self.error("FuelExhausted", "instruction fuel exhausted"));
        }
        self.remaining_fuel -= 1;
        Ok(())
    }
    fn current_span(&self) -> Option<SourceSpan> {
        self.instruction
            .as_ref()
            .and_then(|id| self.program.source_map.get(id).cloned())
            .or_else(|| {
                self.block
                    .as_ref()
                    .and_then(|id| self.program.source_map.get(id).cloned())
            })
    }
    fn error(&self, kind: impl Into<String>, message: impl Into<String>) -> ExecutionError {
        ExecutionError {
            kind: kind.into().into_boxed_str(),
            message: message.into().into_boxed_str(),
            procedure: self.stack.last().cloned(),
            block: self.block.clone(),
            instruction: self.instruction.clone(),
            source_span: self.current_span().map(Box::new),
            trace: None,
        }
    }
    fn host_error(&self, error: HostCallError, span: Option<SourceSpan>) -> ExecutionError {
        let kind = match &error {
            HostCallError::HostFunctionNotDeclared { .. } => "UnknownHostFunction",
            HostCallError::AuthorityDenied {
                reason:
                    AuthorityDenialReason::GrantMissing
                    | AuthorityDenialReason::EffectCeilingTooNarrow
                    | AuthorityDenialReason::UnknownEffects,
                ..
            } => "AuthorityDenied",
        };
        let mut result = self.error(kind, error.to_string());
        result.source_span = span.map(Box::new);
        result
    }

    fn emit(&mut self, event: TraceEventKind, data: Value) {
        let Some(correlation_id) = self.call_ids.last().cloned() else {
            return;
        };
        let parent_correlation_id = self.call_ids.iter().rev().nth(1).cloned();
        let procedure = self.stack.last().cloned().unwrap_or_default();
        self.trace.push(TraceEvent {
            sequence: self.next_sequence,
            trace_id: self.trace_id.clone(),
            correlation_id,
            parent_correlation_id,
            procedure,
            block: self.block.clone(),
            instruction: self.instruction.clone(),
            event,
            data,
        });
        self.next_sequence += 1;
    }
}

fn validate_program(program: &ProgramIR) -> Result<(), ExecutionError> {
    if program.schema_version != silk_ir::SCHEMA_VERSION {
        return Err(simple_error(
            "InvalidIR",
            format!("unsupported IR schema `{}`", program.schema_version),
        ));
    }
    for (id, procedure) in &program.procedures {
        if id != &procedure.id {
            return Err(simple_error(
                "InvalidIR",
                format!("procedure map key `{id}` differs from its embedded ID"),
            ));
        }
        if !procedure.blocks.contains_key(&procedure.entry_block) {
            return Err(simple_error(
                "InvalidIR",
                format!("procedure `{id}` has a missing entry block"),
            ));
        }
        let mut result_ids = std::collections::BTreeSet::new();
        for (block_id, block) in &procedure.blocks {
            if block_id != &block.id {
                return Err(simple_error(
                    "InvalidIR",
                    format!("block map key `{block_id}` differs from its embedded ID"),
                ));
            }
            for instruction in &block.instructions {
                if let Some(result) = &instruction.result {
                    if !result_ids.insert(result) {
                        return Err(simple_error(
                            "InvalidIR",
                            format!("duplicate value ID `{result}` in `{id}`"),
                        ));
                    }
                }
            }
            let targets: Vec<&str> = match &block.terminator {
                Terminator::Jump { target }
                | Terminator::Break { target }
                | Terminator::Continue { target } => vec![target],
                Terminator::Branch {
                    then_target,
                    else_target,
                    ..
                } => vec![then_target, else_target],
                _ => Vec::new(),
            };
            if let Some(target) = targets
                .into_iter()
                .find(|target| !procedure.blocks.contains_key(*target))
            {
                return Err(simple_error(
                    "InvalidIR",
                    format!("block `{block_id}` points to missing successor `{target}`"),
                ));
            }
        }
    }
    Ok(())
}

#[derive(Clone)]
struct IteratorState {
    items: Vec<Value>,
    index: usize,
    current: Option<Value>,
}

fn literal_to_json(value: &Literal) -> Value {
    match value {
        Literal::Null => Value::Null,
        Literal::Boolean(value) => Value::Bool(*value),
        Literal::Integer(value) => json!(value),
        Literal::Number(value) => Number::from_f64(*value).map_or(Value::Null, Value::Number),
        Literal::String(value) => Value::String(value.clone()),
    }
}
fn display_value(value: &Value) -> String {
    if let Some(string) = value.as_str() {
        return string.to_owned();
    }
    if let Some(number) = value.as_f64() {
        if number.fract() == 0.0 && number >= i64::MIN as f64 && number <= i64::MAX as f64 {
            return format!("{}", number as i64);
        }
    }
    value.to_string()
}
fn unary(operator: &str, value: Value) -> Result<Value, ExecutionError> {
    match operator {
        "!" => value
            .as_bool()
            .map(|value| Value::Bool(!value))
            .ok_or_else(|| simple_error("TypeError", "! expects a boolean")),
        "-" => {
            if let Some(integer) = value.as_i64() {
                integer
                    .checked_neg()
                    .map(|value| json!(value))
                    .ok_or_else(|| simple_error("NumericOverflow", "integer negation overflowed"))
            } else {
                number(value).and_then(|value| finite_number(-value))
            }
        }
        _ => Err(simple_error(
            "InvalidIR",
            format!("unknown unary operator `{operator}`"),
        )),
    }
}
fn binary(operator: &str, left: Value, right: Value) -> Result<Value, ExecutionError> {
    if operator == "and" || operator == "&&" || operator == "or" || operator == "||" {
        let (Some(left), Some(right)) = (left.as_bool(), right.as_bool()) else {
            return Err(simple_error(
                "TypeError",
                "boolean operators require booleans",
            ));
        };
        return Ok(Value::Bool(if operator == "and" || operator == "&&" {
            left && right
        } else {
            left || right
        }));
    }
    match operator {
        "==" => Ok(Value::Bool(if left.is_number() && right.is_number() {
            if let (Some(left), Some(right)) = (left.as_i64(), right.as_i64()) {
                left == right
            } else {
                left.as_f64() == right.as_f64()
            }
        } else {
            left == right
        })),
        "!=" => {
            binary("==", left, right).map(|value| Value::Bool(!value.as_bool().unwrap_or(false)))
        }
        "+" if left.is_string() && right.is_string() => Ok(Value::String(format!(
            "{}{}",
            left.as_str().unwrap_or_default(),
            right.as_str().unwrap_or_default()
        ))),
        "+" | "-" | "*" | "/" | "%"
            if left.as_i64().is_some() && right.as_i64().is_some() && operator != "/" =>
        {
            let left = left.as_i64().expect("guarded integer");
            let right = right.as_i64().expect("guarded integer");
            let result = match operator {
                "+" => left.checked_add(right),
                "-" => left.checked_sub(right),
                "*" => left.checked_mul(right),
                "%" => left.checked_rem(right),
                _ => None,
            };
            result.map(|value| json!(value)).ok_or_else(|| {
                simple_error(
                    "NumericOverflow",
                    "integer arithmetic overflowed or divided by zero",
                )
            })
        }
        "+" | "-" | "*" | "/" | "%" => {
            let l = number(left)?;
            let r = number(right)?;
            if (operator == "/" || operator == "%") && r == 0.0 {
                return Err(simple_error("NumericError", "division by zero"));
            }
            let value = match operator {
                "+" => l + r,
                "-" => l - r,
                "*" => l * r,
                "/" => l / r,
                "%" => l % r,
                _ => unreachable!(),
            };
            finite_number(value)
        }
        "<" | ">" | "<=" | ">=" => {
            let order = if let (Some(l), Some(r)) = (left.as_f64(), right.as_f64()) {
                l.partial_cmp(&r)
            } else if let (Some(l), Some(r)) = (left.as_str(), right.as_str()) {
                Some(l.cmp(r))
            } else {
                return Err(simple_error(
                    "TypeError",
                    "ordering requires two numbers or two strings",
                ));
            };
            let Some(order) = order else {
                return Err(simple_error("TypeError", "values are not orderable"));
            };
            Ok(Value::Bool(match operator {
                "<" => order.is_lt(),
                ">" => order.is_gt(),
                "<=" => !order.is_gt(),
                ">=" => !order.is_lt(),
                _ => false,
            }))
        }
        _ => Err(simple_error(
            "InvalidIR",
            format!("unknown binary operator `{operator}`"),
        )),
    }
}
fn number(value: Value) -> Result<f64, ExecutionError> {
    value
        .as_f64()
        .ok_or_else(|| simple_error("TypeError", "arithmetic expects numbers"))
}
fn finite_number(value: f64) -> Result<Value, ExecutionError> {
    Number::from_f64(value)
        .map(Value::Number)
        .ok_or_else(|| simple_error("NumericOverflow", "arithmetic produced a non-finite number"))
}
fn index_value(object: Value, index: Value) -> Result<Value, (&'static str, String)> {
    match object {
        Value::Array(values) => {
            let index = index.as_u64().ok_or_else(|| {
                (
                    "TypeError",
                    "array index must be a non-negative integer".to_owned(),
                )
            })? as usize;
            values.get(index).cloned().ok_or_else(|| {
                (
                    "IndexOutOfBounds",
                    format!("array index {index} is out of bounds"),
                )
            })
        }
        Value::Object(values) => {
            let key = index
                .as_str()
                .ok_or_else(|| ("TypeError", "object index must be a string".to_owned()))?;
            values
                .get(key)
                .cloned()
                .ok_or_else(|| ("MissingKey", format!("missing object key `{key}`")))
        }
        Value::String(value) => {
            let index = index.as_u64().ok_or_else(|| {
                (
                    "TypeError",
                    "string index must be a non-negative integer".to_owned(),
                )
            })? as usize;
            value
                .chars()
                .nth(index)
                .map(|ch| Value::String(ch.to_string()))
                .ok_or_else(|| {
                    (
                        "IndexOutOfBounds",
                        format!("string index {index} is out of bounds"),
                    )
                })
        }
        _ => Err(("TypeError", "value cannot be indexed".to_owned())),
    }
}
fn simple_error(kind: impl Into<String>, message: impl Into<String>) -> ExecutionError {
    ExecutionError {
        kind: kind.into().into_boxed_str(),
        message: message.into().into_boxed_str(),
        procedure: None,
        block: None,
        instruction: None,
        source_span: None,
        trace: None,
    }
}

#[cfg(test)]
mod tests {
    use std::cell::Cell;
    use std::collections::BTreeSet;

    use serde_json::{Value, json};
    use silk_protocol::{Effect, Grant, HostFunctionDescriptor};
    use silk_syntax::lower;

    use super::{ExecutionError, HostFunctionProvider, Session, TraceEventKind};

    struct FakeHost {
        calls: Cell<usize>,
    }
    impl HostFunctionProvider for FakeHost {
        fn call(&mut self, _function: &str, arguments: &[Value]) -> Result<Value, String> {
            self.calls.set(self.calls.get() + 1);
            Ok(arguments.first().cloned().unwrap_or(Value::Null))
        }
    }

    #[test]
    fn evaluates_values_calls_and_control_flow() {
        let program=lower("fn sum(values) { state total = 0; for item in values { total = total + item; } if total > 5 { return total; } else { return 0; } }").expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let result = session
            .execute(&program, "sum", vec![json!([2, 4])], &[], &[], &mut host)
            .expect("evaluated");
        assert_eq!(result.value, json!(6));
        assert!(
            result
                .trace
                .iter()
                .any(|event| event.event == TraceEventKind::Branch)
        );
    }

    #[test]
    fn boolean_operators_short_circuit() {
        let program = lower("fn tick() { state calls = 0; calls = calls + 1; return calls; } fn run() { let result = false and tick(); return result; }").expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let result = session
            .execute(&program, "run", vec![], &[], &[], &mut host)
            .expect("short-circuited");
        assert_eq!(result.value, Value::Bool(false));
        let calls = session
            .execute(&program, "tick", vec![], &[], &[], &mut host)
            .expect("read state");
        assert_eq!(calls.value.as_f64(), Some(1.0));
    }

    #[test]
    fn keeps_session_state_between_calls() {
        let program = lower("fn tick() { state count = 0; count = count + 1; return count; }")
            .expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        assert_eq!(
            session
                .execute(&program, "tick", vec![], &[], &[], &mut host)
                .expect("first run")
                .value,
            json!(1)
        );
        assert_eq!(
            session
                .execute(&program, "tick", vec![], &[], &[], &mut host)
                .expect("second run")
                .value,
            json!(2)
        );
    }

    #[test]
    fn emits_output_and_returns_json_value() {
        let program =
            lower("fn run() { print(\"hello\", 3); return {answer: 42}; }").expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let result = session
            .execute(&program, "run", vec![], &[], &[], &mut host)
            .expect("evaluated");
        assert_eq!(result.output, vec!["hello 3"]);
        assert_eq!(result.value, json!({"answer":42}));
        assert_eq!(
            result.trace.first().expect("entry trace").event,
            TraceEventKind::ProcedureEnter
        );
        assert_eq!(
            result.trace.last().expect("exit trace").event,
            TraceEventKind::ProcedureExit
        );
        let encoded: Value = serde_json::from_str(&result.to_json()).expect("valid JSON output");
        assert_eq!(encoded["output"], json!(["hello 3"]));
        assert_eq!(encoded["value"], json!({"answer":42}));
        assert_eq!(
            encoded["trace"].as_array().expect("trace array").len(),
            result.trace.len()
        );
    }

    #[test]
    fn authorizes_host_call_before_invoking_provider() {
        let program = lower("fn run() { return catalog.read(7); }").expect("lowered");
        let catalog = [HostFunctionDescriptor {
            name: "catalog.read".to_owned(),
            authority: "catalog.read".to_owned(),
            effects: BTreeSet::from([Effect::HostRead]),
        }];
        let grants = [Grant {
            authority: "catalog.read".to_owned(),
            effect_ceiling: BTreeSet::from([Effect::HostRead]),
        }];
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let mut session = Session::default();
        let value = session
            .execute(&program, "run", vec![], &catalog, &grants, &mut host)
            .expect("authorized");
        assert_eq!(value.value, json!(7));
        assert_eq!(host.calls.get(), 1);
        let event_kinds: Vec<_> = value.trace.iter().map(|event| event.event).collect();
        assert!(event_kinds.windows(3).any(|events| events
            == [
                TraceEventKind::HostCallAttempt,
                TraceEventKind::HostCallAuthorized,
                TraceEventKind::HostCallCompleted
            ]));
        let denied = session
            .execute(&program, "run", vec![], &catalog, &[], &mut host)
            .expect_err("grant is required");
        assert_eq!(denied.kind.as_ref(), "AuthorityDenied");
        assert_eq!(host.calls.get(), 1);
        let denied_trace = denied.trace.expect("denial events");
        assert!(
            denied_trace
                .iter()
                .any(|event| event.event == TraceEventKind::HostCallDenied)
        );
        assert!(
            !denied_trace
                .iter()
                .any(|event| event.event == TraceEventKind::HostCallAuthorized)
        );
    }

    #[test]
    fn reports_structured_runtime_errors_and_source_spans() {
        let program = lower("fn bad() { return 1 + true; }").expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let error: ExecutionError = session
            .execute(&program, "bad", vec![], &[], &[], &mut host)
            .expect_err("type error");
        assert_eq!(error.kind.as_ref(), "TypeError");
        assert_eq!(error.procedure.as_deref(), Some("bad"));
        assert!(error.source_span.is_some());
    }

    #[test]
    fn enforces_instruction_fuel() {
        let program = lower("fn spin() { while true { } }").expect("lowered");
        let mut session = Session::default().with_limits(12, 8);
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let error = session
            .execute(&program, "spin", vec![], &[], &[], &mut host)
            .expect_err("fuel exhausted");
        assert_eq!(error.kind.as_ref(), "FuelExhausted");
    }

    #[test]
    fn detects_integer_overflow() {
        let program = lower("fn overflow() { return 9223372036854775807 + 1; }").expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let error = session
            .execute(&program, "overflow", vec![], &[], &[], &mut host)
            .expect_err("overflow");
        assert_eq!(error.kind.as_ref(), "NumericOverflow");
        assert!(
            error
                .trace
                .expect("failure trace")
                .iter()
                .any(|event| event.event == TraceEventKind::Error)
        );
    }

    #[test]
    fn nested_calls_have_correlation_ids() {
        let program =
            lower("fn inner(value) { return value + 1; } fn outer(value) { return inner(value); }")
                .expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let result = session
            .execute(&program, "outer", vec![json!(1)], &[], &[], &mut host)
            .expect("executed");
        let inner_entry = result
            .trace
            .iter()
            .find(|event| {
                event.event == TraceEventKind::ProcedureEnter && event.procedure == "inner"
            })
            .expect("nested entry");
        assert_eq!(inner_entry.parent_correlation_id.as_deref(), Some("call-0"));
        assert_eq!(inner_entry.correlation_id, "call-1");
    }

    #[test]
    fn missing_host_descriptor_has_distinct_error() {
        let program = lower("fn run() { return catalog.read(); }").expect("lowered");
        let mut session = Session::default();
        let mut host = FakeHost {
            calls: Cell::new(0),
        };
        let error = session
            .execute(&program, "run", vec![], &[], &[], &mut host)
            .expect_err("missing descriptor");
        assert_eq!(error.kind.as_ref(), "UnknownHostFunction");
        assert_eq!(host.calls.get(), 0);
    }
}
