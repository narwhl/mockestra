package openfga

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
)

const (
	Image               = "openfga/openfga"
	PlaygroundPort      = "3000/tcp"
	HttpPort            = "8080/tcp"
	GrpcPort            = "8081/tcp"
	ContainerPrettyName = "OpenFGA"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"openfga_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"openfga"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-openfga", p.Prefix),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{
				PlaygroundPort,
				HttpPort,
				GrpcPort,
			},
			Cmd: []string{"run"},
			Env: map[string]string{},
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&req); err != nil {
			return nil, fmt.Errorf("failed to customize OpenFGA container: %w", err)
		}
	}
	return &req, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"openfga"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(
		context.Background(),
		*p.Request,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenFGA container: %w", err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				PlaygroundPort: "playground",
				GrpcPort:       "gRPC",
				HttpPort:       "HTTP",
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
			slog.Info(fmt.Sprintf("%s container is running", ContainerPrettyName), ports...)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			err := c.Terminate(ctx)
			if err != nil {
				slog.Warn(fmt.Sprintf("an error occurred while terminating %s container", ContainerPrettyName), "error", err)
			} else {
				slog.Info(fmt.Sprintf("%s container is terminated", ContainerPrettyName))
			}
			return err
		},
	})
	return c, nil
}

var Module = mockestra.BuildContainerModule(
	"openfga",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"openfga"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"openfga"`),
		),
	),
)

