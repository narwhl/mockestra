package valkey_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/valkey"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestValKeyModule(t *testing.T) {
	app := fxtest.New(
		t,
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
		valkey.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
