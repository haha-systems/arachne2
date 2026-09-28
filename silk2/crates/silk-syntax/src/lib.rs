//! Silk source parser and first-stage semantic IR lowering.

use std::collections::{BTreeMap, BTreeSet};

use silk_ir::{
    BasicBlock, CallTarget, Instruction, Literal, Operation, Procedure, ProgramIR, SCHEMA_VERSION,
    SourceSpan, Terminator,
};

/// A parsed Silk source file retained for diagnostics and later execution.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Program {
    source: String,
}

impl Program {
    /// Returns the exact UTF-8 source text supplied to [`parse`].
    #[must_use]
    pub fn source(&self) -> &str {
        &self.source
    }
}

/// A syntax or lowering diagnostic with a one-based source location.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ParseError {
    message: String,
    span: SourceSpan,
}

impl ParseError {
    /// Returns the diagnostic message.
    #[must_use]
    pub fn message(&self) -> &str {
        &self.message
    }

    /// Returns the UTF-8 byte offsets and one-based line/column.
    #[must_use]
    pub const fn span(&self) -> &SourceSpan {
        &self.span
    }
}

impl std::fmt::Display for ParseError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(
            formatter,
            "{}:{}: {}",
            self.span.line, self.span.column, self.message
        )
    }
}

impl std::error::Error for ParseError {}

/// Parses the supported Silk subset while retaining the original source.
pub fn parse(source: &str) -> Result<Program, ParseError> {
    let tokens = Lexer::new(source).lex()?;
    Parser::new(tokens).program()?;
    Ok(Program {
        source: source.to_owned(),
    })
}

/// Parses and lowers a Silk source program to the canonical `silk.ir.v1` form.
pub fn lower(source: &str) -> Result<ProgramIR, ParseError> {
    let tokens = Lexer::new(source).lex()?;
    let ast = Parser::new(tokens).program()?;
    Lowerer::new(source, ast).lower()
}

#[derive(Clone, Debug, PartialEq)]
struct Token {
    kind: TokenKind,
    span: SourceSpan,
}

#[derive(Clone, Debug, PartialEq)]
enum TokenKind {
    Ident(String),
    Integer(i64),
    Number(f64),
    String(String),
    Symbol(String),
    Eof,
}

struct Lexer<'a> {
    source: &'a str,
    offset: usize,
    line: usize,
    column: usize,
}

impl<'a> Lexer<'a> {
    const fn new(source: &'a str) -> Self {
        Self {
            source,
            offset: 0,
            line: 1,
            column: 1,
        }
    }

    fn lex(mut self) -> Result<Vec<Token>, ParseError> {
        let mut tokens = Vec::new();
        while let Some(ch) = self.peek() {
            if ch.is_whitespace() {
                self.bump();
            } else if self.starts_with("//") || ch == '#' {
                while self.peek().is_some_and(|value| value != '\n') {
                    self.bump();
                }
            } else if ch.is_ascii_alphabetic() || ch == '_' {
                tokens.push(self.identifier());
            } else if ch.is_ascii_digit() {
                tokens.push(self.number()?);
            } else if ch == '"' || ch == '\'' {
                tokens.push(self.string()?);
            } else {
                tokens.push(self.symbol()?);
            }
        }
        let span = SourceSpan {
            start: self.offset,
            end: self.offset,
            line: self.line,
            column: self.column,
        };
        tokens.push(Token {
            kind: TokenKind::Eof,
            span,
        });
        Ok(tokens)
    }

    fn identifier(&mut self) -> Token {
        let start = self.mark();
        let begin = self.offset;
        while self
            .peek()
            .is_some_and(|ch| ch.is_ascii_alphanumeric() || ch == '_')
        {
            self.bump();
        }
        self.token(
            start,
            TokenKind::Ident(self.source[begin..self.offset].to_owned()),
        )
    }

    fn number(&mut self) -> Result<Token, ParseError> {
        let start = self.mark();
        let begin = self.offset;
        while self.peek().is_some_and(|ch| ch.is_ascii_digit()) {
            self.bump();
        }
        if self.peek() == Some('.') && self.peek_after(1).is_some_and(|ch| ch.is_ascii_digit()) {
            self.bump();
            while self.peek().is_some_and(|ch| ch.is_ascii_digit()) {
                self.bump();
            }
        }
        let raw = &self.source[begin..self.offset];
        if raw.contains('.') {
            let value = raw
                .parse::<f64>()
                .map_err(|_| self.error_at(start, "invalid number"))?;
            if !value.is_finite() {
                return Err(self.error_at(start, "number must be finite"));
            }
            Ok(self.token(start, TokenKind::Number(value)))
        } else {
            let value = raw
                .parse::<i64>()
                .map_err(|_| self.error_at(start, "integer literal is out of range"))?;
            Ok(self.token(start, TokenKind::Integer(value)))
        }
    }

