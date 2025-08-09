package valkey_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/valkey"
	"github.com/testcontainers/testcontainers-go"
	"github.com/valkey-io/valkey-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestValKeyModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"valkey_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("valkey-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"valkey"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{endpoint}})
			if err != nil {
				t.Errorf("failed to create valkey client: %v", err)
			}
			defer client.Close()
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
