package hatchet_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/hatchet"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestHatchetModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hatchet_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hatchet-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hatchet"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}

			// The dashboard should be running
			if endpoint == "" {
				t.Fatal("expected non-empty endpoint")
			}
			t.Logf("Hatchet dashboard is running at: http://%s", endpoint)
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestHatchetModuleWithDatabaseURL(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hatchet_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hatchet-db-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithDatabaseURL("postgresql://hatchet:hatchet@localhost:5432/hatchet?sslmode=disable"),
			container.WithServerURL("http://localhost:8080"),
			container.WithGRPCInsecure(),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hatchet"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}

			// Verify the dashboard is accessible
			if endpoint == "" {
				t.Fatal("expected non-empty endpoint")
			}
			t.Logf("Hatchet dashboard with database config is running at: http://%s", endpoint)
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestHatchetModuleWithPostgresConfig(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hatchet_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hatchet-pg-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithPostgresConfig("localhost", "5432", "hatchet", "hatchet", "hatchet"),
			container.WithGRPCInsecure(),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hatchet"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}

			// Verify the dashboard is accessible
			if endpoint == "" {
				t.Fatal("expected non-empty endpoint")
			}
			t.Logf("Hatchet dashboard with postgres config is running at: http://%s", endpoint)
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
