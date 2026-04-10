package livekit_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/livekit"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestLiveKitModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(fx.Annotate("v1.10.1", fx.ResultTags(`name:"livekit_version"`))),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("livekit-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"livekit"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.SignalPort, "http")
			if err != nil {
				t.Fatalf("failed to get endpoint: %v", err)
			}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint+"/", nil)
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to GET livekit root: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 from livekit health endpoint, got %d", resp.StatusCode)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
