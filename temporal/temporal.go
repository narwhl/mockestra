package temporal

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag    = "temporal"
	Image  = "temporalio/auto-setup"
	Port   = "7233/tcp"
	UIPort = "8233/tcp"

	ContainerPrettyName = "Temporal"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"temporal_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"temporal"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {

	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, UIPort},
			WaitingFor: wait.ForHTTP("/").WithPort(UIPort).WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusOK
			}),
			Entrypoint: []string{"/usr/local/bin/temporal"},
			Cmd:        []string{"server", "start-dev", "--ip", "0.0.0.0"},
			Env:        make(map[string]string),
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
	Request   *testcontainers.GenericContainerRequest `name:"temporal"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			temporalPort, err := c.MappedPort(ctx, Port)
			if err != nil {
				return fmt.Errorf("unable to get %s port: %w", ContainerPrettyName, err)
			}

			temporalUiPort, err := c.MappedPort(ctx, UIPort)
			if err != nil {
				return fmt.Errorf("unable to get %s ui port: %w", ContainerPrettyName, err)
			}

			slog.Info(
				fmt.Sprintf("%s container is running", ContainerPrettyName),
				"addr", fmt.Sprintf("localhost:%s", temporalPort.Port()),
				"ui", fmt.Sprintf("localhost:%s", temporalUiPort.Port()),
			)
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
	"temporal",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"temporal"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"temporal"`),
		),
	),
)
