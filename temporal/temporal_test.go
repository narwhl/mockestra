package temporal_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/temporal"
	"github.com/testcontainers/testcontainers-go"
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

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
				t.Errorf("failed to get endpoint: %v", err)
			}
			temporalClient, err := client.Dial(client.Options{
				HostPort: endpoint,
			})
			if err != nil {
				t.Errorf("failed to create temporal client: %v", err)
			}
			defer temporalClient.Close()
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
