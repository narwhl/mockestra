package lgtm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/lgtm"
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
		lgtm.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
