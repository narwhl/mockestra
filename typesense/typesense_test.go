package typesense_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/typesense"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestTypesenseModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"typesense_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("typesense-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		typesense.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
