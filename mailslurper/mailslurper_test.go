package mailslurper_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/mailslurper"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMailslurperModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("mailslurper-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"mailslurper"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s", endpoint), nil)
			if err != nil {
				t.Errorf("failed to create request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("failed to make request: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status OK, got %s", resp.Status)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
