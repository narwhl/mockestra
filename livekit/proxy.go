package livekit

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/narwhl/mockestra/proxy"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
)

type ProxyParams struct {
	fx.In
	LiveKitContainer testcontainers.Container `name:"livekit"`
	Lifecycle        fx.Lifecycle
}

// NewProxy creates a TCPProxy that forwards local traffic to the LiveKit
// container's RTC TCP port. By default, the proxy listens on 127.0.0.1:7881,
// matching the rtc.tcp_port and rtc.node_ip advertised in ICE candidates so
// that browsers on the host can establish WebRTC connections.
//
// Use [proxy.WithListenPort] to override the local port when 7881 is
// unavailable (e.g., parallel test runs). If overridden, ensure the
// LIVEKIT_CONFIG rtc.tcp_port is updated to match via [WithListenPort].
func NewProxy(portName string, port nat.Port, opts ...proxy.Option) func(p ProxyParams) (*proxy.TCPProxy, error) {
	return func(p ProxyParams) (*proxy.TCPProxy, error) {
		livekitEndpoint, err := p.LiveKitContainer.PortEndpoint(context.Background(), port, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get %s %s endpoint: %w", ContainerPrettyName, portName, err)
		}
		rtcProxy := proxy.TCPProxy{
			ListenAddress: net.JoinHostPort(mockestra.LoopbackAddress, proxy.ResolveListenPort(port, opts...)),
			TargetAddress: livekitEndpoint,
		}
		if err := rtcProxy.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to start %s %s access proxy: %w", ContainerPrettyName, portName, err)
		}
		p.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				slog.Info(fmt.Sprintf("Forwarding %s %s traffic via proxy", ContainerPrettyName, portName), "from_addr", rtcProxy.ListenAddress, "to_addr", rtcProxy.TargetAddress)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := rtcProxy.Close(ctx); err != nil {
					return fmt.Errorf("failed to stop %s %s access proxy: %w", ContainerPrettyName, portName, err)
				}
				return nil
			},
		})
		return &rtcProxy, nil
	}
}
