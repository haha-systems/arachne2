package agent

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSupervisorDeliversMessagesInRecipientOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	supervisor, err := NewSupervisor(4)
	if err != nil {
		t.Fatalf("create supervisor: %v", err)
	}
	received := make(chan []Message, 1)
	if err := supervisor.Register("receiver", Func(func(ctx context.Context, inbox <-chan Message, _ Sender) error {
		messages := make([]Message, 0, 3)
		for len(messages) < 3 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case message := <-inbox:
				messages = append(messages, message)
			}
		}
		received <- messages
		return nil
	})); err != nil {
		t.Fatalf("register receiver: %v", err)
	}
	if err := supervisor.Register("sender", Func(func(ctx context.Context, _ <-chan Message, sender Sender) error {
		for _, payload := range []string{`{"n":1}`, `{"n":2}`, `{"n":3}`} {
			if err := sender.Send(ctx, "receiver", "test.value", json.RawMessage(payload)); err != nil {
				return err
			}
		}
		return nil
	})); err != nil {
		t.Fatalf("register sender: %v", err)
	}
	if err := supervisor.Start(ctx); err != nil {
		t.Fatalf("start supervisor: %v", err)
	}

	messages := <-received
	sequences := []uint64{messages[0].Sequence, messages[1].Sequence, messages[2].Sequence}
	if !reflect.DeepEqual(sequences, []uint64{1, 2, 3}) {
		t.Fatalf("unexpected message order: %v", sequences)
	}
	if messages[0].From != "sender" || messages[0].To != "receiver" {
		t.Fatalf("unexpected message attribution: %+v", messages[0])
	}
	if err := supervisor.Stop(ctx); err != nil {
		t.Fatalf("stop supervisor: %v", err)
	}
}

func TestAgentFailureCancelsAndJoinsOtherAgents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	supervisor, err := NewSupervisor(1)
	if err != nil {
		t.Fatalf("create supervisor: %v", err)
	}
	wantErr := errors.New("agent failed")
	failureStarted := make(chan struct{})
	joined := make(chan struct{})
	if err := supervisor.Register("failing", Func(func(context.Context, <-chan Message, Sender) error {
		close(failureStarted)
		return wantErr
	})); err != nil {
		t.Fatalf("register failing agent: %v", err)
	}
	if err := supervisor.Register("waiting", Func(func(ctx context.Context, _ <-chan Message, _ Sender) error {
		<-ctx.Done()
		close(joined)
		return ctx.Err()
	})); err != nil {
		t.Fatalf("register waiting agent: %v", err)
	}
	if err := supervisor.Start(ctx); err != nil {
		t.Fatalf("start supervisor: %v", err)
	}

	<-failureStarted
	if err := supervisor.Wait(ctx); err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected agent failure from wait, got %v", err)
	}
	select {
	case <-joined:
	default:
		t.Fatal("waiting agent was not joined after failure")
	}
}

func TestSendAfterStopFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	supervisor, err := NewSupervisor(1)
	if err != nil {
		t.Fatalf("create supervisor: %v", err)
	}
	if err := supervisor.Register("worker", Func(func(ctx context.Context, _ <-chan Message, _ Sender) error {
		<-ctx.Done()
		return ctx.Err()
	})); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	if err := supervisor.Start(ctx); err != nil {
		t.Fatalf("start supervisor: %v", err)
	}
	if err := supervisor.Stop(ctx); err != nil {
		t.Fatalf("stop supervisor: %v", err)
	}
	if err := supervisor.Send(ctx, "external", "worker", "test.value", json.RawMessage(`{}`)); !errors.Is(err, ErrRouterClosed) {
		t.Fatalf("expected closed router error, got %v", err)
	}
}

func TestSendRejectsInvalidJSONPayload(t *testing.T) {
	ctx := context.Background()
	supervisor, err := NewSupervisor(1)
	if err != nil {
		t.Fatalf("create supervisor: %v", err)
	}
	if err := supervisor.Register("worker", Func(func(ctx context.Context, _ <-chan Message, _ Sender) error {
		<-ctx.Done()
		return ctx.Err()
	})); err != nil {
		t.Fatalf("register worker: %v", err)
	}
	if err := supervisor.Start(ctx); err != nil {
		t.Fatalf("start supervisor: %v", err)
	}

	if err := supervisor.Send(ctx, "external", "worker", "test.value", json.RawMessage("not-json")); err == nil {
		t.Fatal("expected invalid JSON payload to be rejected")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := supervisor.Stop(stopCtx); err != nil {
		t.Fatalf("stop supervisor: %v", err)
	}
}
