package minio_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/minio"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMinioModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"minio_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("minio-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		minio.Module(),
	)

	app.RequireStart()
	defer app.RequireStop()
}