    fn string(&mut self) -> Result<Token, ParseError> {
        let start = self.mark();
        let quote = self.bump().expect("string quote");
        let mut value = String::new();
        loop {
            match self.bump() {
                Some(ch) if ch == quote => break,
                Some('\\') => match self.bump() {
                    Some('n') => value.push('\n'),
                    Some('r') => value.push('\r'),
                    Some('t') => value.push('\t'),
                    Some('\\') => value.push('\\'),
                    Some('"') => value.push('"'),
                    Some('\'') => value.push('\''),
                    Some(ch) => {
                        return Err(self.error_at(start, &format!("unsupported escape \\{ch}")));
                    }
                    None => return Err(self.error_at(start, "unterminated escape")),
                },
                Some(ch) => value.push(ch),
                None => return Err(self.error_at(start, "unterminated string")),
            }
        }
        Ok(self.token(start, TokenKind::String(value)))
    }

    fn symbol(&mut self) -> Result<Token, ParseError> {
        let start = self.mark();
        for operator in ["==", "!=", "<=", ">=", "&&", "||", "->"] {
            if self.starts_with(operator) {
                for _ in operator.chars() {
                    self.bump();
                }
                return Ok(self.token(start, TokenKind::Symbol(operator.to_owned())));
            }
        }
        let ch = self.bump().expect("peeked symbol");
        if "{}()[],:.;+-*/%!<>=".contains(ch) {
            Ok(self.token(start, TokenKind::Symbol(ch.to_string())))
        } else {
            Err(self.error_at(start, &format!("unexpected character {ch:?}")))
        }
    }

    fn token(&self, mark: (usize, usize, usize), kind: TokenKind) -> Token {
        Token {
            kind,
            span: SourceSpan {
                start: mark.0,
                end: self.offset,
                line: mark.1,
                column: mark.2,
            },
        }
    }
    fn mark(&self) -> (usize, usize, usize) {
        (self.offset, self.line, self.column)
    }
    fn peek(&self) -> Option<char> {
        self.source[self.offset..].chars().next()
    }
    fn peek_after(&self, count: usize) -> Option<char> {
        self.source[self.offset..].chars().nth(count)
    }
    fn starts_with(&self, text: &str) -> bool {
        self.source[self.offset..].starts_with(text)
    }
    fn bump(&mut self) -> Option<char> {
        let ch = self.peek()?;
        self.offset += ch.len_utf8();
        if ch == '\n' {
            self.line += 1;
            self.column = 1;
        } else {
            self.column += 1;
        }
        Some(ch)
    }
    fn error_at(&self, mark: (usize, usize, usize), message: &str) -> ParseError {
        ParseError {
            message: message.to_owned(),
            span: SourceSpan {
                start: mark.0,
                end: self.offset,
                line: mark.1,
                column: mark.2,
            },
        }
    }
}

#[derive(Clone, Debug)]
struct Decl {
    name: String,
    params: Vec<String>,
    body: Vec<Stmt>,
    span: SourceSpan,
}
#[derive(Clone, Debug)]
enum Stmt {
    Let {
        name: String,
        value: Expr,
        state: bool,
        span: SourceSpan,
    },
    Assign {
        name: String,
        value: Expr,
        span: SourceSpan,
    },
    Expr(Expr),
    Return(Option<Expr>, SourceSpan),
    Yield(Option<Expr>, SourceSpan),
    If(Expr, Vec<Stmt>, Vec<Stmt>, SourceSpan),
    While(Expr, Vec<Stmt>, SourceSpan),
    For(String, Expr, Vec<Stmt>, SourceSpan),
    Break(SourceSpan),
    Continue(SourceSpan),
}
impl Stmt {
    fn span(&self) -> &SourceSpan {
        match self {
            Self::Let { span, .. }
            | Self::Assign { span, .. }
            | Self::Return(_, span)
            | Self::Yield(_, span)
            | Self::If(_, _, _, span)
            | Self::While(_, _, span)
            | Self::For(_, _, _, span)
            | Self::Break(span)
            | Self::Continue(span) => span,
            Self::Expr(expression) => expression.span(),
        }
    }
}
#[derive(Clone, Debug)]
enum Expr {
    Null(SourceSpan),
    Bool(bool, SourceSpan),
    Integer(i64, SourceSpan),
    Number(f64, SourceSpan),
    String(String, SourceSpan),
    Name(String, SourceSpan),
    Array(Vec<Expr>, SourceSpan),
    Object(Vec<(String, Expr)>, SourceSpan),
    Unary(String, Box<Expr>, SourceSpan),
    Binary(String, Box<Expr>, Box<Expr>, SourceSpan),
    Call(Box<Expr>, Vec<Expr>, SourceSpan),
    Member(Box<Expr>, String, SourceSpan),
    Index(Box<Expr>, Box<Expr>, SourceSpan),
}
impl Expr {
    fn span(&self) -> &SourceSpan {
        match self {
            Self::Null(s)
            | Self::Bool(_, s)
            | Self::Integer(_, s)
            | Self::Number(_, s)
            | Self::String(_, s)
            | Self::Name(_, s)
            | Self::Array(_, s)
            | Self::Object(_, s)
            | Self::Unary(_, _, s)
            | Self::Binary(_, _, _, s)
            | Self::Call(_, _, s)
            | Self::Member(_, _, s)
            | Self::Index(_, _, s) => s,
        }
    }
}

