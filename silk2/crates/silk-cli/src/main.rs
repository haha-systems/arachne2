use std::env;
use std::fs;
use std::process::ExitCode;

use sha2::{Digest, Sha256};
use silk_corpus::{ExecutionSnapshot, compare_snapshots};
use silk_ir::{CallTarget, Operation, ProgramIR};
use silk_runtime::Runtime;
use silk_syntax::{lower, parse};
use std::collections::{BTreeMap, BTreeSet};

mod server;

fn main() -> ExitCode {
    let mut arguments = env::args().skip(1);
    if arguments.next().as_deref() == Some("serve") {
        return match server::serve() {
            Ok(()) => ExitCode::SUCCESS,
            Err(error) => {
                eprintln!("Silk protocol server failed: {error}");
                ExitCode::FAILURE
            }
        };
    }
    match run(env::args().skip(1)) {
        Ok((message, exit_code)) => {
            println!("{message}");
            ExitCode::from(exit_code)
        }
        Err(error) => {
            eprintln!("{error}");
            ExitCode::from(error.exit_code)
        }
    }
}

struct CliError {
    exit_code: u8,
    message: String,
}

impl std::fmt::Display for CliError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.message)
    }
}

fn run(mut arguments: impl Iterator<Item = String>) -> Result<(String, u8), CliError> {
    match arguments.next().as_deref() {
        Some("run") => run_source(arguments),
        Some("serve") => Err(CliError {
            exit_code: 2,
            message: "usage: silk serve (length-prefixed SRP 1.0 over stdio)".to_owned(),
        }),
        Some("inspect") => run_inspect(arguments),
        Some("compare") => run_compare(arguments),
        Some("capture") => run_capture(arguments),
        _ => Err(CliError {
            exit_code: 2,
            message: "usage: silk run <path> | silk inspect <source.silk|snapshot.json> | silk capture <source.silk> <snapshot.json> | silk compare <reference.json> <candidate.json>".to_owned(),
        }),
    }
}

fn run_inspect(mut arguments: impl Iterator<Item = String>) -> Result<(String, u8), CliError> {
    let usage = "usage: silk inspect <source.silk|snapshot.json>";
    let Some(path) = arguments.next() else {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    };
    if arguments.next().is_some() {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    }
    let bytes = fs::read(&path).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot read inspection input {path}: {error}"),
    })?;
    let report = if path.ends_with(".json") {
        let value: serde_json::Value =
            serde_json::from_slice(&bytes).map_err(|error| CliError {
                exit_code: 1,
                message: format!("cannot decode inspection input {path}: {error}"),
            })?;
        if value
            .get("schema_version")
            .and_then(serde_json::Value::as_str)
            == Some(silk_corpus::COMPARISON_SCHEMA_VERSION)
        {
            inspect_snapshot(value)?
        } else {
            return Err(CliError {
                exit_code: 1,
                message: format!("unsupported inspection JSON schema in {path}"),
            });
        }
    } else {
        let source = std::str::from_utf8(&bytes).map_err(|error| CliError {
            exit_code: 1,
            message: format!("Silk source {path} is not UTF-8: {error}"),
        })?;
        let ir = lower(source).map_err(|error| CliError {
            exit_code: 1,
            message: format!("cannot inspect Silk source {path}: {error}"),
        })?;
        inspect_program(&ir, &bytes)
    };
    serde_json::to_string_pretty(&report)
        .map(|json| (json, 0))
        .map_err(|error| CliError {
            exit_code: 1,
            message: format!("cannot serialize inspection report: {error}"),
        })
}

