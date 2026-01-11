package registry_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/registry"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestRegistryModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"2",
				fx.ResultTags(`name:"registry_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("registry-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"registry"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "http")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			resp, err := http.Get(fmt.Sprintf("%s/v2/", endpoint))
			if err != nil {
				t.Errorf("failed to reach registry: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("unexpected status code: %d", resp.StatusCode)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