struct Parser {
    tokens: Vec<Token>,
    cursor: usize,
}
impl Parser {
    fn new(tokens: Vec<Token>) -> Self {
        Self { tokens, cursor: 0 }
    }
    fn program(mut self) -> Result<(Vec<Decl>, Vec<Stmt>), ParseError> {
        let mut declarations = Vec::new();
        let mut top_level = Vec::new();
        let mut names = BTreeSet::new();
        while !self.at_eof() {
            let start = self.current().span.clone();
            let is_procedure = matches!(&self.current().kind, TokenKind::Ident(value) if ["fn", "policy", "learn"].contains(&value.as_str()));
            if !is_procedure {
                if matches!(&self.current().kind, TokenKind::Ident(value) if ["act", "tool", "capability", "import", "export", "learning"].contains(&value.as_str()))
                {
                    let name = match &self.current().kind {
                        TokenKind::Ident(value) => value,
                        _ => unreachable!(),
                    };
                    return Err(self.error(format!("unsupported top-level declaration `{name}`; see the Silk 2 syntax migration rules")));
                }
                top_level.push(self.statement()?);
                self.consume(";");
                continue;
            }
            self.cursor += 1;
            let name = self.ident()?.to_owned();
            if !names.insert(name.clone()) {
                return Err(self.error(format!("duplicate procedure `{name}`")));
            }
            self.expect("(")?;
            let mut params = Vec::new();
            if !self.check(")") {
                loop {
                    let param = self.ident()?.to_owned();
                    if params.contains(&param) {
                        return Err(self.error(format!("duplicate parameter `{param}`")));
                    }
                    params.push(param);
                    if !self.consume(",") {
                        break;
                    }
                }
            }
            self.expect(")")?;
            let body = self.block()?;
            declarations.push(Decl {
                name,
                params,
                body,
                span: start,
            });
            self.consume(";");
        }
        Ok((declarations, top_level))
    }
    fn block(&mut self) -> Result<Vec<Stmt>, ParseError> {
        self.expect("{")?;
        let mut statements = Vec::new();
        while !self.check("}") && !self.at_eof() {
            statements.push(self.statement()?);
            self.consume(";");
        }
        self.expect("}")?;
        Ok(statements)
    }
    fn statement(&mut self) -> Result<Stmt, ParseError> {
        let span = self.current().span.clone();
        if self.consume_word("let") || self.consume_word("state") {
            let state = self.previous_ident_is("state");
            let name = self.ident()?.to_owned();
            self.expect("=")?;
            let value = self.expression(0)?;
            return Ok(Stmt::Let {
                name,
                value,
                state,
                span,
            });
        }
        if self.consume_word("return") {
            let value = if self.check("}") || self.check(";") {
                None
            } else {
                Some(self.expression(0)?)
            };
            return Ok(Stmt::Return(value, span));
        }
        if self.consume_word("yield") {
            let value = if self.check("}") || self.check(";") {
                None
            } else {
                Some(self.expression(0)?)
            };
            return Ok(Stmt::Yield(value, span));
        }
        if self.consume_word("if") {
            let condition = self.expression(0)?;
            let then_body = self.block()?;
            let else_body = if self.consume_word("else") {
                if self.consume_word("if") {
                    let condition = self.expression(0)?;
                    let then_body = self.block()?;
                    let else_body = if self.consume_word("else") {
                        self.block()?
                    } else {
                        Vec::new()
                    };
                    vec![Stmt::If(condition, then_body, else_body, span.clone())]
                } else {
                    self.block()?
                }
            } else {
                Vec::new()
            };
            return Ok(Stmt::If(condition, then_body, else_body, span));
        }
        if self.consume_word("while") {
            let condition = self.expression(0)?;
            let body = self.block()?;
            return Ok(Stmt::While(condition, body, span));
        }
        if self.consume_word("for") {
            let name = self.ident()?.to_owned();
            if !self.consume_word("in") {
                return Err(self.error("expected `in` after loop variable"));
            }
            let iterable = self.expression(0)?;
            let body = self.block()?;
            return Ok(Stmt::For(name, iterable, body, span));
        }
        if self.consume_word("break") {
            return Ok(Stmt::Break(span));
        }
        if self.consume_word("continue") {
            return Ok(Stmt::Continue(span));
        }
        if let (TokenKind::Ident(name), Some(next)) =
            (&self.current().kind, self.tokens.get(self.cursor + 1))
        {
            if matches!(&next.kind, TokenKind::Symbol(op) if op=="=") {
                let name = name.clone();
                self.cursor += 2;
                let value = self.expression(0)?;
                return Ok(Stmt::Assign { name, value, span });
            }
        }
        Ok(Stmt::Expr(self.expression(0)?))
    }
    fn expression(&mut self, min_precedence: u8) -> Result<Expr, ParseError> {
        let mut lhs = self.prefix()?;
        loop {
            if self.consume("(") {
                let span = lhs.span().clone();
                let mut args = Vec::new();
                if !self.check(")") {
                    loop {
                        args.push(self.expression(0)?);
                        if !self.consume(",") {
                            break;
                        }
                    }
                }
                self.expect(")")?;
                lhs = Expr::Call(Box::new(lhs), args, span);
                continue;
            }
            if self.consume(".") {
                let span = lhs.span().clone();
                let name = self.ident()?.to_owned();
                lhs = Expr::Member(Box::new(lhs), name, span);
                continue;
            }
            if self.consume("[") {
                let span = lhs.span().clone();
                let index = self.expression(0)?;
                self.expect("]")?;
                lhs = Expr::Index(Box::new(lhs), Box::new(index), span);
                continue;
            }
            let Some((op, precedence)) = self.binary_operator() else {
                break;
            };
            if precedence < min_precedence {
                break;
            }
            self.cursor += 1;
            let rhs = self.expression(precedence + 1)?;
            let span = lhs.span().clone();
            lhs = Expr::Binary(op, Box::new(lhs), Box::new(rhs), span);
        }
        Ok(lhs)
    }
    fn prefix(&mut self) -> Result<Expr, ParseError> {
        let token = self.advance().clone();
        let span = token.span.clone();
        match token.kind {
            TokenKind::Integer(v) => Ok(Expr::Integer(v, span)),
            TokenKind::Number(v) => Ok(Expr::Number(v, span)),
            TokenKind::String(v) => Ok(Expr::String(v, span)),
            TokenKind::Ident(v) if v == "null" => Ok(Expr::Null(span)),
            TokenKind::Ident(v) if v == "true" => Ok(Expr::Bool(true, span)),
            TokenKind::Ident(v) if v == "false" => Ok(Expr::Bool(false, span)),
            TokenKind::Ident(v) => Ok(Expr::Name(v, span)),
            TokenKind::Symbol(op) if op == "-" || op == "!" => {
                let rhs = self.expression(7)?;
                Ok(Expr::Unary(op, Box::new(rhs), span))
            }
            TokenKind::Symbol(symbol) if symbol == "(" => {
                let expr = self.expression(0)?;
                self.expect(")")?;
                Ok(expr)
            }
            TokenKind::Symbol(symbol) if symbol == "[" => {
                let mut items = Vec::new();
                if !self.check("]") {
                    loop {
                        items.push(self.expression(0)?);
                        if !self.consume(",") {
                            break;
                        }
                    }
                }
                self.expect("]")?;
                Ok(Expr::Array(items, span))
            }
            TokenKind::Symbol(symbol) if symbol == "{" => {
                let mut entries = Vec::new();
                let mut keys = BTreeSet::new();
                if !self.check("}") {
                    loop {
                        let key = match self.advance().kind.clone() {
                            TokenKind::Ident(k) | TokenKind::String(k) => k,
                            _ => {
                                return Err(
                                    self.error("object keys must be identifiers or strings")
                                );
                            }
                        };
                        if !keys.insert(key.clone()) {
                            return Err(self.error(format!("duplicate object key `{key}`")));
                        }
                        self.expect(":")?;
                        entries.push((key, self.expression(0)?));
                        if !self.consume(",") {
                            break;
                        }
                    }
                }
                self.expect("}")?;
                Ok(Expr::Object(entries, span))
            }
            _ => Err(ParseError {
                message: "expected expression".to_owned(),
                span,
            }),
        }
    }
    fn binary_operator(&self) -> Option<(String, u8)> {
        if let TokenKind::Ident(op) = &self.current().kind {
            let precedence = match op.as_str() {
                "or" => 1,
                "and" => 2,
                _ => return None,
            };
            return Some((op.clone(), precedence));
        }
        let TokenKind::Symbol(op) = &self.current().kind else {
            return None;
        };
        let p = match op.as_str() {
            "||" => 1,
            "or" => 1,
            "&&" => 2,
            "and" => 2,
            "==" | "!=" => 3,
            "<" | ">" | "<=" | ">=" => 4,
            "+" | "-" => 5,
            "*" | "/" | "%" => 6,
            _ => return None,
        };
        Some((op.clone(), p))
    }
    fn ident(&mut self) -> Result<String, ParseError> {
        match self.advance().kind.clone() {
            TokenKind::Ident(v) => Ok(v),
            _ => Err(self.error("expected identifier")),
        }
    }
    fn expect(&mut self, s: &str) -> Result<(), ParseError> {
        if self.consume(s) {
            Ok(())
        } else {
            Err(self.error(format!("expected `{s}`")))
        }
    }
    fn consume(&mut self, s: &str) -> bool {
        if self.check(s) {
            self.cursor += 1;
            true
        } else {
            false
        }
    }
    fn consume_word(&mut self, s: &str) -> bool {
        matches!(&self.current().kind,TokenKind::Ident(v) if v==s)
            .then(|| {
                self.cursor += 1;
            })
            .is_some()
    }
    fn check(&self, s: &str) -> bool {
        matches!(&self.current().kind,TokenKind::Symbol(v) if v==s)
    }
    fn current(&self) -> &Token {
        &self.tokens[self.cursor]
    }
    fn advance(&mut self) -> &Token {
        let current = self.cursor;
        if !self.at_eof() {
            self.cursor += 1;
        }
        &self.tokens[current]
    }
    fn at_eof(&self) -> bool {
        matches!(self.current().kind, TokenKind::Eof)
    }
    fn error(&self, message: impl Into<String>) -> ParseError {
        ParseError {
            message: message.into(),
            span: self.current().span.clone(),
        }
    }
    fn previous_ident_is(&self, word: &str) -> bool {
        matches!(self.tokens.get(self.cursor.saturating_sub(1)).map(|token|&token.kind),Some(TokenKind::Ident(value)) if value==word)
    }
}

