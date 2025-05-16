package lgtm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Port     = "3000/tcp"
	HttpPort = "4318/tcp"
	GrpcPort = "4317/tcp"

	ContainerPrettyName = "LGTM"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"lgtm_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"lgtm"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-lgtm", p.Prefix),
			Image: fmt.Sprintf("grafana/otel-lgtm:%s", p.Version),
			ExposedPorts: []string{
				Port,
				HttpPort,
				GrpcPort,
			},
			WaitingFor: wait.ForHTTP("/").WithPort("3000"),
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
	Request   *testcontainers.GenericContainerRequest `name:"lgtm"`
	Logger    *slog.Logger                            `optional:"true"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, err
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if p.Logger != nil {
				portLabels := map[string]string{
					Port:     "dashboard",
					GrpcPort: "otlp (gRPC)",
					HttpPort: "otlp (HTTP)",
				}
				var ports []any
				for port, label := range portLabels {
					p, err := c.MappedPort(ctx, nat.Port(port))
					if err != nil {
						return fmt.Errorf("an error occurred while querying %s container mapped port: %w", ContainerPrettyName, err)
					}
					ports = append(ports, label)
					ports = append(ports, fmt.Sprintf("localhost:%s", p.Port()))
				}
				p.Logger.Info(fmt.Sprintf("%s container is running", ContainerPrettyName), ports...)
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

var Module = mockestra.BuildContainerModule(
	"lgtm",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"lgtm"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"lgtm"`),
		),
	),
)
