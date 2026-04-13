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

func allocateRTCProxyPort() (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(mockestra.LoopbackAddress, "0"))
	if err != nil {
		return 0, fmt.Errorf("failed to allocate free port for %s RTC proxy: %w", ContainerPrettyName, err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	slog.Info(fmt.Sprintf("Allocated dynamic RTC TCP proxy port for %s", ContainerPrettyName), "port", port)
	return port, nil
}

type ProxyParams struct {
	fx.In
	LiveKitContainer testcontainers.Container `name:"livekit"`
	RTCProxyPort     int                      `name:"livekit_rtc_proxy_port"`
	Lifecycle        fx.Lifecycle
}

func NewProxy(p ProxyParams) (*proxy.TCPProxy, error) {
	rtcPort := nat.Port(RTCTCPPort)
	livekitEndpoint, err := p.LiveKitContainer.PortEndpoint(context.Background(), rtcPort, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get %s RTC TCP endpoint: %w", ContainerPrettyName, err)
	}
	rtcProxy := proxy.TCPProxy{
		ListenAddress: net.JoinHostPort(mockestra.LoopbackAddress, fmt.Sprintf("%d", p.RTCProxyPort)),
		TargetAddress: livekitEndpoint,
	}
	if err := rtcProxy.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start %s RTC TCP access proxy: %w", ContainerPrettyName, err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			slog.Info(fmt.Sprintf("Forwarding %s RTC TCP traffic via proxy", ContainerPrettyName), "from_addr", rtcProxy.ListenAddress, "to_addr", rtcProxy.TargetAddress)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := rtcProxy.Close(ctx); err != nil {
				return fmt.Errorf("failed to stop %s RTC TCP access proxy: %w", ContainerPrettyName, err)
			}
			return nil
		},
	})
	return &rtcProxy, nil
}
