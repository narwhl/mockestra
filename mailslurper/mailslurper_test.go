package mailslurper_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/lgtm"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMailslurperModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("mailslurper-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		lgtm.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
