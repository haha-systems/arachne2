//! Test-only loader for versioned Silk reference recordings.

mod comparison;
pub use comparison::{
    COMPARISON_SCHEMA_VERSION, ComparisonReport, ExecutionSnapshot, compare_snapshots,
};

use std::collections::BTreeMap;
use std::error::Error;
use std::fmt::{Display, Formatter};
use std::fs;
use std::path::{Component, Path, PathBuf};

use serde::Deserialize;
use serde_json::Value;

/// Manifest envelope for the Stage 0 corpus.
#[derive(Clone, Debug, Deserialize)]
pub struct Manifest {
    /// Manifest schema identifier.
    pub schema_version: String,
    /// Number of indexed programs.
    pub recorded_programs: usize,
    /// Exit-code summary retained as generic JSON because it includes `timeout`.
    pub exit_code_counts: BTreeMap<String, Value>,
    /// Total builtin trace records across entries.
    pub total_trace_records: usize,
    /// Number of programs with at least one trace record.
    pub files_with_trace_records: usize,
    /// Number of entries with incomplete trace sequence numbers.
    pub files_with_incomplete_sequence: usize,
    /// Recording index.
    pub entries: Vec<ManifestEntry>,
}

/// One source and its recorded result.
#[derive(Clone, Debug, Deserialize)]
pub struct ManifestEntry {
    /// Original repository-relative program path.
    pub program: String,
    /// Recording path relative to the corpus root.
    pub fixture: String,
    /// Source digest recorded by Stage 0.
    pub source_sha256: String,
    /// Exit status, absent for timed-out programs.
    pub exit_code: Option<i32>,
    /// Whether the reference process hit its timeout.
    pub timed_out: bool,
    /// Recorded wall time in milliseconds.
    pub duration_ms: f64,
    /// Number of builtin trace events.
    pub trace_record_count: usize,
    /// Whether the sequence set is complete.
    pub trace_sequence_complete: bool,
}

/// One reference recording envelope.
#[derive(Clone, Debug, Deserialize)]
pub struct Recording {
    /// Recording schema identifier.
    pub schema_version: String,
    /// Original program path.
    pub program: String,
    /// Source digest recorded by Stage 0.
    pub source_sha256: String,
    /// Invocation metadata.
    pub invocation: Value,
    /// Reference execution result.
    pub result: RecordingResult,
    /// Captured standard output.
    pub stdout: String,
    /// Program output with trace records removed.
    pub output: String,
    /// Ordered trace records.
    pub trace: Vec<Value>,
    /// Whether the trace sequence set is complete.
    pub trace_sequence_complete: bool,
}

/// Result metadata in a recording.
#[derive(Clone, Debug, Deserialize)]
pub struct RecordingResult {
    /// Exit status, absent for timed-out programs.
    pub exit_code: Option<i32>,
    /// Whether the reference process hit its timeout.
    pub timed_out: bool,
    /// Recorded wall time in milliseconds.
    pub duration_ms: f64,
}

/// Integrity result for loading a manifest and all referenced recordings.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CorpusReport {
    /// Number of recordings loaded and structurally validated.
    pub loaded: usize,
    /// Programs that passed Silk execution compatibility. Phase 1 is always zero.
    pub passing: usize,
    /// Programs not yet run by a semantic evaluator.
    pub failing: usize,
}

impl Display for CorpusReport {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        write!(
            formatter,
            "Corpus: {}/{} passing ({} failing)",
            self.passing, self.loaded, self.failing
        )
    }
}

/// Corpus load or integrity error.
#[derive(Debug)]
pub struct CorpusError {
    message: String,
}

impl CorpusError {
    fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }
}

impl Display for CorpusError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.message)
    }
}

impl Error for CorpusError {}

/// Loads every manifest entry and validates its recording envelope.
pub fn load_corpus(root: &Path) -> Result<CorpusReport, CorpusError> {
    let manifest_path = root.join("corpus/MANIFEST.json");
    let manifest_bytes = fs::read(&manifest_path).map_err(|error| {
        CorpusError::new(format!("cannot read {}: {error}", manifest_path.display()))
    })?;
    let manifest: Manifest = serde_json::from_slice(&manifest_bytes).map_err(|error| {
        CorpusError::new(format!(
            "cannot decode {}: {error}",
            manifest_path.display()
        ))
    })?;

    if manifest.schema_version != "silk.corpus_manifest.v1" {
        return Err(CorpusError::new(format!(
            "unsupported corpus manifest schema: {}",
            manifest.schema_version
        )));
    }
    if manifest.recorded_programs != manifest.entries.len() {
        return Err(CorpusError::new(format!(
            "manifest count {} does not match entry count {}",
            manifest.recorded_programs,
            manifest.entries.len()
        )));
    }

    for entry in &manifest.entries {
        let fixture_relative = safe_relative_path(&entry.fixture)?;
        let fixture_path = root.join(fixture_relative);
        let bytes = fs::read(&fixture_path).map_err(|error| {
            CorpusError::new(format!("cannot read {}: {error}", fixture_path.display()))
        })?;
        let recording: Recording = serde_json::from_slice(&bytes).map_err(|error| {
            CorpusError::new(format!("cannot decode {}: {error}", fixture_path.display()))
        })?;

        if recording.schema_version != "silk.corpus_recording.v1" {
            return Err(CorpusError::new(format!(
                "unsupported recording schema in {}: {}",
                fixture_path.display(),
                recording.schema_version
            )));
        }
        if recording.program != entry.program
            || recording.source_sha256 != entry.source_sha256
            || recording.result.exit_code != entry.exit_code
            || recording.result.timed_out != entry.timed_out
            || recording.trace.len() != entry.trace_record_count
            || recording.trace_sequence_complete != entry.trace_sequence_complete
        {
            return Err(CorpusError::new(format!(
                "manifest and recording disagree for {}",
                entry.program
            )));
        }
    }

    let loaded = manifest.entries.len();
    Ok(CorpusReport {
        loaded,
        passing: 0,
        failing: loaded,
    })
}

fn safe_relative_path(value: &str) -> Result<PathBuf, CorpusError> {
    let path = Path::new(value);
    if path.is_absolute()
        || path
            .components()
            .any(|component| !matches!(component, Component::Normal(_)))
    {
        return Err(CorpusError::new(format!(
            "fixture path is not a normalized relative path: {value}"
        )));
    }
    Ok(path.to_owned())
}
