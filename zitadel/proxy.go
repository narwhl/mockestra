package zitadel

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/narwhl/mockestra"
	"github.com/narwhl/mockestra/proxy"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
)

const (
	ProxyPort = "8081"
)

type ProxyParams struct {
	fx.In
	ZitadelContainer testcontainers.Container `name:"zitadel"`
	Lifecycle        fx.Lifecycle
}

func NewProxy(p ProxyParams) (*proxy.TCPProxy, error) {
	zitadelEndpoint, err := p.ZitadelContainer.Endpoint(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("failed to get zitadel endpoint: %w", err)
	}
	accessProxy := proxy.TCPProxy{
		ListenAddress: net.JoinHostPort(mockestra.LoopbackAddress, ProxyPort),
		TargetAddress: zitadelEndpoint,
	}
	if err := accessProxy.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start access proxy: %w", err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			slog.Info("Forwarding Zitadel traffic via proxy", "from_addr", accessProxy.ListenAddress, "to_addr", accessProxy.TargetAddress)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := accessProxy.Close(ctx); err != nil {
				return fmt.Errorf("failed to stop access proxy: %w", err)
			}
			return nil
		},
	})
	return &accessProxy, nil
}
