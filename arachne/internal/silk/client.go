// Package silk implements Arachne's versioned stdio client for the Silk Runtime Protocol.
package silk

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	protocolVersion = "1.1"
	maxFrameBytes   = 16 << 20
)

// ErrReentrantCall reports a host callback that attempted to call the same Silk client.
var ErrReentrantCall error = ReentrantCallError{}

// ReentrantCallError identifies an unsupported call back into its active Silk client.
type ReentrantCallError struct{}

func (ReentrantCallError) Error() string {
	return "silk client calls from a host callback are unsupported"
}

// HostFunctionDescriptor declares the schema, effects, and authority for a host callback.
type HostFunctionDescriptor struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
	Effects     []string        `json:"effects"`
	Authority   string          `json:"authority"`
}

// HostFunction is the application callback invoked for an authorized nested host.call.
type HostFunction func(context.Context, json.RawMessage) (json.RawMessage, error)

// HostCapability keeps a host callback paired with the exact descriptor exposed to Silk.
type HostCapability struct {
	Descriptor HostFunctionDescriptor
	Call       HostFunction
}

// Grant bounds the effect set allowed for one authority identifier.
type Grant struct {
	Authority     string   `json:"authority"`
	EffectCeiling []string `json:"effect_ceiling"`
}

// Limits configures resource bounds for one Silk session.
type Limits struct {
	Fuel      uint64 `json:"fuel,omitempty"`
	CallDepth int    `json:"call_depth,omitempty"`
	TimeoutMS uint64 `json:"timeout_ms,omitempty"`
}

// SessionConfig creates an isolated session with explicit host capabilities and grants.
type SessionConfig struct {
	ID            string           `json:"session_id"`
	Input         json.RawMessage  `json:"input,omitempty"`
	HostFunctions []HostCapability `json:"-"`
	Grants        []Grant          `json:"grants"`
	Limits        Limits           `json:"limits"`
}

// SessionInfo is the runtime's accepted session configuration summary.
type SessionInfo struct {
	ID              string `json:"session_id"`
	ProtocolVersion string `json:"protocol_version"`
	CatalogDigest   string `json:"catalog_digest"`
	Limits          Limits `json:"limits"`
}

// RunResult includes the value, printed output, and ordered structured Silk trace.
type RunResult struct {
	Value   json.RawMessage   `json:"value"`
	Output  []string          `json:"output"`
	TraceID string            `json:"trace_id"`
	Trace   []json.RawMessage `json:"trace"`
}

// PreparedCandidate is Silk-validated executable content that is not admitted or retained yet.
type PreparedCandidate struct {
	Artifact       json.RawMessage `json:"artifact"`
	EntryProcedure string          `json:"entry_procedure"`
	RetentionState string          `json:"retention_state"`
}

// CandidateAdmission identifies an immutable revision admitted to Silk's registry.
type CandidateAdmission struct {
	Admission      string `json:"admission"`
	ProcedureID    string `json:"procedure_id"`
	RevisionDigest string `json:"revision_digest"`
	EntryProcedure string `json:"entry_procedure"`
	RetentionState string `json:"retention_state"`
}

// RegisteredRevision pins one retained artifact for execution through a session.
type RegisteredRevision struct {
	ProcedureID    string
	RevisionDigest string
}

// ProcedureCall selects a loaded session program or one exact retained registry revision.
type ProcedureCall struct {
	SessionID string
	Procedure string
	Arguments []json.RawMessage
	Retained  *RegisteredRevision
}

// Command starts one Silk subprocess and negotiates SRP 1.1 on its framed stdio stream.
type Command struct {
	Path string
	Args []string
	Env  []string
	Err  io.Writer
}

// Client serializes protocol calls over one Silk process and routes nested host calls.
type Client struct {
	gate            chan struct{}
	command         *exec.Cmd
	stdin           io.WriteCloser
	stdout          *bufio.Reader
	nextID          uint64
	sessions        map[string]map[string]HostCapability
	traces          map[string][]json.RawMessage
	traceSession    string
	closed          bool
	failed          error
	callbacksActive atomic.Int32
	terminateOnce   sync.Once
}

