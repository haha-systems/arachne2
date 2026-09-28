//! Versioned deterministic host-call recording and replay.

use std::error::Error;
use std::fmt::{Display, Formatter};

use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::HostFunctionProvider;

/// Current host-call replay tape schema.
pub const REPLAY_SCHEMA_VERSION: &str = "silk.replay.v1";

/// Ordered host outcomes captured from one session execution.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ReplayTape {
    /// Exact replay schema identifier.
    pub schema_version: String,
    /// Host invocations in dispatch sequence order.
    pub entries: Vec<ReplayEntry>,
}

/// One recorded host invocation and its outcome.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ReplayEntry {
    /// Zero-based call sequence within the tape.
    pub sequence: u64,
    /// Exact declared host function name.
    pub function: String,
    /// Arguments at the host boundary.
    pub arguments: Vec<Value>,
    /// Successful result, mutually exclusive with `error`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<Value>,
    /// Host error text, mutually exclusive with `result`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

/// Deterministic host provider that consumes an ordered replay tape.
#[derive(Debug)]
pub struct ReplayProvider {
    tape: ReplayTape,
    cursor: usize,
}

impl ReplayProvider {
    /// Creates a replay provider after validating schema, sequence, and outcomes.
    pub fn new(tape: ReplayTape) -> Result<Self, ReplayError> {
        validate_tape(&tape)?;
        Ok(Self { tape, cursor: 0 })
    }

    /// Returns an error if an execution did not consume the complete fixture.
    pub fn verify_consumed(&self) -> Result<(), ReplayError> {
        if self.cursor == self.tape.entries.len() {
            Ok(())
        } else {
            Err(ReplayError::new(format!(
                "replay ended after {} of {} host calls",
                self.cursor,
                self.tape.entries.len()
            )))
        }
    }

    /// Number of calls consumed so far.
    #[must_use]
    pub const fn consumed(&self) -> usize {
        self.cursor
    }
}

impl HostFunctionProvider for ReplayProvider {
    fn call(&mut self, function: &str, arguments: &[Value]) -> Result<Value, String> {
        let Some(entry) = self.tape.entries.get(self.cursor) else {
            return Err(format!("replay tape exhausted before `{function}`"));
        };
        if entry.function != function {
            return Err(format!(
                "replay call {} expected `{}`, got `{function}`",
                entry.sequence, entry.function
            ));
        }
        if entry.arguments != arguments {
            return Err(format!(
                "arguments differ from replay call {} for `{function}`",
                entry.sequence
            ));
        }
        self.cursor += 1;
        if let Some(error) = &entry.error {
            Err(error.clone())
        } else {
            Ok(entry.result.clone().expect("validated replay outcome"))
        }
    }
}

/// Provider wrapper that records exact calls and returned outcomes.
pub struct RecordingProvider<P> {
    inner: P,
    entries: Vec<ReplayEntry>,
}

impl<P> RecordingProvider<P> {
    /// Wraps a live or deterministic host provider.
    pub const fn new(inner: P) -> Self {
        Self {
            inner,
            entries: Vec::new(),
        }
    }

    /// Returns the wrapped provider after recording is complete.
    pub fn into_inner(self) -> P {
        self.inner
    }

    /// Finalizes the calls recorded so far as a versioned replay tape.
    pub fn into_tape(self) -> ReplayTape {
        ReplayTape {
            schema_version: REPLAY_SCHEMA_VERSION.to_owned(),
            entries: self.entries,
        }
    }
}

impl<P: HostFunctionProvider> HostFunctionProvider for RecordingProvider<P> {
    fn call(&mut self, function: &str, arguments: &[Value]) -> Result<Value, String> {
        let sequence = self.entries.len() as u64;
        let outcome = self.inner.call(function, arguments);
        let (result, error) = match &outcome {
            Ok(value) => (Some(value.clone()), None),
            Err(message) => (None, Some(message.clone())),
        };
        self.entries.push(ReplayEntry {
            sequence,
            function: function.to_owned(),
            arguments: arguments.to_vec(),
            result,
            error,
        });
        outcome
    }
}

