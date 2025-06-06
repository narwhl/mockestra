package kratos

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
	KratosContainer testcontainers.Container `name:"kratos"`
	Lifecycle       fx.Lifecycle
}

func NewProxy(portName string, port nat.Port) func(p ProxyParams) (*proxy.TCPProxy, error) {
	return func(p ProxyParams) (*proxy.TCPProxy, error) {
		kratosAPIEndpoint, err := p.KratosContainer.PortEndpoint(context.Background(), port, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get %s %s endpoint: %w", ContainerPrettyName, portName, err)
		}
		apiAccessProxy := proxy.TCPProxy{
			ListenAddress: net.JoinHostPort(mockestra.LoopbackAddress, port.Port()),
			TargetAddress: kratosAPIEndpoint,
		}
		if err := apiAccessProxy.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to start %s %s access proxy: %w", ContainerPrettyName, portName, err)
		}
		p.Lifecycle.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				slog.Info(fmt.Sprintf("Forwarding %s %s traffic via proxy", ContainerPrettyName, portName), "from_addr", apiAccessProxy.ListenAddress, "to_addr", apiAccessProxy.TargetAddress)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := apiAccessProxy.Close(ctx); err != nil {
					return fmt.Errorf("failed to stop %s %s access proxy: %w", ContainerPrettyName, portName, err)
				}
				return nil
			},
		})
		return &apiAccessProxy, nil
	}
}