fn inspect_program(program: &ProgramIR, source: &[u8]) -> serde_json::Value {
    let procedures = program
        .procedures
        .iter()
        .map(|(id, procedure)| {
            let mut effects = BTreeSet::new();
            let mut host_dependencies = BTreeSet::new();
            let mut calls = BTreeSet::new();
            let mut instruction_count = 0;
            let blocks = procedure
                .blocks
                .iter()
                .map(|(block_id, block)| {
                    (
                        block_id.clone(),
                        serde_json::json!({
                            "instructions": block.instructions.iter().map(|instruction| {
                                serde_json::json!({"id": instruction.id, "operation": instruction.op})
                            }).collect::<Vec<_>>(),
                            "terminator": block.terminator
                        }),
                    )
                })
                .collect::<BTreeMap<_, _>>();
            for block in procedure.blocks.values() {
                instruction_count += block.instructions.len();
                for instruction in &block.instructions {
                    match &instruction.op {
                        Operation::LoadState { .. } => {
                            effects.insert("session_state_read");
                        }
                        Operation::InitState { .. } | Operation::StoreState { .. } => {
                            effects.insert("session_state_write");
                        }
                        Operation::Call { callee, target, .. } => match target {
                            CallTarget::Procedure => {
                                calls.insert(callee.clone());
                            }
                            CallTarget::HostFunction => {
                                host_dependencies.insert(callee.clone());
                                effects.insert("unknown");
                            }
                            CallTarget::Dynamic => {
                                effects.insert("unknown");
                            }
                            CallTarget::Core => match callee.as_str() {
                                "print" => {
                                    effects.insert("process_output");
                                }
                                "now" => {
                                    effects.insert("clock");
                                }
                                "sleep" => {
                                    effects.insert("delay");
                                }
                                _ => {}
                            },
                        },
                        _ => {}
                    }
                }
            }
            (
                id.clone(),
                serde_json::json!({
                    "name": procedure.name,
                    "parameters": procedure.parameters,
                    "entry_block": procedure.entry_block,
                    "blocks": procedure.blocks.len(),
                    "instruction_count": instruction_count,
                    "semantic_blocks": blocks,
                    "direct_effects": effects,
                    "procedure_dependencies": calls,
                    "host_dependencies": host_dependencies,
                    "authority_requirements": "unresolved_without_host_catalog",
                    "return_behavior": "see semantic IR terminators"
                }),
            )
        })
        .collect::<BTreeMap<_, _>>();
    serde_json::json!({
        "schema_version": "silk.inspection.v1",
        "kind": "static_program",
        "program_id": program.program_id,
        "source_sha256": format!("{:x}", Sha256::digest(source)),
        "purpose": "not declared by this source subset",
        "procedures": procedures,
        "limitations": [
            "effects are direct; transitive procedure calls are listed but not expanded",
            "host authority requirements need the active host descriptor catalog",
            "semantic prose and contracts are not represented by the current IR subset"
        ]
    })
}

fn inspect_snapshot(snapshot: serde_json::Value) -> Result<serde_json::Value, CliError> {
    let snapshot: ExecutionSnapshot =
        serde_json::from_value(snapshot).map_err(|error| CliError {
            exit_code: 1,
            message: format!("invalid execution snapshot: {error}"),
        })?;
    let mut event_counts = BTreeMap::<String, usize>::new();
    let events = snapshot
        .trace
        .iter()
        .map(|event| {
            let label = event
                .get("event")
                .and_then(|value| {
                    value
                        .as_str()
                        .or_else(|| value.get("kind").and_then(serde_json::Value::as_str))
                })
                .or_else(|| event.get("kind").and_then(serde_json::Value::as_str))
                .unwrap_or("unknown")
                .to_owned();
            *event_counts.entry(label).or_default() += 1;
            event.clone()
        })
        .collect::<Vec<_>>();
    Ok(serde_json::json!({
        "schema_version": "silk.inspection.v1",
        "kind": "execution_history",
        "runtime": snapshot.runtime,
        "source_sha256": snapshot.source_sha256,
        "exit_code": snapshot.exit_code,
        "timed_out": snapshot.timed_out,
        "stdout": snapshot.stdout,
        "output": snapshot.output,
        "result": snapshot.result,
        "host_replay": snapshot.host_replay,
        "event_count": events.len(),
        "event_counts": event_counts,
        "trace": events
    }))
}

