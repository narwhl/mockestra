package zitadel_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/zitadel"

	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestZitadelModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"zitadel_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("zitadel-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"zitadel"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Zitadel container endpoint: %v", err)
			}
			t.Logf("Zitadel container is running at %s", endpoint)
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}
