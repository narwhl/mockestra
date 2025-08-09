package nats_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/nats"
	"github.com/nats-io/nats.go"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestNATSModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"nats_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("nats-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"nats"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get NATS container endpoint: %v", err)
			}
			nc, err := nats.Connect(endpoint)
			if err != nil {
				t.Fatalf("Failed to connect to NATS server: %v", err)
			}
			defer nc.Close()
			err = nc.Publish("foo", []byte("Hello World"))
			if err != nil {
				t.Fatalf("Failed to publish message to NATS server: %v", err)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
