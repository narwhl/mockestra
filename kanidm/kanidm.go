package kanidm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag                 = "kanidm"
	Image               = "kanidm/server"
	Port                = "8443/tcp"
	LDAPPort            = "3636/tcp"
	ContainerPrettyName = "kanidm"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"kanidm_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"kanidm"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{
				Port,
			},
			WaitingFor: wait.ForLog(".*[info]: ready to rock!.*\\s").AsRegexp().WithOccurrence(1),
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
	Request   *testcontainers.GenericContainerRequest `name:"kanidm"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"kanidm"`
	ContainerGroup testcontainers.Container `name:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, err
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port: "https",
			}
			var ports []any
			for port, label := range portLabels {
				p, err := c.MappedPort(ctx, nat.Port(port))
				if err != nil {
					return fmt.Errorf("failed to get mapped port for %s: %w", port, err)
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

	return Result{
		Container:      c,
		ContainerGroup: c,
	}, nil
}