type callbackClientContextKey struct{}

// ProtocolError is a typed JSON-RPC failure returned by the Silk process.
type ProtocolError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("Silk protocol error %d: %s", e.Code, e.Message)
}

// Start launches the runtime and negotiates the version before returning a client.
func Start(ctx context.Context, command Command) (*Client, error) {
	if command.Path == "" {
		return nil, errors.New("silk command path must not be empty")
	}
	// #nosec G204 -- the embedding application supplies an executable path and argument vector; no shell is invoked.
	process := exec.CommandContext(ctx, command.Path, command.Args...)
	if len(command.Env) > 0 {
		process.Env = append(process.Environ(), command.Env...)
	}
	stdin, err := process.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open Silk stdin: %w", err)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open Silk stdout: %w", err)
	}
	process.Stderr = command.Err
	if err := process.Start(); err != nil {
		return nil, fmt.Errorf("start Silk: %w", err)
	}
	client := &Client{
		gate:     make(chan struct{}, 1),
		command:  process,
		stdin:    stdin,
		stdout:   bufio.NewReader(stdout),
		sessions: make(map[string]map[string]HostCapability),
		traces:   make(map[string][]json.RawMessage),
	}
	var result struct {
		ProtocolVersion string `json:"protocol_version"`
	}
	if err := client.call(ctx, "initialize", map[string]any{"protocol_version": protocolVersion}, &result); err != nil {
		_ = client.Close()
		return nil, err
	}
	if result.ProtocolVersion != protocolVersion {
		_ = client.Close()
		return nil, fmt.Errorf("silk negotiated unexpected protocol %q", result.ProtocolVersion)
	}
	return client, nil
}

// CreateSession installs explicit callbacks and returns the runtime's accepted catalog digest.
func (c *Client) CreateSession(ctx context.Context, config SessionConfig) (SessionInfo, error) {
	if err := c.rejectReentrant(ctx); err != nil {
		return SessionInfo{}, err
	}
	if err := c.lock(ctx); err != nil {
		return SessionInfo{}, err
	}
	defer c.unlock()
	if config.ID == "" {
		return SessionInfo{}, errors.New("session ID must not be empty")
	}
	if _, exists := c.sessions[config.ID]; exists {
		return SessionInfo{}, fmt.Errorf("session %q already exists", config.ID)
	}
	hostFunctions := make([]HostFunctionDescriptor, 0, len(config.HostFunctions))
	callbacks := make(map[string]HostCapability, len(config.HostFunctions))
	for _, capability := range config.HostFunctions {
		if capability.Call == nil {
			return SessionInfo{}, fmt.Errorf("host function %q has no callback", capability.Descriptor.Name)
		}
		if _, duplicate := callbacks[capability.Descriptor.Name]; duplicate {
			return SessionInfo{}, fmt.Errorf("duplicate host function %q", capability.Descriptor.Name)
		}
		callbacks[capability.Descriptor.Name] = capability
		hostFunctions = append(hostFunctions, capability.Descriptor)
	}
	c.sessions[config.ID] = callbacks
	params := map[string]any{
		"session_id":     config.ID,
		"input":          config.Input,
		"host_functions": hostFunctions,
		"grants":         config.Grants,
		"limits":         config.Limits,
		"trace":          map[string]string{"level": "standard"},
	}
	var info SessionInfo
	if err := c.callLocked(ctx, "session.create", params, &info); err != nil {
		delete(c.sessions, config.ID)
		return SessionInfo{}, err
	}
	return info, nil
}

// LoadProgram parses source in a session without executing it.
func (c *Client) LoadProgram(ctx context.Context, sessionID, source string) (string, error) {
	var result struct {
		ProgramID string `json:"program_id"`
	}
	if err := c.call(ctx, "program.load", map[string]any{"session_id": sessionID, "source": source}, &result); err != nil {
		return "", err
	}
	return result.ProgramID, nil
}

