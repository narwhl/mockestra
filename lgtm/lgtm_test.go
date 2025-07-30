package lgtm_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/lgtm"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestLGTMModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"lgtm_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("lgtm-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"lgtm"`
		}) {
			t.Log("To be implemented: LGTM module test")
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
