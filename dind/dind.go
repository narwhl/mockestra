package dind

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
	Tag   = "dind"
	Image = "docker"
	Port  = "2375/tcp" // Non-TLS port (TLS uses 2376)

	ContainerPrettyName = "Docker-in-Docker"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"dind_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"dind"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s-dind", Image, p.Version),
			ExposedPorts: []string{Port, "2376/tcp"},
			Env: map[string]string{
				"DOCKER_TLS_CERTDIR": "",
			},
			Privileged: true,
			WaitingFor: wait.ForLog("API listen on"),
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

// WithTLS enables TLS for the Docker daemon
func WithTLS(certDir string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["DOCKER_TLS_CERTDIR"] = certDir
		return nil
	}
}

// WithInsecureRegistries configures the Docker daemon to allow insecure registries
func WithInsecureRegistries(registries ...string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		for i, registry := range registries {
			if i == 0 {
				req.Cmd = append(req.Cmd, "--insecure-registry", registry)
			} else {
				req.Cmd = append(req.Cmd, "--insecure-registry", registry)
			}
		}
		return nil
	}
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"dind"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"dind"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			dindEndpoint, err := c.Endpoint(ctx, "")
			if err != nil {
				return fmt.Errorf("an error occurred while querying %s endpoint: %w", ContainerPrettyName, err)
			}
			slog.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", dindEndpoint)
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

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"dind"`),
		),
		Actualize,
	),
)
