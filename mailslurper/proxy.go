package mailslurper

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
	MailslurperContainer testcontainers.Container `name:"mailslurper"`
	Logger               *slog.Logger             `optional:"true"`
	Lifecycle            fx.Lifecycle
}

func NewProxy(p ProxyParams) (*proxy.TCPProxy, error) {
	mailslurperAPIEndpoint, err := p.MailslurperContainer.PortEndpoint(context.Background(), nat.Port(APIPort), "")
	if err != nil {
		return nil, fmt.Errorf("failed to get mailslurper API endpoint: %w", err)
	}
	apiAccessProxy := proxy.TCPProxy{
		ListenAddress: net.JoinHostPort(mockestra.LoopbackAddress, nat.Port(APIPort).Port()),
		TargetAddress: mailslurperAPIEndpoint,
	}
	if err := apiAccessProxy.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start access proxy: %w", err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if p.Logger != nil {
				p.Logger.Info("Forwarding mailslurper API traffic via proxy", "from_addr", apiAccessProxy.ListenAddress, "to_addr", apiAccessProxy.TargetAddress)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := apiAccessProxy.Close(ctx); err != nil {
				return fmt.Errorf("failed to stop access proxy: %w", err)
			}
			return nil
		},
	})
	return &apiAccessProxy, nil
}