// PrepareCandidate performs Silk lowering and effect/authority analysis without admission.
func (c *Client) PrepareCandidate(ctx context.Context, sessionID, source string, candidateRequest json.RawMessage) (PreparedCandidate, error) {
	var result PreparedCandidate
	err := c.call(ctx, "candidate.prepare", map[string]any{
		"session_id": sessionID, "source": source, "candidate_request": candidateRequest,
	}, &result)
	return result, err
}

// AdmitCandidate inserts one exact prepared artifact as an unretained registry candidate.
func (c *Client) AdmitCandidate(ctx context.Context, candidate PreparedCandidate) (CandidateAdmission, error) {
	var result CandidateAdmission
	err := c.call(ctx, "registry.admit", map[string]any{
		"artifact": candidate.Artifact, "entry_procedure": candidate.EntryProcedure,
	}, &result)
	return result, err
}

// RetainCandidate applies a revision-bound retention decision after Arachne governance.
func (c *Client) RetainCandidate(ctx context.Context, procedureID, revisionDigest string, decision json.RawMessage) error {
	return c.call(ctx, "registry.retain", map[string]any{
		"procedure_id": procedureID, "revision_digest": revisionDigest, "decision": decision,
	}, nil)
}

// Run invokes a loaded program or exact retained entry through the current session's checks.
func (c *Client) Run(ctx context.Context, call ProcedureCall) (RunResult, error) {
	params := map[string]any{
		"session_id": call.SessionID, "procedure": call.Procedure, "arguments": call.Arguments,
	}
	method := "procedure.run"
	if call.Retained != nil {
		method = "registry.run"
		params["procedure_id"] = call.Retained.ProcedureID
		params["revision_digest"] = call.Retained.RevisionDigest
	}
	return c.runWithTrace(ctx, method, call.SessionID, params)
}

// RunProcedure invokes one named procedure and returns its value and trace.
func (c *Client) RunProcedure(ctx context.Context, sessionID, procedure string, arguments []json.RawMessage) (RunResult, error) {
	return c.Run(ctx, ProcedureCall{
		SessionID: sessionID, Procedure: procedure, Arguments: arguments,
	})
}

func (c *Client) runWithTrace(ctx context.Context, method, sessionID string, params any) (RunResult, error) {
	if err := c.rejectReentrant(ctx); err != nil {
		return RunResult{}, err
	}
	if err := c.lock(ctx); err != nil {
		return RunResult{}, err
	}
	defer c.unlock()
	c.traces[sessionID] = nil
	previousTraceSession := c.traceSession
	c.traceSession = sessionID
	defer func() { c.traceSession = previousTraceSession }()
	var result RunResult
	err := c.callLocked(ctx, method, params, &result)
	result.Trace = append([]json.RawMessage(nil), c.traces[sessionID]...)
	return result, err
}

// CloseSession discards one isolated runtime session.
func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
	if err := c.rejectReentrant(ctx); err != nil {
		return err
	}
	if err := c.lock(ctx); err != nil {
		return err
	}
	defer c.unlock()
	err := c.callLocked(ctx, "session.close", map[string]string{"session_id": sessionID}, nil)
	if err != nil {
		return err
	}
	delete(c.sessions, sessionID)
	delete(c.traces, sessionID)
	return nil
}

