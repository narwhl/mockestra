package dind_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/dind"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestDindModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"27",
				fx.ResultTags(`name:"dind_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("dind-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"dind"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			if endpoint == "" {
				t.Errorf("endpoint is empty")
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