struct Lowerer<'a> {
    source: &'a str,
    ast: (Vec<Decl>, Vec<Stmt>),
}
impl<'a> Lowerer<'a> {
    fn new(source: &'a str, ast: (Vec<Decl>, Vec<Stmt>)) -> Self {
        Self { source, ast }
    }
    fn lower(self) -> Result<ProgramIR, ParseError> {
        let mut procedures = BTreeMap::new();
        let mut source_map = BTreeMap::new();
        let (declarations, top_level) = self.ast;
        if let Some(statement) = top_level
            .iter()
            .find(|statement| matches!(statement, Stmt::Return(..) | Stmt::Yield(..)))
        {
            let span = match statement {
                Stmt::Return(_, span) | Stmt::Yield(_, span) => span,
                _ => unreachable!(),
            };
            return Err(ParseError {
                message: "return and yield are only valid inside a procedure".to_owned(),
                span: span.clone(),
            });
        }
        for declaration in declarations {
            let id = format!("proc:{}", declaration.name);
            let (procedure, mappings) = ProcedureBuilder::new(
                id.clone(),
                declaration.name.clone(),
                declaration.params,
                &declaration.body,
            )
            .build()?;
            source_map.extend(mappings);
            source_map.insert(id.clone(), declaration.span);
            procedures.insert(id, procedure);
        }
        if !top_level.is_empty() {
            let id = "proc:main".to_owned();
            if procedures.contains_key(&id) {
                return Err(ParseError {
                    message: "procedure name `main` conflicts with the top-level entry procedure"
                        .to_owned(),
                    span: top_level[0].span().clone(),
                });
            }
            let (procedure, mappings) =
                ProcedureBuilder::new(id.clone(), "main".to_owned(), Vec::new(), &top_level)
                    .build()?;
            source_map.extend(mappings);
            procedures.insert(id, procedure);
        }
        for (procedure_id, procedure) in &procedures {
            for block in procedure.blocks.values() {
                for instruction in &block.instructions {
                    if let Operation::Call {
                        callee,
                        target: CallTarget::Procedure,
                        ..
                    } = &instruction.op
                    {
                        if !procedures.contains_key(&format!("proc:{callee}")) {
                            let span =
                                source_map
                                    .get(&instruction.id)
                                    .cloned()
                                    .unwrap_or(SourceSpan {
                                        start: 0,
                                        end: 0,
                                        line: 1,
                                        column: 1,
                                    });
                            return Err(ParseError {
                                message: format!(
                                    "unknown procedure `{callee}` called from `{procedure_id}`"
                                ),
                                span,
                            });
                        }
                    }
                }
            }
        }
        let digest = source_digest(self.source);
        Ok(ProgramIR {
            schema_version: SCHEMA_VERSION.to_owned(),
            program_id: format!("source:{digest:016x}"),
            procedures,
            source_map,
        })
    }
}

