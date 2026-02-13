package temporal_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/temporal"
	"github.com/testcontainers/testcontainers-go"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestWithNamespace(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"temporal_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("temporal-ns-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithNamespace("test-namespace"),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"temporal"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}
			temporalClient, err := client.Dial(client.Options{
				HostPort:  endpoint,
				Namespace: "test-namespace",
			})
			if err != nil {
				t.Fatalf("failed to create temporal client: %v", err)
			}
			defer temporalClient.Close()
			resp, err := temporalClient.WorkflowService().DescribeNamespace(t.Context(), &workflowservice.DescribeNamespaceRequest{
				Namespace: "test-namespace",
			})
			if err != nil {
				t.Fatalf("failed to describe namespace: %v", err)
			}
			if resp.NamespaceInfo.GetName() != "test-namespace" {
				t.Errorf("expected namespace 'test-namespace', got '%s'", resp.NamespaceInfo.GetName())
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestTemporalModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"temporal_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("temporal-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"temporal"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}
			temporalClient, err := client.Dial(client.Options{
				HostPort: endpoint,
			})
			if err != nil {
				t.Fatalf("failed to create temporal client: %v", err)
			}
			defer temporalClient.Close()
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
