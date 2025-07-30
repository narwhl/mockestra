package mailslurper

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
	Tag      = "mailslurper"
	Image    = "oryd/mailslurper"
	Port     = "4436/tcp"
	APIPort  = "4437/tcp"
	SMTPPort = "1025/tcp"

	ContainerPrettyName = "Mailslurper"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"mailslurper_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"mailslurper"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, APIPort, SMTPPort},
			WaitingFor:   wait.ForHTTP("/").WithPort(Port),
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
	Request   *testcontainers.GenericContainerRequest `name:"mailslurper"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while instantiating mailslurper container: %w", err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port:     "dashboard",
				APIPort:  "api",
				SMTPPort: "SMTP",
			}
			var endpoints []any
			for port, label := range portLabels {
				endpoint, err := c.PortEndpoint(context.Background(), nat.Port(port), "")
				if err != nil {
					return fmt.Errorf("an error occurred while querying %s container mapped port: %w", ContainerPrettyName, err)
				}
				endpoints = append(endpoints, label)
				endpoints = append(endpoints, endpoint)
			}
			slog.Info(fmt.Sprintf("%s container is running", ContainerPrettyName), endpoints...)
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

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"mailslurper"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"mailslurper"`),
		),
		fx.Annotate(
			NewProxy,
			fx.ResultTags(`name:"mailslurper"`),
		),
	),
)
