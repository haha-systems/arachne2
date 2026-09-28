package silk

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

const testPeerEnv = "ARACHNE_SILK_PROTOCOL_PEER"

type peerSession struct {
	functions []HostFunctionDescriptor
	grants    []Grant
}

func TestSilkProtocolPeer(_ *testing.T) {
	if os.Getenv(testPeerEnv) != "1" {
		return
	}
	if err := serveTestPeer(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func TestClientKeepsSilkCallbacksAndTracesSessionScoped(t *testing.T) {
	ctx := context.Background()
	client, err := Start(ctx, Command{
		Path: os.Args[0], Args: []string{"-test.run=^TestSilkProtocolPeer$"},
		Env: []string{testPeerEnv + "=1"}, Err: io.Discard,
	})
	if err != nil {
		t.Fatalf("start protocol peer: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	calls := map[string]int{}
	capability := func(sessionID string) HostCapability {
		return HostCapability{
			Descriptor: HostFunctionDescriptor{
				Name: "memory.write", Authority: "memory.write", Effects: []string{"memory_write"},
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
			Call: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
				calls[sessionID]++
				return json.RawMessage(`{"accepted":true}`), nil
			},
		}
	}
	grant := Grant{Authority: "memory.write", EffectCeiling: []string{"memory_write"}}
	for _, session := range []struct {
		id     string
		grants []Grant
	}{
		{id: "session-a", grants: []Grant{grant}},
		{id: "session-b", grants: []Grant{grant}},
		{id: "session-denied"},
	} {
		if _, err := client.CreateSession(ctx, SessionConfig{
			ID: session.id, HostFunctions: []HostCapability{capability(session.id)}, Grants: session.grants,
		}); err != nil {
			t.Fatalf("create %s: %v", session.id, err)
		}
	}

	for _, sessionID := range []string{"session-a", "session-b"} {
		result, err := client.RunProcedure(ctx, sessionID, "invoke", nil)
		if err != nil {
			t.Fatalf("run %s: %v", sessionID, err)
		}
		if calls[sessionID] != 1 || string(result.Value) != `{"accepted":true}` {
			t.Fatalf("session %s callback/result = %d/%s", sessionID, calls[sessionID], result.Value)
		}
		if result.TraceID != "trace-"+sessionID || len(result.Trace) != 1 {
			t.Fatalf("session %s trace envelope = %+v", sessionID, result)
		}
		var trace struct {
			TraceID       string `json:"trace_id"`
			CorrelationID string `json:"correlation_id"`
			Procedure     string `json:"procedure"`
		}
		if err := json.Unmarshal(result.Trace[0], &trace); err != nil {
			t.Fatal(err)
		}
		if trace.TraceID != result.TraceID || trace.CorrelationID != "correlation-"+sessionID || trace.Procedure != "invoke" {
			t.Fatalf("trace was cross-correlated: session=%s trace=%+v", sessionID, trace)
		}
	}
	if calls["session-a"] != 1 || calls["session-b"] != 1 || calls["session-denied"] != 0 {
		t.Fatalf("callbacks crossed sessions: %v", calls)
	}

	if _, err := client.RunProcedure(ctx, "session-denied", "invoke", nil); err == nil {
		t.Fatal("Silk peer accepted an ungranted host call")
	} else {
		var protocolError *ProtocolError
		if !errors.As(err, &protocolError) || protocolError.Code != -32011 {
			t.Fatalf("ungranted call error = %v, want protocol denial", err)
		}
	}
	if calls["session-denied"] != 0 {
		t.Fatalf("ungranted call dispatched callback %d times", calls["session-denied"])
	}

	if err := client.CloseSession(ctx, "session-a"); err != nil {
		t.Fatalf("close session: %v", err)
	}
	err = client.handleHostCall(ctx, json.RawMessage(`"late-call"`), json.RawMessage(`{
		"session_id":"session-a","function":"memory.write","arguments":{},"deadline_ms":0
	}`))
	if err != nil {
		t.Fatalf("respond to post-close host call: %v", err)
	}
	if calls["session-a"] != 1 {
		t.Fatalf("closed session callback remained accessible: %v", calls)
	}
	if _, err := client.RunProcedure(ctx, "session-b", "invoke", nil); err != nil {
		t.Fatalf("closing one session affected another: %v", err)
	}
	if calls["session-b"] != 2 {
		t.Fatalf("remaining session callback count = %d, want 2", calls["session-b"])
	}
}

func serveTestPeer(input io.Reader, output io.Writer) error {
	reader := bufio.NewReader(input)
	sessions := make(map[string]peerSession)
	for {
		frame, err := readFrame(reader)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read test-peer request: %w", err)
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(frame, &request); err != nil {
			return fmt.Errorf("decode test-peer request: %w", err)
		}
		switch request.Method {
		case "initialize":
			if err := writeFrame(output, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{"protocol_version": protocolVersion}}); err != nil {
				return err
			}
		case "session.create":
			var params struct {
				ID            string                   `json:"session_id"`
				HostFunctions []HostFunctionDescriptor `json:"host_functions"`
				Grants        []Grant                  `json:"grants"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return err
			}
			sessions[params.ID] = peerSession{functions: params.HostFunctions, grants: params.Grants}
			result := SessionInfo{ID: params.ID, ProtocolVersion: protocolVersion, CatalogDigest: "test-catalog"}
			if err := writeFrame(output, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
				return err
			}
		case "session.close":
			var params struct {
				ID string `json:"session_id"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return err
			}
			delete(sessions, params.ID)
			if err := writeFrame(output, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": map[string]any{}}); err != nil {
				return err
			}
		case "procedure.run":
			var params struct {
				SessionID string `json:"session_id"`
				Procedure string `json:"procedure"`
			}
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return err
			}
			if !peerAllows(sessions[params.SessionID], "memory.write") {
				if err := writeFrame(output, map[string]any{
					"jsonrpc": "2.0", "id": request.ID,
					"error": map[string]any{"code": -32011, "message": "procedure execution failed"},
				}); err != nil {
					return err
				}
				continue
			}
			hostID := json.RawMessage(`"host-` + params.SessionID + `"`)
			if err := writeFrame(output, map[string]any{
				"jsonrpc": "2.0", "id": hostID, "method": "host.call",
				"params": map[string]any{
					"session_id": params.SessionID, "function": "memory.write",
					"arguments": map[string]string{"session": params.SessionID}, "deadline_ms": 1000,
				},
			}); err != nil {
				return err
			}
			hostResponse, err := readFrame(reader)
			if err != nil {
				return fmt.Errorf("read host callback response: %w", err)
			}
			var hostEnvelope struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(hostResponse, &hostEnvelope); err != nil {
				return err
			}
			if string(hostEnvelope.ID) != string(hostID) {
				return fmt.Errorf("host callback response ID %s does not match %s", hostEnvelope.ID, hostID)
			}
			if len(hostEnvelope.Error) != 0 {
				return writeFrame(output, map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": hostEnvelope.Error})
			}
			trace := map[string]any{
				"trace_id": "trace-" + params.SessionID, "sequence": 1,
				"correlation_id":        "correlation-" + params.SessionID,
				"parent_correlation_id": "host-" + params.SessionID,
				"procedure":             params.Procedure,
				"event":                 map[string]string{"kind": "host.call"},
			}
			if err := writeFrame(output, map[string]any{
				"jsonrpc": "2.0", "method": "trace.emit",
				"params": map[string]any{"session_id": params.SessionID, "event": trace},
			}); err != nil {
				return err
			}
			result := RunResult{Value: hostEnvelope.Result, TraceID: "trace-" + params.SessionID}
			if err := writeFrame(output, map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
				return err
			}
		default:
			if request.Method == "" {
				continue
			}
			return fmt.Errorf("unexpected test-peer method %q", request.Method)
		}
	}
}

func peerAllows(session peerSession, function string) bool {
	var descriptor *HostFunctionDescriptor
	for index := range session.functions {
		if session.functions[index].Name == function {
			descriptor = &session.functions[index]
			break
		}
	}
	if descriptor == nil {
		return false
	}
	for _, grant := range session.grants {
		if grant.Authority != descriptor.Authority {
			continue
		}
		covered := true
		for _, effect := range descriptor.Effects {
			if !strings.Contains("\x00"+strings.Join(grant.EffectCeiling, "\x00")+"\x00", "\x00"+effect+"\x00") {
				covered = false
				break
			}
		}
		if covered {
			return true
		}
	}
	return false
}
