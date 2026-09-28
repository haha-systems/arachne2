//! Versioned protocol data types. Transport and authority checks live elsewhere.

use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeSet;

/// The initial Silk Runtime Protocol version.
pub const PROTOCOL_VERSION: &str = "1.1";

/// A stable effect label used in function descriptors and session grants.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Effect {
    /// Reads session-local mutable bindings.
    SessionStateRead,
    /// Changes session-local mutable bindings.
    SessionStateWrite,
    /// Emits to a runtime-controlled output channel.
    ProcessOutput,
    /// Reads nondeterministic time.
    Clock,
    /// Suspends execution.
    Delay,
    /// Reads information from an external host service.
    HostRead,
    /// Requests a host-owned state change.
    HostWrite,
    /// Requests an explicitly elevated host action.
    HostPrivileged,
    /// Static analysis cannot determine the callable's effects.
    Unknown,
}

/// A host function available in a particular runtime session.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct HostFunctionDescriptor {
    /// Exact callable name resolved by Silk source.
    pub name: String,
    /// Exact authority identifier required for invocation.
    pub authority: String,
    /// Effects the function may perform.
    pub effects: BTreeSet<Effect>,
}

/// Session-scoped authority to invoke functions with bounded effects.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct Grant {
    /// Exact authority identifier authorized by this grant.
    pub authority: String,
    /// Maximum effect set permitted under this authority.
    pub effect_ceiling: BTreeSet<Effect>,
}

/// JSON-RPC request identifier accepted by protocol version 1.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum RequestId {
    /// String request identifier.
    String(String),
    /// Unsigned integer request identifier.
    Number(u64),
}

/// Minimal JSON-RPC 2.0 request envelope.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Request {
    /// JSON-RPC version marker.
    pub jsonrpc: String,
    /// Missing ID denotes a notification.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<RequestId>,
    /// Method name.
    pub method: String,
    /// Method parameters.
    #[serde(default)]
    pub params: Value,
}

/// Minimal JSON-RPC success response envelope.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct SuccessResponse {
    /// JSON-RPC version marker.
    pub jsonrpc: String,
    /// Request identifier.
    pub id: RequestId,
    /// Result value.
    pub result: Value,
}

/// JSON-RPC error value.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ProtocolError {
    /// Stable numeric JSON-RPC error code.
    pub code: i64,
    /// Human-readable diagnostic.
    pub message: String,
    /// Optional structured error details.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<Value>,
}

/// Minimal JSON-RPC error response envelope.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ErrorResponse {
    /// JSON-RPC version marker.
    pub jsonrpc: String,
    /// Request identifier, or `null` for an invalid request without a usable ID.
    pub id: Option<RequestId>,
    /// Structured failure.
    pub error: ProtocolError,
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::{Effect, PROTOCOL_VERSION, Request, RequestId};

    #[test]
    fn request_round_trips_as_json_rpc() {
        let request = Request {
            jsonrpc: "2.0".to_owned(),
            id: Some(RequestId::Number(7)),
            method: "session.create".to_owned(),
            params: json!({"protocol_version": PROTOCOL_VERSION}),
        };
        let bytes = serde_json::to_vec(&request).expect("serialize request");
        let decoded: Request = serde_json::from_slice(&bytes).expect("decode request");

        assert_eq!(decoded, request);
    }

    #[test]
    fn effects_use_stable_protocol_labels() {
        assert_eq!(
            serde_json::to_string(&Effect::HostPrivileged).expect("serialize effect"),
            "\"host_privileged\""
        );
    }
}
