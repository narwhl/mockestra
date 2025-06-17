package openfga_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/openfga"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestOpenFGAModule(t *testing.T) {
	app := fxtest.New(
		t,
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
		openfga.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
