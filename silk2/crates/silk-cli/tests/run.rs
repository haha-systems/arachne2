use std::fs;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn temp_source_path() -> std::path::PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("system clock")
        .as_nanos();
    std::env::temp_dir().join(format!("silk-cli-{}-{timestamp}.silk", std::process::id()))
}

#[test]
fn run_command_reads_a_silk_file() {
    let path = temp_source_path();
    fs::write(&path, "let answer = 42\n").expect("write source fixture");

    let output = Command::new(env!("CARGO_BIN_EXE_silk"))
        .args(["run", path.to_str().expect("UTF-8 path")])
        .output()
        .expect("run silk CLI");
    let _ = fs::remove_file(&path);

    assert!(output.status.success());
    let result: serde_json::Value = serde_json::from_slice(&output.stdout).expect("JSON result");
    assert_eq!(result["output"], serde_json::json!([]));
    assert_eq!(result["value"], serde_json::Value::Null);
    assert_eq!(result["trace"][0]["event"], "procedure_enter");
}

#[test]
fn run_command_executes_and_returns_structured_json() {
    let path = temp_source_path();
    fs::write(&path, "let greeting = \"hello\"\nprint(greeting)\n").expect("write source fixture");
    let output = Command::new(env!("CARGO_BIN_EXE_silk"))
        .args(["run", path.to_str().expect("UTF-8 path")])
        .output()
        .expect("run silk CLI");
    let _ = fs::remove_file(&path);

    assert!(output.status.success());
    let result: serde_json::Value = serde_json::from_slice(&output.stdout).expect("JSON result");
    assert_eq!(result["output"], serde_json::json!(["hello"]));
    assert_eq!(result["value"], serde_json::Value::Null);
    assert_eq!(result["trace"][1]["event"], "effect_executed");
}

#[test]
fn run_command_reports_usage_and_missing_files() {
    let binary = env!("CARGO_BIN_EXE_silk");
    let usage = Command::new(binary).arg("run").output().expect("run CLI");
    assert_eq!(usage.status.code(), Some(2));
    assert!(String::from_utf8_lossy(&usage.stderr).contains("usage: silk run <path>"));

    let missing_path = temp_source_path();
    let missing = Command::new(binary)
        .args(["run", missing_path.to_str().expect("UTF-8 path")])
        .output()
        .expect("run CLI");
    assert_eq!(missing.status.code(), Some(1));
    assert!(String::from_utf8_lossy(&missing.stderr).contains("cannot read Silk source"));
}

#[test]
fn compare_command_returns_a_machine_readable_verdict() {
    let reference_path = temp_source_path();
    let candidate_path = temp_source_path().with_extension("candidate.json");
    let reference = silk_corpus::ExecutionSnapshot {
        schema_version: silk_corpus::COMPARISON_SCHEMA_VERSION.to_owned(),
        source_sha256: "a".repeat(64),
        runtime: "zig@pin".to_owned(),
        exit_code: Some(0),
        timed_out: false,
        stdout: "ok".to_owned(),
        output: "ok".to_owned(),
        result: None,
        host_replay: None,
        trace: vec![],
    };
    let mut candidate = reference.clone();
    candidate.runtime = "silk2@pin".to_owned();
    candidate.trace = vec![serde_json::json!({"new_trace":true})];
    fs::write(
        &reference_path,
        serde_json::to_vec(&reference).expect("encode reference"),
    )
    .expect("write reference");
    fs::write(
        &candidate_path,
        serde_json::to_vec(&candidate).expect("encode candidate"),
    )
    .expect("write candidate");
    let output = Command::new(env!("CARGO_BIN_EXE_silk"))
        .args([
            "compare",
            reference_path.to_str().expect("reference path"),
            candidate_path.to_str().expect("candidate path"),
        ])
        .output()
        .expect("run comparison");
    assert_eq!(output.status.code(), Some(0));
    let report: serde_json::Value =
        serde_json::from_slice(&output.stdout).expect("comparison report");
    assert_eq!(report["matched"], true);
    candidate.output = "different".to_owned();
    fs::write(
        &candidate_path,
        serde_json::to_vec(&candidate).expect("encode mismatch"),
    )
    .expect("write mismatch");
    let mismatch = Command::new(env!("CARGO_BIN_EXE_silk"))
        .args([
            "compare",
            reference_path.to_str().expect("reference path"),
            candidate_path.to_str().expect("candidate path"),
        ])
        .output()
        .expect("run mismatch");
    assert_eq!(mismatch.status.code(), Some(1));
    let _ = fs::remove_file(reference_path);
    let _ = fs::remove_file(candidate_path);
}

#[test]
fn capture_command_writes_versioned_source_hashed_snapshot() {
    let source_path = temp_source_path();
    let snapshot_path = source_path.with_extension("snapshot.json");
    fs::write(&source_path, "print(\"captured\")\n").expect("write source");
    let output = Command::new(env!("CARGO_BIN_EXE_silk"))
        .args([
            "capture",
            source_path.to_str().expect("source path"),
            snapshot_path.to_str().expect("snapshot path"),
        ])
        .output()
        .expect("run capture");
    assert!(output.status.success());
    let snapshot: serde_json::Value =
        serde_json::from_slice(&fs::read(&snapshot_path).expect("read snapshot"))
            .expect("decode snapshot");
    assert_eq!(
        snapshot["schema_version"],
        silk_corpus::COMPARISON_SCHEMA_VERSION
    );
    assert_eq!(
        snapshot["source_sha256"]
            .as_str()
            .expect("source hash")
            .len(),
        64
    );
    assert_eq!(snapshot["output"], "captured");
    let _ = fs::remove_file(source_path);
    let _ = fs::remove_file(snapshot_path);
}
