package temporal_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/temporal"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestTemporalModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"temporal_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("temporal-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		temporal.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
