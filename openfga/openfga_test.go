package openfga_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/openfga"
	. "github.com/openfga/go-sdk/client"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestOpenFGAModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"openfga_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("openfga-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"openfga"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.HttpPort, "")
			if err != nil {
				t.Fatalf("Failed to get OpenFGA container endpoint: %v", err)
			}
			fgaClient, err := NewSdkClient(&ClientConfiguration{
				ApiUrl: fmt.Sprintf("http://%s", endpoint),
			})
			if err != nil {
				t.Fatalf("Failed to create OpenFGA client: %v", err)
			}
			_, err = fgaClient.ListStores(t.Context()).Options(ClientListStoresOptions{}).Execute()
			if err != nil {
				t.Fatalf("Failed to list OpenFGA stores: %v", err)
			}
		}),
	)

	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}
