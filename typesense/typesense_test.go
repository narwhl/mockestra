package typesense_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/typesense"
	"github.com/testcontainers/testcontainers-go"
	"github.com/typesense/typesense-go/v3/typesense"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestTypesenseModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"29.0",
				fx.ResultTags(`name:"typesense_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("typesense-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithApiKey("testing"),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"typesense"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client := typesense.NewClient(
				typesense.WithServer(
					fmt.Sprintf("http://%s", endpoint),
				),
				typesense.WithAPIKey("testing"),
			)
			_, err = client.Stats().Retrieve(t.Context())
			if err != nil {
				t.Errorf("failed to retrieve stats: %v", err)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