struct ProcedureBuilder<'a> {
    id: String,
    name: String,
    parameters: Vec<String>,
    statements: &'a [Stmt],
    blocks: BTreeMap<String, BasicBlock>,
    current: String,
    next_value: usize,
    next_instruction: usize,
    next_block: usize,
    source_map: BTreeMap<String, SourceSpan>,
    state_names: BTreeSet<String>,
    locals: BTreeSet<String>,
    loops: Vec<(String, String)>,
}
impl<'a> ProcedureBuilder<'a> {
    fn new(id: String, name: String, parameters: Vec<String>, statements: &'a [Stmt]) -> Self {
        let entry = "b0".to_owned();
        Self {
            id,
            name,
            parameters,
            statements,
            blocks: BTreeMap::new(),
            current: entry,
            next_value: 0,
            next_instruction: 0,
            next_block: 1,
            source_map: BTreeMap::new(),
            state_names: BTreeSet::new(),
            locals: BTreeSet::new(),
            loops: Vec::new(),
        }
    }
    fn build(mut self) -> Result<(Procedure, BTreeMap<String, SourceSpan>), ParseError> {
        let entry = "b0".to_owned();
        self.lower_statements(self.statements)?;
        if !self.is_terminated() {
            self.terminate(Terminator::End)
        }
        let procedure = Procedure {
            id: self.id,
            name: self.name,
            parameters: self.parameters,
            entry_block: entry,
            blocks: self.blocks,
        };
        Ok((procedure, self.source_map))
    }
    fn lower_statements(&mut self, statements: &[Stmt]) -> Result<(), ParseError> {
        for statement in statements {
            if self.is_terminated() {
                break;
            }
            self.statement(statement)?;
        }
        Ok(())
    }
    fn statement(&mut self, stmt: &Stmt) -> Result<(), ParseError> {
        match stmt {
            Stmt::Let {
                name,
                value,
                state,
                span,
            } => {
                if self.locals.contains(name)
                    || self.state_names.contains(name)
                    || self.parameters.contains(name)
                {
                    return Err(self.error_at(span, format!("duplicate binding `{name}`")));
                }
                let v = self.expression(value)?;
                if *state {
                    self.state_names.insert(name.clone());
                    self.emit(
                        None,
                        Operation::InitState {
                            name: name.clone(),
                            value: v,
                        },
                        span.clone(),
                    );
                } else {
                    self.locals.insert(name.clone());
                    self.emit(
                        None,
                        Operation::Bind {
                            name: name.clone(),
                            value: v,
                        },
                        span.clone(),
                    );
                }
            }
            Stmt::Assign { name, value, span } => {
                let v = self.expression(value)?;
                if self.state_names.contains(name) {
                    self.emit(
                        None,
                        Operation::StoreState {
                            name: name.clone(),
                            value: v,
                        },
                        span.clone(),
                    );
                } else {
                    return Err(self.error_at(
                        span,
                        format!("assignment to unknown or immutable binding `{name}`"),
                    ));
                }
            }
            Stmt::Expr(expr) => {
                let v = self.expression(expr)?;
                self.emit(None, Operation::Discard { value: v }, expr.span().clone());
            }
            Stmt::Return(value, span) | Stmt::Yield(value, span) => {
                let v = value.as_ref().map(|e| self.expression(e)).transpose()?;
                let term = if matches!(stmt, Stmt::Yield(..)) {
                    Terminator::Yield { value: v }
                } else {
                    Terminator::Return { value: v }
                };
                self.terminate(term);
                self.source_map.insert(self.current.clone(), span.clone());
            }
            Stmt::If(condition, then_body, else_body, span) => {
                self.if_statement(condition, then_body, else_body, span)?
            }
            Stmt::While(condition, body, span) => self.while_statement(condition, body, span)?,
            Stmt::For(name, iterable, body, span) => {
                self.for_statement(name, iterable, body, span)?
            }
            Stmt::Break(span) => {
                let Some((break_target, _)) = self.loops.last().cloned() else {
                    return Err(self.error_at(span, "break outside loop"));
                };
                self.terminate(Terminator::Break {
                    target: break_target,
                });
            }
            Stmt::Continue(span) => {
                let Some((_, continue_target)) = self.loops.last().cloned() else {
                    return Err(self.error_at(span, "continue outside loop"));
                };
                self.terminate(Terminator::Continue {
                    target: continue_target,
                });
            }
        }
        Ok(())
    }
    fn if_statement(
        &mut self,
        condition: &Expr,
        then_body: &[Stmt],
        else_body: &[Stmt],
        span: &SourceSpan,
    ) -> Result<(), ParseError> {
        let cond = self.expression(condition)?;
        let then_id = self.new_block();
        let else_id = self.new_block();
        let join_id = self.new_block();
        self.terminate(Terminator::Branch {
            condition: cond,
            then_target: then_id.clone(),
            else_target: else_id.clone(),
        });
        self.current = then_id;
        self.lower_statements(then_body)?;
        if !self.is_terminated() {
            self.terminate(Terminator::Jump {
                target: join_id.clone(),
            });
        }
        self.current = else_id;
        self.lower_statements(else_body)?;
        if !self.is_terminated() {
            self.terminate(Terminator::Jump {
                target: join_id.clone(),
            });
        }
        self.current = join_id.clone();
        self.source_map.insert(join_id, span.clone());
        Ok(())
    }
    fn while_statement(
        &mut self,
        condition: &Expr,
        body: &[Stmt],
        span: &SourceSpan,
    ) -> Result<(), ParseError> {
        let cond_id = self.new_block();
        let body_id = self.new_block();
        let exit_id = self.new_block();
        self.terminate(Terminator::Jump {
            target: cond_id.clone(),
        });
        self.current = cond_id.clone();
        let cond = self.expression(condition)?;
        self.terminate(Terminator::Branch {
            condition: cond,
            then_target: body_id.clone(),
            else_target: exit_id.clone(),
        });
        self.current = body_id;
        self.loops.push((exit_id.clone(), cond_id.clone()));
        self.lower_statements(body)?;
        self.loops.pop();
        if !self.is_terminated() {
            self.terminate(Terminator::Jump { target: cond_id });
        }
        self.current = exit_id.clone();
        self.source_map.insert(exit_id, span.clone());
        Ok(())
    }
    fn for_statement(
        &mut self,
        name: &str,
        iterable: &Expr,
        body: &[Stmt],
        span: &SourceSpan,
    ) -> Result<(), ParseError> {
        let collection = self.expression(iterable)?;
        let iterator_value = self.value();
        let iterator = self
            .emit(
                Some(iterator_value),
                Operation::IteratorInit { collection },
                span.clone(),
            )
            .expect("iterator result");
        let loop_id = self.new_block();
        let body_id = self.new_block();
        let exit_id = self.new_block();
        self.terminate(Terminator::Jump {
            target: loop_id.clone(),
        });
        self.current = loop_id.clone();
        let has_item_value = self.value();
        let has_item = self
            .emit(
                Some(has_item_value),
                Operation::IteratorNext {
                    iterator: iterator.clone(),
                },
                span.clone(),
            )
            .expect("iterator result");
        self.terminate(Terminator::Branch {
            condition: has_item,
            then_target: body_id.clone(),
            else_target: exit_id.clone(),
        });
        self.current = body_id;
        self.locals.insert(name.to_owned());
        let item_value = self.value();
        let item = self
            .emit(
                Some(item_value),
                Operation::IteratorItem { iterator },
                span.clone(),
            )
            .expect("iterator item");
        self.emit(
            None,
            Operation::Bind {
                name: name.to_owned(),
                value: item,
            },
            span.clone(),
        );
        self.loops.push((exit_id.clone(), loop_id.clone()));
        self.lower_statements(body)?;
        self.loops.pop();
        if !self.is_terminated() {
            self.terminate(Terminator::Jump { target: loop_id });
        }
        self.current = exit_id.clone();
        self.source_map.insert(exit_id, span.clone());
        Ok(())
    }
    fn expression(&mut self, expr: &Expr) -> Result<String, ParseError> {
        let span = expr.span().clone();
        let op = match expr {
            Expr::Null(_) => Operation::Constant {
                value: Literal::Null,
            },
            Expr::Bool(value, _) => Operation::Constant {
                value: Literal::Boolean(*value),
            },
            Expr::Integer(value, _) => Operation::Constant {
                value: Literal::Integer(*value),
            },
            Expr::Number(value, _) => Operation::Constant {
                value: Literal::Number(*value),
            },
            Expr::String(value, _) => Operation::Constant {
                value: Literal::String(value.clone()),
            },
            Expr::Name(name, _)
                if self.locals.contains(name)
                    || self.state_names.contains(name)
                    || self.parameters.contains(name) =>
            {
                if self.state_names.contains(name) {
                    Operation::LoadState { name: name.clone() }
                } else {
                    Operation::Load { name: name.clone() }
                }
            }
            Expr::Name(name, _) => {
                return Err(self.error_at(&span, format!("unknown binding `{name}`")));
            }
            Expr::Array(items, _) => Operation::Array {
                elements: items
                    .iter()
                    .map(|item| self.expression(item))
                    .collect::<Result<_, _>>()?,
            },
            Expr::Object(entries, _) => Operation::Object {
                entries: entries
                    .iter()
                    .map(|(key, value)| Ok((key.clone(), self.expression(value)?)))
                    .collect::<Result<_, ParseError>>()?,
            },
            Expr::Unary(operator, value, _) => Operation::Unary {
                operator: operator.clone(),
                operand: self.expression(value)?,
            },
            Expr::Binary(operator, left, right, _)
                if ["and", "&&", "or", "||"].contains(&operator.as_str()) =>
            {
                let left_value = self.expression(left)?;
                let right_block = self.new_block();
                let short_block = self.new_block();
                let join_block = self.new_block();
                let is_and = operator == "and" || operator == "&&";
                let (then_target, else_target) = if is_and {
                    (right_block.clone(), short_block.clone())
                } else {
                    (short_block.clone(), right_block.clone())
                };
                self.terminate(Terminator::Branch {
                    condition: left_value,
                    then_target,
                    else_target,
                });
                self.current = short_block.clone();
                let short_result = self.value();
                let short_value = self
                    .emit(
                        Some(short_result),
                        Operation::Constant {
                            value: Literal::Boolean(!is_and),
                        },
                        span.clone(),
                    )
                    .expect("short-circuit value");
                self.terminate(Terminator::Jump {
                    target: join_block.clone(),
                });
                self.current = right_block.clone();
                let right_value = self.expression(right)?;
                self.terminate(Terminator::Jump {
                    target: join_block.clone(),
                });
                self.current = join_block;
                Operation::Phi {
                    incoming: vec![(short_block, short_value), (right_block, right_value)],
                }
            }
            Expr::Binary(operator, left, right, _) => Operation::Binary {
                operator: operator.clone(),
                left: self.expression(left)?,
                right: self.expression(right)?,
            },
            Expr::Call(callee, args, _) => {
                let name = self.call_name(callee)?;
                let target = if name.contains('.') {
                    CallTarget::HostFunction
                } else if ["len", "str", "append", "contains", "print", "sleep", "now"]
                    .contains(&name.as_str())
                {
                    CallTarget::Core
                } else if name.chars().next().is_some_and(char::is_lowercase) {
                    CallTarget::Procedure
                } else {
                    CallTarget::Dynamic
                };
                Operation::Call {
                    callee: name,
                    target,
                    arguments: args
                        .iter()
                        .map(|arg| self.expression(arg))
                        .collect::<Result<_, _>>()?,
                }
            }
            Expr::Member(object, key, _) => Operation::Member {
                object: self.expression(object)?,
                key: key.clone(),
            },
            Expr::Index(object, index, _) => Operation::Index {
                object: self.expression(object)?,
                index: self.expression(index)?,
            },
        };
        let value = self.value();
        Ok(self.emit(Some(value), op, span).expect("expression result"))
    }
    fn call_name(&self, callee: &Expr) -> Result<String, ParseError> {
        match callee {
            Expr::Name(name, _) => Ok(name.clone()),
            Expr::Member(base, key, _) => Ok(format!("{}.{}", self.call_name(base)?, key)),
            _ => Err(self.error_at(callee.span(), "call target must be a name or member path")),
        }
    }
    fn emit(&mut self, result: Option<String>, op: Operation, span: SourceSpan) -> Option<String> {
        let id = format!("{}:i{}", self.id, self.next_instruction);
        self.next_instruction += 1;
        let instruction = Instruction {
            id: id.clone(),
            result: result.clone(),
            op,
        };
        self.block_mut().instructions.push(instruction);
        self.source_map.insert(id, span);
        result
    }
    fn terminate(&mut self, terminator: Terminator) {
        self.block_mut().terminator = terminator;
    }
    fn block_mut(&mut self) -> &mut BasicBlock {
        self.blocks
            .entry(self.current.clone())
            .or_insert_with(|| BasicBlock {
                id: self.current.clone(),
                instructions: Vec::new(),
                terminator: Terminator::End,
            })
    }
    fn is_terminated(&self) -> bool {
        self.blocks
            .get(&self.current)
            .is_some_and(|block| !matches!(block.terminator, Terminator::End))
    }
    fn new_block(&mut self) -> String {
        let id = format!("b{}", self.next_block);
        self.next_block += 1;
        id
    }
    fn value(&mut self) -> String {
        let id = format!("v{}", self.next_value);
        self.next_value += 1;
        id
    }
    fn error_at(&self, span: &SourceSpan, message: impl Into<String>) -> ParseError {
        ParseError {
            message: message.into(),
            span: span.clone(),
        }
    }
}

