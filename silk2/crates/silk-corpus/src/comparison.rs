//! Versioned snapshots and deterministic observable-outcome comparison.

use serde::{Deserialize, Serialize};
use serde_json::Value;

/// Schema identifier for Silk compatibility execution snapshots.
pub const COMPARISON_SCHEMA_VERSION: &str = "silk.execution_snapshot.v1";

/// Observable result captured from one isolated corpus execution.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ExecutionSnapshot {
    /// Snapshot schema identifier.
    pub schema_version: String,
    /// SHA-256 of the exact source bytes executed.
    pub source_sha256: String,
    /// Runtime name and immutable version or commit pin.
    pub runtime: String,
    /// Process exit status, absent when the process timed out.
    pub exit_code: Option<i32>,
    /// Whether the runner hit its configured timeout.
    pub timed_out: bool,
    /// Captured process stdout, excluding diagnostic stderr.
    pub stdout: String,
    /// Program output after removing only documented trace records.
    pub output: String,
    /// Canonical JSON result when the runner exposes one.
    pub result: Option<Value>,
    /// Versioned deterministic host replay tape, when host calls occurred.
    pub host_replay: Option<Value>,
    /// Diagnostic trace; format may differ between runtimes.
    pub trace: Vec<Value>,
}

/// Deterministic comparison of two execution snapshots.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct ComparisonReport {
    /// Comparison schema identifier.
    pub schema_version: String,
    /// Whether every comparable public observable matched.
    pub matched: bool,
    /// Names of observables that matched.
    pub compared: Vec<String>,
    /// Concrete mismatch descriptions.
    pub differences: Vec<String>,
    /// Traces are diagnostic and are not compared across schema versions.
    pub trace_comparison: String,
}

/// Compares public outcomes while keeping diagnostic trace formats separate.
pub fn compare_snapshots(
    reference: &ExecutionSnapshot,
    candidate: &ExecutionSnapshot,
) -> ComparisonReport {
    let mut compared = Vec::new();
    let mut differences = Vec::new();
    if reference.schema_version != COMPARISON_SCHEMA_VERSION
        || candidate.schema_version != COMPARISON_SCHEMA_VERSION
    {
        differences.push(format!(
            "unsupported snapshot schemas {} and {}",
            reference.schema_version, candidate.schema_version
        ));
    }
    if !is_sha256(&reference.source_sha256) || !is_sha256(&candidate.source_sha256) {
        differences.push("source_sha256 must be 64 lowercase hexadecimal characters".to_owned());
    }
    if reference.source_sha256 != candidate.source_sha256 {
        differences.push("source SHA-256 differs".to_owned());
    } else {
        compared.push("source_sha256".to_owned());
    }
    compare_field(
        "exit_code",
        &reference.exit_code,
        &candidate.exit_code,
        &mut compared,
        &mut differences,
    );
    compare_field(
        "timed_out",
        &reference.timed_out,
        &candidate.timed_out,
        &mut compared,
        &mut differences,
    );
    compare_field(
        "stdout",
        &reference.stdout,
        &candidate.stdout,
        &mut compared,
        &mut differences,
    );
    compare_field(
        "output",
        &reference.output,
        &candidate.output,
        &mut compared,
        &mut differences,
    );
    compare_field(
        "result",
        &reference.result,
        &candidate.result,
        &mut compared,
        &mut differences,
    );
    compare_field(
        "host_replay",
        &reference.host_replay,
        &candidate.host_replay,
        &mut compared,
        &mut differences,
    );
    ComparisonReport {
        schema_version: "silk.comparison_report.v1".to_owned(),
        matched: differences.is_empty(),
        compared,
        differences,
        trace_comparison: "not_compared_diagnostic_schema_may_differ".to_owned(),
    }
}

fn is_sha256(value: &str) -> bool {
    value.len() == 64
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

fn compare_field<T: PartialEq + std::fmt::Debug>(
    name: &str,
    reference: &T,
    candidate: &T,
    compared: &mut Vec<String>,
    differences: &mut Vec<String>,
) {
    if reference == candidate {
        compared.push(name.to_owned());
    } else {
        differences.push(format!(
            "{name} differs: reference={reference:?}, candidate={candidate:?}"
        ));
    }
}

#[cfg(test)]
mod tests {
    use super::{COMPARISON_SCHEMA_VERSION, ExecutionSnapshot, compare_snapshots};
    use serde_json::json;

    fn snapshot(runtime: &str, output: &str) -> ExecutionSnapshot {
        ExecutionSnapshot {
            schema_version: COMPARISON_SCHEMA_VERSION.to_owned(),
            source_sha256: "a".repeat(64),
            runtime: runtime.to_owned(),
            exit_code: Some(0),
            timed_out: false,
            stdout: output.to_owned(),
            output: output.to_owned(),
            result: Some(json!({"value": 4})),
            host_replay: None,
            trace: vec![json!({"legacy_trace": true})],
        }
    }

    #[test]
    fn ignores_runtime_labels_and_trace_shape_but_compares_observable_results() {
        let reference = snapshot("zig@pin", "answer=4");
        let mut candidate = snapshot("silk2@pin", "answer=4");
        candidate.trace = vec![json!({"new_trace":true})];
        let report = compare_snapshots(&reference, &candidate);
        assert!(report.matched);
        assert_eq!(
            report.trace_comparison,
            "not_compared_diagnostic_schema_may_differ"
        );
    }

    #[test]
    fn reports_source_output_and_replay_mismatches() {
        let reference = snapshot("zig@pin", "answer=4");
        let mut candidate = snapshot("silk2@pin", "answer=5");
        candidate.source_sha256 = "b".repeat(64);
        candidate.host_replay = Some(json!({"schema_version":"silk.replay.v1"}));
        let report = compare_snapshots(&reference, &candidate);
        assert!(!report.matched);
        assert!(
            report
                .differences
                .iter()
                .any(|difference| difference.contains("source SHA-256"))
        );
        assert!(
            report
                .differences
                .iter()
                .any(|difference| difference.starts_with("output differs"))
        );
        assert!(
            report
                .differences
                .iter()
                .any(|difference| difference.starts_with("host_replay differs"))
        );
    }
}
