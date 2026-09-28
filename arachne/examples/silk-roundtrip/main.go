// Package main demonstrates the Arachne Silk protocol client.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/haha-systems/arachne2/internal/silk"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: silk-roundtrip <path-to-silk-binary>")
		os.Exit(2)
	}
	ctx := context.Background()
	client, err := silk.Start(ctx, silk.Command{Path: os.Args[1], Args: []string{"serve"}, Err: os.Stderr})
	fatalIf(err)
	defer func() { fatalIf(client.Close()) }()

	result, err := client.CreateSession(ctx, silk.SessionConfig{
		ID: "arachne-demo-session",
		HostFunctions: []silk.HostCapability{{
			Descriptor: silk.HostFunctionDescriptor{
				Name:        "demo.echo",
				Description: "Return the provided value to demonstrate an explicit host call",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"]}`),
				Effects:     []string{"host_read"},
				Authority:   "demo.echo",
			},
			Call: func(_ context.Context, arguments json.RawMessage) (json.RawMessage, error) {
				return append(json.RawMessage(nil), arguments...), nil
			},
		}},
		Grants: []silk.Grant{{Authority: "demo.echo", EffectCeiling: []string{"host_read"}}},
		Limits: silk.Limits{Fuel: 10000, CallDepth: 16, TimeoutMS: 5000},
	})
	fatalIf(err)

	_, err = client.LoadProgram(ctx, result.ID, `fn main(input) { return demo.echo(input); }`)
	fatalIf(err)
	run, err := client.RunProcedure(ctx, result.ID, "main", []json.RawMessage{json.RawMessage(`{"value":"hello from Arachne"}`)})
	fatalIf(err)
	fmt.Printf("result: %s\ntrace events: %d\n", run.Value, len(run.Trace))
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
