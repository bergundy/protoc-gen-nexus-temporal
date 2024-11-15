# protoc-gen-nexus-temporal

A Protobuf plugin for generating Temporal Nexus code.

**⚠️ EXPERIMENTAL: Generated code structure is subject to change as feedback is collected. ⚠️**

Supported languages:

- Golang
- Java (TBD)

## Installation

### From GitHub releases (recommended)

1. Download an archive from the [latest release](https://github.com/bergundy/protoc-gen-nexus-temporal/releases/latest).
2. Extract and add to your system's path.

### Using go install

```
go install github.com/bergundy/protoc-gen-nexus-temporal/cmd/protoc-gen-nexus-temporal@latest
```

## Usage

### Create a proto file

> NOTE: the directory structure here determines the directory structure of the generated files.

`example/v1/service.proto`

```protobuf
syntax="proto3";

package example.v1;

option go_package = "github.com/bergundy/greet-nexus-example/gen/example/v1;example";

message GreetInput {
  string name = 1;
}

message GreetOutput {
  string greeting = 1;
}

service Greeting {
  rpc Greet(GreetInput) returns (GreetOutput) {
  }
}
```

### Create `buf` config files

> NOTE: Alternatively you may use protoc directly.

`buf.yaml`

```yaml
version: v2
modules:
  - path: .
deps:
  - buf.build/bergundy/protoc-gen-nexus-temporal
lint:
  use:
    - BASIC
  except:
    - FIELD_NOT_REQUIRED
    - PACKAGE_NO_IMPORT_CYCLE
breaking:
  use:
    - FILE
  except:
    - EXTENSION_NO_DELETE
    - FIELD_SAME_DEFAULT
```

`buf.gen.yaml`

```yaml
version: v2
clean: true
managed:
  enabled: true
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt:
      - paths=source_relative
  - local: protoc-gen-nexus-temporal
    out: gen
    strategy: all
    opt:
      - paths=source_relative
      - lang=go
```

### Generate code 

```
buf generate
```

### Implement a service handler and register it with a Temporal worker

```go
package main

import (
	"context"

	example "github.com/bergundy/greet-nexus-example/gen/example/v1"
	"github.com/bergundy/greet-nexus-example/gen/example/v1/examplev1nexustemporal"
	"github.com/nexus-rpc/sdk-go/nexus"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporalnexus"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func GreetWorkflow(ctx workflow.Context, input *example.GreetInput) (*example.GreetOutput, error) {
	return &example.GreetOutput{Greeting: "Hello " + input.Name}, nil
}

type greetingHandler struct {
}

func (*greetingHandler) Greet(name string) nexus.Operation[*example.GreetInput, *example.GreetOutput] {
	return temporalnexus.NewWorkflowRunOperation(
		// The name of the Greet operation as defined in the proto.
		name,
		// Workflow to expose as the operation.
		// Input must match the operation input using this builder. See `NewWorkflowRunOperationWithOptions` for
		// exposing workflows with alternative signatures.
		GreetWorkflow,
		func(ctx context.Context, input *example.GreetInput, options nexus.StartOperationOptions) (client.StartWorkflowOptions, error) {
			return client.StartWorkflowOptions{
				ID: meaningfulBusinessID(input),
			}, nil
		})
}

func main() {
	c, _ := client.Dial(client.Options{HostPort: "localhost:7233"})
	w := worker.New(c, "example", worker.Options{})
	// All operations will automatically be registered on the service.
	examplev1nexustemporal.RegisterGreetingNexusServiceHandler(w, &greetingHandler{})
	// Workflows need to be registered separately.
	w.RegisterWorkflow(GreetWorkflow)

	_ = w.Run(worker.InterruptCh())
}
```

### Invoke an operation from a workflow

#### Synchronous Call

```go
func CallerWorkflow(ctx workflow.Context) error {
	c := examplev1nexustemporal.NewGreetingNexusClient("example-endpoint")
	output, err := c.Greet(ctx, &example.GreetInput{Name: "World"}, workflow.NexusOperationOptions{})
	if err != nil {
		return err
	}
	workflow.GetLogger(ctx).Info("Got greeting", "greeting", output.Greeting)
	return nil
}
```

#### Asynchronous Call

```go
func CallerWorkflow(ctx workflow.Context) error {
	c := examplev1nexustemporal.NewGreetingNexusClient("example-endpoint")
	fut := c.GreetAsync(ctx, &example.GreetInput{Name: "World"}, workflow.NexusOperationOptions{})
	exec := workflow.NexusOperationExecution{}
	// Wait for operation to be started.
	if err := fut.GetNexusOperationExecution().Get(ctx, &exec); err != nil {
		return err
	}
	output, err := fut.GetTyped(ctx)
	if err != nil {
		return err
	}
	workflow.GetLogger(ctx).Info("Got greeting", "greeting", output.Greeting)
	return nil
}
```

## Options

### Service

#### (nexustemporal.v1.service).disabled

`bool`

Boolean option that can be used to exclude all service methods from generated
artifacts.

**Example:**

```protobuf
syntax = "proto3";

package example.v1;

import "nexustemporal/v1/options.proto";

service ExampleService {
  option (nexustemporal.v1.service).disabled = true;
}
```

#### (nexustemporal.v1.service).name

`string`

Defines the Nexus Service name. Defaults to the proto Service full name.

**Example:**

```protobuf
syntax = "proto3";

package example.v1;

import "nexustemporal/v1/options.proto";

service ExampleService {
  option (nexustemporal.v1.service).name = "example.v1.Example";
}
```

### Method

#### (nexustemporal.v1.operation).disabled

`bool`

Boolean option that can be used to explicitly exclude method from generated
artifacts.

**Example:**

```protobuf
syntax = "proto3";

package example.v1;

import "nexustemporal/v1/options.proto";

service ExampleService {
  rpc Foo(FooInput) returns (FooResponse) {
	option (nexustemporal.v1.operation).disabled = false;
  }
}
```

#### (nexustemporal.v1.operation).enabled

`bool`

Boolean option that can be used to explicitly include method from generated
artifacts.

**Example:**

```protobuf
syntax = "proto3";

package example.v1;

import "nexustemporal/v1/options.proto";

service ExampleService {
  option (nexustemporal.v1.service).disabled = true;

  rpc Foo(FooInput) returns (FooResponse) {
	option (nexustemporal.v1.operation).enabled = false;
  }
}
```

#### (nexustemporal.v1.operation).name

`string`

Defines the Nexus Operation name. Defaults to the proto Method name.

**Example:**

```protobuf
syntax = "proto3";

package example.v1;

import "nexustemporal/v1/options.proto";

service ExampleService {
  rpc Foo(FooInput) returns (FooResponse) {
	option (nexustemporal.v1.operation).name = "foo";
  }
}
```

## Contributing

### Prerequisites

- Go >=1.23
- [Buf](https://buf.build/docs/installation/)

### Build the plugin

```
go build ./cmd/...
```

### Generate code

```
rm -rf ./gen && PATH=${PWD}:${PATH} buf generate
```

### Run sanity tests

```
go test ./...
```

### Lint code

[Install](https://golangci-lint.run/welcome/install/) the latest version of `golangci-lint` and run:

```
golangci-lint run ./...
```