fn run_source(mut arguments: impl Iterator<Item = String>) -> Result<(String, u8), CliError> {
    let usage = "usage: silk run <path>";
    let Some(path) = arguments.next() else {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    };
    if arguments.next().is_some() {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    }

    let source = fs::read_to_string(&path).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot read Silk source {path}: {error}"),
    })?;
    let program = parse(&source).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot load Silk source {path}: {error}"),
    })?;

    Runtime
        .execute(&program)
        .map(|result| (result.to_json(), 0))
        .map_err(|error| CliError {
            exit_code: 1,
            message: format!("Silk execution failed: {error}"),
        })
}

fn run_compare(mut arguments: impl Iterator<Item = String>) -> Result<(String, u8), CliError> {
    let usage = "usage: silk compare <reference.json> <candidate.json>";
    let (Some(reference_path), Some(candidate_path)) = (arguments.next(), arguments.next()) else {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    };
    if arguments.next().is_some() {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    }
    let reference = load_snapshot(&reference_path)?;
    let candidate = load_snapshot(&candidate_path)?;
    let report = compare_snapshots(&reference, &candidate);
    let exit_code = if report.matched { 0 } else { 1 };
    serde_json::to_string(&report)
        .map(|json| (json, exit_code))
        .map_err(|error| CliError {
            exit_code: 1,
            message: format!("cannot serialize comparison report: {error}"),
        })
}

fn load_snapshot(path: &str) -> Result<ExecutionSnapshot, CliError> {
    let bytes = fs::read(path).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot read execution snapshot {path}: {error}"),
    })?;
    serde_json::from_slice(&bytes).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot decode execution snapshot {path}: {error}"),
    })
}

fn run_capture(mut arguments: impl Iterator<Item = String>) -> Result<(String, u8), CliError> {
    let usage = "usage: silk capture <source.silk> <snapshot.json>";
    let (Some(source_path), Some(snapshot_path)) = (arguments.next(), arguments.next()) else {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    };
    if arguments.next().is_some() {
        return Err(CliError {
            exit_code: 2,
            message: usage.to_owned(),
        });
    }
    let source = fs::read(&source_path).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot read Silk source {source_path}: {error}"),
    })?;
    let source_text = std::str::from_utf8(&source).map_err(|error| CliError {
        exit_code: 1,
        message: format!("Silk source {source_path} is not UTF-8: {error}"),
    })?;
    let program = parse(source_text).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot load Silk source {source_path}: {error}"),
    })?;
    let result = Runtime.execute(&program).map_err(|error| CliError {
        exit_code: 1,
        message: format!("Silk execution failed: {error}"),
    })?;
    let digest = format!("{:x}", Sha256::digest(&source));
    let snapshot = ExecutionSnapshot {
        schema_version: silk_corpus::COMPARISON_SCHEMA_VERSION.to_owned(),
        source_sha256: digest,
        runtime: format!("silk2@{}", env!("CARGO_PKG_VERSION")),
        exit_code: Some(0),
        timed_out: false,
        stdout: result.to_json(),
        output: result.output.join("\n"),
        result: Some(result.value),
        host_replay: None,
        trace: result
            .trace
            .iter()
            .map(serde_json::to_value)
            .collect::<Result<_, _>>()
            .expect("trace serializes"),
    };
    let bytes = serde_json::to_vec_pretty(&snapshot).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot encode snapshot: {error}"),
    })?;
    fs::write(&snapshot_path, bytes).map_err(|error| CliError {
        exit_code: 1,
        message: format!("cannot write execution snapshot {snapshot_path}: {error}"),
    })?;
    Ok((format!("captured {source_path} as {snapshot_path}"), 0))
}