// Close shuts down the subprocess after closing every remaining session.
func (c *Client) Close() error {
	if c.callbacksActive.Load() > 0 {
		return ErrReentrantCall
	}
	if err := c.lock(context.Background()); err != nil {
		return err
	}
	defer c.unlock()
	if c.closed {
		return nil
	}
	ids := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var closeErr error
	for _, id := range ids {
		if c.failed == nil {
			if err := c.callLocked(context.Background(), "session.close", map[string]string{"session_id": id}, nil); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		delete(c.sessions, id)
		delete(c.traces, id)
	}
	c.closed = true
	_ = c.stdin.Close()
	waitErr := c.command.Wait()
	if closeErr != nil {
		return closeErr
	}
	return waitErr
}

func (c *Client) call(ctx context.Context, method string, params any, target any) error {
	if err := c.rejectReentrant(ctx); err != nil {
		return err
	}
	if err := c.lock(ctx); err != nil {
		return err
	}
	defer c.unlock()
	return c.callLocked(ctx, method, params, target)
}

func (c *Client) callLocked(ctx context.Context, method string, params any, target any) error {
	if err := c.rejectReentrant(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.failed != nil {
		return fmt.Errorf("silk client is unusable after a transport failure: %w", c.failed)
	}
	if c.closed {
		return errors.New("silk client is closed")
	}
	stopCancellation := context.AfterFunc(ctx, c.terminateProcess)
	defer stopCancellation()
	c.nextID++
	id := c.nextID
	if err := writeFrame(c.stdin, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return c.failLocked(ctxErr)
		}
		return c.failLocked(fmt.Errorf("write Silk request: %w", err))
	}
	for {
		frame, err := readFrame(c.stdout)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return c.failLocked(ctxErr)
			}
			return c.failLocked(fmt.Errorf("read Silk response: %w", err))
		}
		var envelope struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
			Result  json.RawMessage `json:"result"`
			Error   *ProtocolError  `json:"error"`
		}
		if err := json.Unmarshal(frame, &envelope); err != nil {
			return c.failLocked(fmt.Errorf("decode Silk frame: %w", err))
		}
		if envelope.JSONRPC != "2.0" {
			return c.failLocked(fmt.Errorf("invalid Silk JSON-RPC version %q", envelope.JSONRPC))
		}
		if envelope.Method != "" {
			switch envelope.Method {
			case "host.call":
				if len(envelope.ID) == 0 || len(envelope.Params) == 0 {
					return c.failLocked(errors.New("malformed Silk host.call request"))
				}
				if err := c.handleHostCall(ctx, envelope.ID, envelope.Params); err != nil {
					if ctxErr := ctx.Err(); ctxErr != nil {
						return c.failLocked(ctxErr)
					}
					return c.failLocked(err)
				}
				continue
			case "trace.emit":
				if len(envelope.ID) != 0 || len(envelope.Params) == 0 {
					return c.failLocked(errors.New("malformed Silk trace.emit notification"))
				}
				var notification struct {
					SessionID string          `json:"session_id"`
					Event     json.RawMessage `json:"event"`
				}
				if err := json.Unmarshal(envelope.Params, &notification); err != nil {
					return c.failLocked(fmt.Errorf("decode trace notification: %w", err))
				}
				if notification.SessionID == "" || !json.Valid(notification.Event) || notification.SessionID != c.traceSession {
					return c.failLocked(fmt.Errorf("silk trace notification has invalid session correlation %q", notification.SessionID))
				}
				c.traces[notification.SessionID] = append(c.traces[notification.SessionID], append(json.RawMessage(nil), notification.Event...))
				continue
			default:
				// JSON-RPC notifications have no response ID and may be ignored when
				// this client does not implement their optional method.
				if len(envelope.ID) == 0 {
					continue
				}
				return c.failLocked(fmt.Errorf("unexpected Silk server request %q", envelope.Method))
			}
		}
		if len(envelope.ID) == 0 || string(envelope.ID) != fmt.Sprint(id) {
			return c.failLocked(fmt.Errorf("unexpected Silk response ID %s for request %d", envelope.ID, id))
		}
		if (len(envelope.Result) == 0) == (envelope.Error == nil) {
			return c.failLocked(errors.New("silk response must contain exactly one of result or error"))
		}
		if !stopCancellation() {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return c.failLocked(ctxErr)
			}
		}
		if envelope.Error != nil {
			return envelope.Error
		}
		if target != nil {
			if err := json.Unmarshal(envelope.Result, target); err != nil {
				return c.failLocked(fmt.Errorf("decode Silk result: %w", err))
			}
		}
		return nil
	}
}