/// Malformed tape or replay mismatch.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReplayError {
    message: String,
}

impl ReplayError {
    fn new(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
        }
    }
}

impl Display for ReplayError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(&self.message)
    }
}
impl Error for ReplayError {}

fn validate_tape(tape: &ReplayTape) -> Result<(), ReplayError> {
    if tape.schema_version != REPLAY_SCHEMA_VERSION {
        return Err(ReplayError::new(format!(
            "unsupported replay schema `{}`",
            tape.schema_version
        )));
    }
    for (index, entry) in tape.entries.iter().enumerate() {
        if entry.sequence != index as u64 {
            return Err(ReplayError::new(format!(
                "replay sequence must be contiguous; entry {index} has sequence {}",
                entry.sequence
            )));
        }
        if entry.function.is_empty() {
            return Err(ReplayError::new(format!(
                "replay entry {index} has an empty function name"
            )));
        }
        if entry.result.is_some() == entry.error.is_some() {
            return Err(ReplayError::new(format!(
                "replay entry {index} must contain exactly one of result or error"
            )));
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use serde_json::{Value, json};

    use crate::HostFunctionProvider;

    use super::{
        REPLAY_SCHEMA_VERSION, RecordingProvider, ReplayEntry, ReplayProvider, ReplayTape,
    };

    struct FixedHost;
    impl HostFunctionProvider for FixedHost {
        fn call(&mut self, function: &str, arguments: &[Value]) -> Result<Value, String> {
            if function == "lookup" {
                Ok(json!({"value":arguments[0]}))
            } else {
                Err("offline".to_owned())
            }
        }
    }

    #[test]
    fn recording_round_trips_and_replay_is_repeatable() {
        let mut recording = RecordingProvider::new(FixedHost);
        assert_eq!(
            recording.call("lookup", &[json!(4)]),
            Ok(json!({"value":4}))
        );
        assert_eq!(recording.call("offline", &[]), Err("offline".to_owned()));
        let tape = recording.into_tape();
        let bytes = serde_json::to_vec(&tape).expect("encode tape");
        let decoded: ReplayTape = serde_json::from_slice(&bytes).expect("decode tape");
        assert_eq!(decoded, tape);
        let mut replay = ReplayProvider::new(decoded).expect("valid replay");
        assert_eq!(replay.call("lookup", &[json!(4)]), Ok(json!({"value":4})));
        assert_eq!(replay.call("offline", &[]), Err("offline".to_owned()));
        replay.verify_consumed().expect("all events consumed");
    }

    #[test]
    fn mismatched_call_does_not_advance_cursor() {
        let tape = ReplayTape {
            schema_version: REPLAY_SCHEMA_VERSION.to_owned(),
            entries: vec![ReplayEntry {
                sequence: 0,
                function: "lookup".to_owned(),
                arguments: vec![json!(4)],
                result: Some(json!(4)),
                error: None,
            }],
        };
        let mut replay = ReplayProvider::new(tape).expect("valid replay");
        assert!(
            replay
                .call("lookup", &[json!(5)])
                .unwrap_err()
                .contains("arguments differ")
        );
        assert_eq!(replay.consumed(), 0);
        assert_eq!(replay.call("lookup", &[json!(4)]), Ok(json!(4)));
        replay.verify_consumed().expect("consumed");
    }

    #[test]
    fn rejects_invalid_sequences_and_ambiguous_results() {
        let tape = ReplayTape {
            schema_version: REPLAY_SCHEMA_VERSION.to_owned(),
            entries: vec![ReplayEntry {
                sequence: 2,
                function: "lookup".to_owned(),
                arguments: vec![],
                result: Some(Value::Null),
                error: None,
            }],
        };
        assert!(
            ReplayProvider::new(tape)
                .unwrap_err()
                .to_string()
                .contains("contiguous")
        );
    }
}
