//! Canonical, serializable Silk semantic IR.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

pub const SCHEMA_VERSION: &str = "silk.ir.v1";

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ProgramIR {
    pub schema_version: String,
    pub program_id: String,
    pub procedures: BTreeMap<String, Procedure>,
    pub source_map: BTreeMap<String, SourceSpan>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Procedure {
    pub id: String,
    pub name: String,
    pub parameters: Vec<String>,
    pub entry_block: String,
    pub blocks: BTreeMap<String, BasicBlock>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct BasicBlock {
    pub id: String,
    pub instructions: Vec<Instruction>,
    pub terminator: Terminator,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Instruction {
    pub id: String,
    pub result: Option<String>,
    pub op: Operation,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Operation {
    Constant {
        value: Literal,
    },
    Load {
        name: String,
    },
    LoadState {
        name: String,
    },
    Bind {
        name: String,
        value: String,
    },
    InitState {
        name: String,
        value: String,
    },
    StoreState {
        name: String,
        value: String,
    },
    IteratorInit {
        collection: String,
    },
    IteratorNext {
        iterator: String,
    },
    IteratorItem {
        iterator: String,
    },
    Unary {
        operator: String,
        operand: String,
    },
    Binary {
        operator: String,
        left: String,
        right: String,
    },
    Phi {
        incoming: Vec<(String, String)>,
    },
    Array {
        elements: Vec<String>,
    },
    Object {
        entries: Vec<(String, String)>,
    },
    Call {
        callee: String,
        target: CallTarget,
        arguments: Vec<String>,
    },
    Member {
        object: String,
        key: String,
    },
    Index {
        object: String,
        index: String,
    },
    Assign {
        target: String,
        value: String,
    },
    Discard {
        value: String,
    },
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum CallTarget {
    Core,
    Procedure,
    HostFunction,
    Dynamic,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum Literal {
    Null,
    Boolean(bool),
    Integer(i64),
    Number(f64),
    String(String),
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Terminator {
    Jump {
        target: String,
    },
    Branch {
        condition: String,
        then_target: String,
        else_target: String,
    },
    Return {
        value: Option<String>,
    },
    Yield {
        value: Option<String>,
    },
    End,
    Break {
        target: String,
    },
    Continue {
        target: String,
    },
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct SourceSpan {
    pub start: usize,
    pub end: usize,
    pub line: usize,
    pub column: usize,
}

#[cfg(test)]
mod tests {
    use super::{ProgramIR, SCHEMA_VERSION};

    #[test]
    fn schema_identifier_is_stable() {
        assert_eq!(SCHEMA_VERSION, "silk.ir.v1");
        let ir = ProgramIR {
            schema_version: SCHEMA_VERSION.to_owned(),
            program_id: "source:abc".to_owned(),
            procedures: Default::default(),
            source_map: Default::default(),
        };
        assert_eq!(ir.schema_version, "silk.ir.v1");
    }
}
