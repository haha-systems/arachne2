// Package agent provides bounded message delivery and supervision for organism workers.
package agent

import (
	"context"
	"encoding/json"
)

// Message is a point-to-point event delivered between supervised agents.
type Message struct {
	Sequence      uint64          `json:"sequence"`
	CorrelationID string          `json:"correlation_id"`
	From          string          `json:"from"`
	To            string          `json:"to"`
	Kind          string          `json:"kind"`
	Payload       json.RawMessage `json:"payload"`
}

// Sender publishes messages using the identity of the sending agent.
type Sender interface {
	Send(ctx context.Context, to, kind string, payload json.RawMessage) error
}

// Agent runs until it returns or its context is cancelled.
type Agent interface {
	Run(ctx context.Context, inbox <-chan Message, sender Sender) error
}

// Func adapts a function to the Agent interface.
type Func func(context.Context, <-chan Message, Sender) error

// Run implements Agent.
func (f Func) Run(ctx context.Context, inbox <-chan Message, sender Sender) error {
	return f(ctx, inbox, sender)
}
