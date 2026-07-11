package livekit_test

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/livekit"
	"github.com/narwhl/mockestra/proxy"
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

// TestLiveKitRTCProxy verifies the RTC TCP access proxy is wired to the port
// LiveKit actually listens on. LiveKit's rtc.tcp_port is set to the
// dynamically-allocated RTCProxyPort, so that port (not the static 7881) must
// be the one the container exposes and the proxy targets. Before the fix the
// container exposed 7881 while LiveKit listened on RTCProxyPort, so the proxy
// forwarded into a dead port.
func TestLiveKitRTCProxy(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(fx.Annotate("v1.10.1", fx.ResultTags(`name:"livekit_version"`))),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("livekit-proxy-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		// Consume the proxy so fx instantiates it (NewProxy starts the listener
		// at construction time); RTCProxyPort lets us assert the exposed port.
		fx.Invoke(func(params struct {
			fx.In
			Container    testcontainers.Container `name:"livekit"`
			Proxy        *proxy.TCPProxy          `name:"livekit"`
			RTCProxyPort int                      `name:"livekit_rtc_proxy_port"`
		}) {
			if params.Proxy == nil {
				t.Fatal("expected RTC proxy to be instantiated, got nil")
			}

			// The container must expose the dynamic RTC port — proof that the
			// exposed port matches LiveKit's configured rtc.tcp_port.
			rtcPort := fmt.Sprintf("%d/tcp", params.RTCProxyPort)
			rtcEndpoint, err := params.Container.PortEndpoint(t.Context(), rtcPort, "")
			if err != nil {
				t.Fatalf("RTC port %s not exposed by container (proxy would forward into a void): %v", rtcPort, err)
			}

			// LiveKit's RTC TCP listener must be live on the mapped host port.
			conn, err := net.DialTimeout("tcp", rtcEndpoint, 5*time.Second)
			if err != nil {
				t.Fatalf("failed to reach LiveKit RTC TCP listener at %s: %v", rtcEndpoint, err)
			}
			_ = conn.Close()

			// And the proxy must be accepting on the advertised loopback port
			// (the address LiveKit hands clients in ICE candidates).
			proxyAddr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", params.RTCProxyPort))
			pconn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
			if err != nil {
				t.Fatalf("RTC proxy not accepting on advertised address %s: %v", proxyAddr, err)
			}
			_ = pconn.Close()
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
