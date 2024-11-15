package example

import (
	"context"
	"testing"

	example "github.com/bergundy/protoc-gen-nexus-temporal/gen/example/v1"
	"github.com/bergundy/protoc-gen-nexus-temporal/gen/example/v1/examplev1nexustemporal"
	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/stretchr/testify/require"
	nexuspb "go.temporal.io/api/nexus/v1"
	operatorservice "go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var oneWayClient = examplev1nexustemporal.NewOneWayNexusClient("example-endpoint")
var twoWayClient = examplev1nexustemporal.NewTwoWayNexusClient("example-endpoint")

func CallerWorkflow(ctx workflow.Context) error {
	output, err := oneWayClient.NoInput(ctx, workflow.NexusOperationOptions{})
	if err != nil {
		return err
	}
	output, err = twoWayClient.Example(ctx, &example.ExampleInput{Foo: output.Foo}, workflow.NexusOperationOptions{})
	if err != nil {
		return err
	}

	return oneWayClient.NoOutput(ctx, &example.ExampleInput{Foo: output.Foo}, workflow.NexusOperationOptions{})
}

func CallerWorkflowAsync(ctx workflow.Context) error {
	oneWayClient := examplev1nexustemporal.NewOneWayNexusClient("example-endpoint")
	outputFuture := oneWayClient.NoInputAsync(ctx, workflow.NexusOperationOptions{})
	output, err := outputFuture.GetTyped(ctx)
	if err != nil {
		return err
	}

	exampleFuture := twoWayClient.ExampleAsync(ctx, &example.ExampleInput{Foo: output.Foo}, workflow.NexusOperationOptions{})
	output, err = exampleFuture.GetTyped(ctx)
	if err != nil {
		return err
	}

	noValueFuture := oneWayClient.NoOutputAsync(ctx, &example.ExampleInput{Foo: output.Foo}, workflow.NexusOperationOptions{})
	return noValueFuture.Get(ctx, nil)
}

func TwoWayWorkflow(ctx workflow.Context, input *example.ExampleInput) (*example.ExampleOutput, error) {
	return &example.ExampleOutput{Foo: input.Foo}, nil
}

type twoWayHandler struct {
}

// CreateOrder implements oms.OrdersNexusServiceHandler.
func (*twoWayHandler) Example(name string) nexus.Operation[*example.ExampleInput, *example.ExampleOutput] {
	return temporalnexus.NewWorkflowRunOperation(
		name,
		TwoWayWorkflow,
		func(ctx context.Context, input *example.ExampleInput, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			return client.StartWorkflowOptions{
				ID: options.RequestID, // Do not use this in production code.
			}, nil
		})
}

type oneWayHandler struct {
}

// NoInput implements example.OneWayNexusServiceHandler.
func (*oneWayHandler) NoInput(name string) nexus.Operation[nexus.NoValue, *example.ExampleOutput] {
	return nexus.NewSyncOperation(name, func(ctx context.Context, e nexus.NoValue, soo nexus.StartOperationOptions) (*example.ExampleOutput, error) {
		return &example.ExampleOutput{Foo: "bar"}, nil
	})
}

// NoOutput implements example.OneWayNexusServiceHandler.
func (*oneWayHandler) NoOutput(name string) nexus.Operation[*example.ExampleInput, nexus.NoValue] {
	return nexus.NewSyncOperation(name, func(ctx context.Context, input *example.ExampleInput, soo nexus.StartOperationOptions) (nexus.NoValue, error) {
		if input.Foo != "bar" {
			return nil, nexus.HandlerErrorf(nexus.HandlerErrorTypeBadRequest, "input.Foo != bar")
		}
		return nil, nil
	})
}

func TestE2E(t *testing.T) {
	ctx := context.Background()
	srv, err := testsuite.StartDevServer(ctx, testsuite.DevServerOptions{
		ClientOptions: &client.Options{
			HostPort: "0.0.0.0:7233",
		},
		EnableUI: true,
		ExtraArgs: []string{
			"--http-port", "7243",
			"--dynamic-config-value", "system.enableNexus=true",
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Stop()) })

	c, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	require.NoError(t, err)
	w := worker.New(c, "example", worker.Options{})
	examplev1nexustemporal.RegisterTwoWayNexusServiceHandler(w, &twoWayHandler{})
	examplev1nexustemporal.RegisterOneWayNexusServiceHandler(w, &oneWayHandler{})
	w.RegisterWorkflow(CallerWorkflow)
	w.RegisterWorkflow(CallerWorkflowAsync)
	w.RegisterWorkflow(TwoWayWorkflow)

	_, err = c.OperatorService().CreateNexusEndpoint(ctx, &operatorservice.CreateNexusEndpointRequest{
		Spec: &nexuspb.EndpointSpec{
			Name: "example-endpoint",
			Target: &nexuspb.EndpointTarget{
				Variant: &nexuspb.EndpointTarget_Worker_{
					Worker: &nexuspb.EndpointTarget_Worker{
						Namespace: "default",
						TaskQueue: "example",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, w.Start())
	t.Cleanup(w.Stop)

	fut, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue: "example",
	}, CallerWorkflow)
	require.NoError(t, err)
	require.NoError(t, fut.Get(ctx, nil))

	fut, err = c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue: "example",
	}, CallerWorkflowAsync)
	require.NoError(t, err)
	require.NoError(t, fut.Get(ctx, nil))
}
