package nats

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag       = "nats"
	Image     = "nats"
	Port      = "4222/tcp"
	HttpPort  = "8222/tcp"
	RoutePort = "6222/tcp"

	ContainerPrettyName = "NATS Server"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"nats_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"nats"`
}

var WithUsername = nats.WithUsername
var WithPassword = nats.WithPassword
var WithArgument = nats.WithArgument
var WithConfigFile = nats.WithConfigFile

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, HttpPort, RoutePort},
			Env:          map[string]string{},
			Cmd:          []string{"-DV", "-js"},
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
	Request   *testcontainers.GenericContainerRequest `name:"nats"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"nats"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := c.Start(ctx); err != nil {
				return fmt.Errorf("failed to start %s container: %w", ContainerPrettyName, err)
			}
			portLabels := map[string]string{
				Port:      "client",
				HttpPort:  "http",
				RoutePort: "route",
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
				slog.Warn("failed to terminate NATS container", "error", err)
			} else {
				slog.Info("NATS container terminated successfully")
			}
			return err
		},
	})

	return Result{
		Container:      c,
		ContainerGroup: c,
	}, nil
}

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"nats"`),
		),
		Actualize,
	),
)
