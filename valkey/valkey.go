package valkey

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Port = "6379/tcp"

	ContainerPrettyName = "Valkey"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"valkey_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"valkey"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-valkey", p.Prefix),
			Image:        fmt.Sprintf("valkey/valkey:%s", p.Version),
			ExposedPorts: []string{Port},
			Env:          make(map[string]string),
			WaitingFor:   wait.ForListeningPort(Port),
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&r); err != nil {
			return nil, err
		}
	}

	return &r, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"valkey"`
	Logger    *slog.Logger                            `optional:"true"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if p.Logger != nil {
				valkeyEndpoint, err := c.Endpoint(ctx, "")
				if err != nil {
					return fmt.Errorf("an error occurred while querying %s endpoint: %w", ContainerPrettyName, err)
				}
				p.Logger.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", valkeyEndpoint)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			err := c.Terminate(ctx)
			if p.Logger != nil {
				if err != nil {
					p.Logger.Warn(fmt.Sprintf("an error occurred while terminating %s container", ContainerPrettyName), "error", err)
				} else {
					p.Logger.Info(fmt.Sprintf("%s container is terminated", ContainerPrettyName))
				}
			}
			return err
		},
	})
	return c, nil
}

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	"valkey",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"valkey"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"valkey"`),
		),
	),
)
