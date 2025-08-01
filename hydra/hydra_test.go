package hydra_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/hydra"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestHydraModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hydra"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Hydra container endpoint: %v", err)
			}
			t.Logf("Hydra container is running at %s", endpoint)
		}),
	)
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