fn source_digest(source: &str) -> u64 {
    source
        .as_bytes()
        .iter()
        .fold(0xcbf29ce484222325_u64, |hash, byte| {
            (hash ^ u64::from(*byte)).wrapping_mul(0x100000001b3)
        })
}

#[cfg(test)]
mod tests {
    use super::{lower, parse};
    use silk_ir::{Operation, Terminator};

    #[test]
    fn preserves_source_bytes() {
        let source = "state count = 1\nprint(count)\n";
        assert_eq!(parse(source).expect("parsed source").source(), source);
    }
    #[test]
    fn lowers_procedures_bindings_calls_and_source_spans() {
        let ir = lower(
            "fn greet(name) { let message = \"hello\" + name; print(message); return message }",
        )
        .expect("lowered program");
        let procedure = &ir.procedures["proc:greet"];
        assert_eq!(procedure.parameters, vec!["name"]);
        assert!(
            procedure.blocks["b0"]
                .instructions
                .iter()
                .any(|i| matches!(i.op,Operation::Call{ref callee,..} if callee=="print"))
        );
        assert!(matches!(
            procedure.blocks["b0"].terminator,
            Terminator::Return { .. }
        ));
        assert!(!ir.source_map.is_empty());
    }
    #[test]
    fn lowers_control_flow_and_loops_to_blocks() {
        let ir=lower("fn count(n) { state i = 0; while i < n { i = i + 1; } if i > 2 { return true; } else { return false; } }").expect("lowered control flow");
        assert!(ir.procedures["proc:count"].blocks.len() >= 6);
    }
    #[test]
    fn rejects_unsupported_authority_declarations_with_location() {
        let error = lower("tool lookup using catalog.search")
            .expect_err("legacy authority syntax must fail closed");
        assert!(error.message().contains("unsupported top-level"));
        assert_eq!(error.span().line, 1);
    }
    #[test]
    fn rejects_duplicate_parameters_and_unbound_assignment() {
        assert!(
            lower("fn bad(x, x) {} ")
                .unwrap_err()
                .message()
                .contains("duplicate parameter")
        );
        assert!(
            lower("fn bad() { x = 2 }")
                .unwrap_err()
                .message()
                .contains("unknown or immutable")
        );
        assert!(
            lower("fn bad() { let x = 1; x = 2 }")
                .unwrap_err()
                .message()
                .contains("unknown or immutable")
        );
    }

    #[test]
    fn lowers_operator_precedence_and_top_level_execution() {
        let ir = lower("let answer = 1 + 2 * 3\nprint(answer)").expect("lowered top-level program");
        let instructions = &ir.procedures["proc:main"].blocks["b0"].instructions;
        let operators: Vec<_> = instructions
            .iter()
            .filter_map(|instruction| match &instruction.op {
                Operation::Binary { operator, .. } => Some(operator.as_str()),
                _ => None,
            })
            .collect();
        assert_eq!(operators, vec!["*", "+"]);
        assert_eq!(ir.procedures.len(), 1);
    }

    #[test]
    fn rejects_top_level_return() {
        let error = lower("return 1").expect_err("top-level return is invalid");
        assert!(error.message().contains("only valid inside a procedure"));
    }

    #[test]
    fn rejects_unknown_value_references() {
        let error = lower("fn bad() { return missing }").expect_err("unknown variable is invalid");
        assert!(error.message().contains("unknown binding `missing`"));
    }

    #[test]
    fn rejects_unresolved_procedure_calls() {
        let error =
            lower("fn bad() { return missing() }").expect_err("unresolved procedure is invalid");
        assert!(error.message().contains("unknown procedure `missing`"));
    }
}
