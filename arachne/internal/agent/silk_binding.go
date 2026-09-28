package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/haha-systems/arachne2/internal/cognition"
	"github.com/haha-systems/arachne2/internal/silk"
)

const (
	// SilkRunMessage requests a procedure invocation through the bound session.
	SilkRunMessage = "silk.procedure.run"
	// SilkResultMessage carries a procedure result or error and its execution trace.
	SilkResultMessage = "silk.procedure.result"
)

// SilkProcedureRequest selects a procedure already loaded into the bound session.
type SilkProcedureRequest struct {
	Procedure string            `json:"procedure"`
	Arguments []json.RawMessage `json:"arguments"`
}

// SilkProcedureResponse carries the public result and traces back to the requesting agent.
type SilkProcedureResponse struct {
	Value   json.RawMessage   `json:"value,omitempty"`
	Output  []string          `json:"output,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
	Trace   []json.RawMessage `json:"trace,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// SilkProcedureAgent routes explicit procedure messages through one public Silk session.
type SilkProcedureAgent struct {
	client    *silk.Client
	sessionID string
	events    *cognition.Spine
}

// NewSilkProcedureAgent binds an agent worker to an existing Silk client session.
func NewSilkProcedureAgent(client *silk.Client, sessionID string, events ...*cognition.Spine) (*SilkProcedureAgent, error) {
	if client == nil {
		return nil, errors.New("silk client must not be nil")
	}
	if sessionID == "" {
		return nil, errors.New("silk session ID must not be empty")
	}
	if len(events) > 1 {
		return nil, errors.New("at most one cognitive event spine may be supplied")
	}
	var spine *cognition.Spine
	if len(events) == 1 {
		spine = events[0]
	}
	return &SilkProcedureAgent{client: client, sessionID: sessionID, events: spine}, nil
}

// Run handles Silk procedure messages until cancellation or inbox closure.
func (a *SilkProcedureAgent) Run(ctx context.Context, inbox <-chan Message, sender Sender) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, open := <-inbox:
			if !open {
				return nil
			}
			if message.Kind != SilkRunMessage {
				continue
			}
			response, err := a.execute(ctx, message)
			if err != nil {
				return err
			}
			payload, err := json.Marshal(response)
			if err != nil {
				return fmt.Errorf("encode Silk procedure response: %w", err)
			}
			if err := sender.Send(ctx, message.From, SilkResultMessage, payload); err != nil {
				return fmt.Errorf("send Silk procedure result to %q: %w", message.From, err)
			}
		}
	}
}

func (a *SilkProcedureAgent) execute(ctx context.Context, message Message) (SilkProcedureResponse, error) {
	var request SilkProcedureRequest
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		return SilkProcedureResponse{Error: fmt.Sprintf("invalid Silk procedure request: %v", err)}, nil
	}
	if request.Procedure == "" {
		return SilkProcedureResponse{Error: "procedure name must not be empty"}, nil
	}
	var activationID string
	if a.events != nil {
		perception, err := a.events.Emit(ctx, cognition.Draft{
			AgentID: message.To, SessionID: a.sessionID, CorrelationID: message.CorrelationID,
			Kind: cognition.KindPerception, Payload: message.Payload,
		})
		if err != nil {
			return SilkProcedureResponse{}, fmt.Errorf("record Silk request perception: %w", err)
		}
		activationPayload, err := json.Marshal(map[string]string{
			"message_kind": SilkRunMessage,
			"procedure":    request.Procedure,
		})
		if err != nil {
			return SilkProcedureResponse{}, fmt.Errorf("encode Silk activation event: %w", err)
		}
		activation, err := a.events.Emit(ctx, cognition.Draft{
			AgentID: message.To, SessionID: a.sessionID, CorrelationID: message.CorrelationID,
			ParentEventIDs: []string{perception.EventID}, Kind: cognition.KindActivation,
			Payload: activationPayload,
		})
		if err != nil {
			return SilkProcedureResponse{}, fmt.Errorf("record Silk request activation: %w", err)
		}
		activationID = activation.EventID
	}
	result, err := a.client.RunProcedure(ctx, a.sessionID, request.Procedure, request.Arguments)
	response := SilkProcedureResponse{Value: result.Value, Output: result.Output, TraceID: result.TraceID, Trace: result.Trace}
	if a.events != nil {
		lastParentID := activationID
		for _, trace := range result.Trace {
			recorded, traceErr := a.events.RecordSilkTrace(ctx, message.To, a.sessionID, message.CorrelationID, []string{lastParentID}, trace)
			if traceErr != nil {
				return SilkProcedureResponse{}, fmt.Errorf("record Silk execution trace: %w", traceErr)
			}
			lastParentID = recorded.EventID
		}
		actionPayload, marshalErr := json.Marshal(map[string]any{
			"procedure": request.Procedure, "trace_id": result.TraceID,
			"value": result.Value, "output": result.Output,
			"error": errorText(err),
		})
		if marshalErr != nil {
			return SilkProcedureResponse{}, fmt.Errorf("encode cognitive action event: %w", marshalErr)
		}
		if _, emitErr := a.events.Emit(ctx, cognition.Draft{
			AgentID: message.To, SessionID: a.sessionID, CorrelationID: message.CorrelationID,
			ParentEventIDs: []string{lastParentID}, Kind: cognition.KindAction, Payload: actionPayload,
		}); emitErr != nil {
			return SilkProcedureResponse{}, fmt.Errorf("record Silk action: %w", emitErr)
		}
	}
	if err != nil {
		response.Error = err.Error()
	}
	return response, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
