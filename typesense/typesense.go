package typesense

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Port = "8108/tcp"

	ContainerPrettyName = "Typesense"
)

func WithApiKey(apiKey string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["TYPESENSE_API_KEY"] = apiKey
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"typesense_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"typesense"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-typesense", p.Prefix),
			Image:        fmt.Sprintf("typesense/typesense:%s", p.Version),
			ExposedPorts: []string{Port},
			Env: map[string]string{
				"TYPESENSE_DATA_DIR": "/data",
			},
			Mounts: testcontainers.ContainerMounts{
				{
					Source: testcontainers.GenericVolumeMountSource{},
					Target: "/data",
				},
			},
			WaitingFor: wait.ForHTTP("/health").WithPort(Port).WithStatusCodeMatcher(func(status int) bool { return status == 200 }).WithStartupTimeout(time.Second * 20),
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
	Request   *testcontainers.GenericContainerRequest `name:"typesense"`
	Logger    *slog.Logger                            `optional:"true"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while instantiating redis container: %w", err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if p.Logger != nil {
				typesenseEndpoint, err := c.Endpoint(ctx, "")
				if err != nil {
					return fmt.Errorf("failed to get %s endpoint: %w", ContainerPrettyName, err)
				}
				p.Logger.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", typesenseEndpoint)
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
	"typesense",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"typesense"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"typesense"`),
		),
	),
)