func (c *Client) handleHostCall(ctx context.Context, id json.RawMessage, params json.RawMessage) error {
	var call struct {
		SessionID  string          `json:"session_id"`
		Function   string          `json:"function"`
		Arguments  json.RawMessage `json:"arguments"`
		DeadlineMS uint64          `json:"deadline_ms"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return c.writeHostError(id, -32602, "invalid host.call parameters")
	}
	capability, ok := c.sessions[call.SessionID][call.Function]
	if !ok {
		return c.writeHostError(id, -32010, "host function is not registered")
	}
	callContext := context.WithValue(ctx, callbackClientContextKey{}, c)
	cancel := func() {}
	if call.DeadlineMS > 0 {
		const maxDeadlineMillis = uint64((1<<63 - 1) / int64(time.Millisecond))
		if call.DeadlineMS > maxDeadlineMillis {
			return c.writeHostError(id, -32602, "host call deadline is out of range")
		}
		// #nosec G115 -- the preceding bound ensures the millisecond duration fits in int64.
		callContext, cancel = context.WithTimeout(callContext, time.Duration(call.DeadlineMS)*time.Millisecond)
	}
	defer cancel()
	type callbackResult struct {
		value json.RawMessage
		err   error
	}
	callbackResults := make(chan callbackResult, 1)
	c.callbacksActive.Add(1)
	go func() {
		callback := callbackResult{}
		defer func() {
			if panicValue := recover(); panicValue != nil {
				callback.err = fmt.Errorf("host callback panicked: %v", panicValue)
			}
			c.callbacksActive.Add(-1)
			callbackResults <- callback
		}()
		callback.value, callback.err = capability.Call(callContext, call.Arguments)
	}()
	var result json.RawMessage
	var err error
	select {
	case callback := <-callbackResults:
		result, err = callback.value, callback.err
	case <-callContext.Done():
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return c.writeHostError(id, -32011, "host callback deadline exceeded")
	}
	if err != nil {
		return c.writeHostError(id, -32011, err.Error())
	}
	if len(result) == 0 {
		result = json.RawMessage("null")
	}
	if !json.Valid(result) {
		return c.writeHostError(id, -32603, "host callback returned invalid JSON")
	}
	return writeFrame(c.stdin, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (c *Client) rejectReentrant(ctx context.Context) error {
	if activeClient, _ := ctx.Value(callbackClientContextKey{}).(*Client); activeClient == c {
		return ErrReentrantCall
	}
	return nil
}

func (c *Client) lock(ctx context.Context) error {
	select {
	case c.gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			c.unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) unlock() { <-c.gate }

func (c *Client) failLocked(err error) error {
	if c.failed == nil {
		c.failed = err
	}
	c.terminateProcess()
	return err
}

func (c *Client) terminateProcess() {
	c.terminateOnce.Do(func() {
		_ = c.stdin.Close()
		if c.command.Process != nil {
			_ = c.command.Process.Kill()
		}
	})
}

func (c *Client) writeHostError(id json.RawMessage, code int64, message string) error {
	return writeFrame(c.stdin, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}

func writeFrame(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > maxFrameBytes {
		return fmt.Errorf("silk frame size %d is out of bounds", len(data))
	}
	var prefix [4]byte
	// #nosec G115 -- len(data) is bounded by maxFrameBytes above (16 MiB).
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	if err := writeAll(writer, prefix[:]); err != nil {
		return err
	}
	err = writeAll(writer, data)
	return err
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func readFrame(reader io.Reader) ([]byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(prefix[:])
	if length == 0 || length > maxFrameBytes {
		return nil, fmt.Errorf("invalid Silk frame length %d", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, err
	}
	return data, nil
}
