package minio

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
)

const (
	Port        = "9000/tcp"
	ConsolePort = "9001/tcp"

	ContainerPrettyName = "Minio"
)

type MinioCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

func WithObjectStorageCredentials(credentials MinioCredentials) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["MINIO_ROOT_USER"] = credentials.AccessKeyID
		req.Env["MINIO_ROOT_PASSWORD"] = credentials.SecretAccessKey
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"minio_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"minio"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-minio", p.Prefix),
			Image: fmt.Sprintf("minio/minio:%s", p.Version),
			Cmd:   []string{"server", "/data"},
			ExposedPorts: []string{
				Port,
				ConsolePort,
			},
			Env: make(map[string]string),
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
	Request   *testcontainers.GenericContainerRequest `name:"minio"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, err
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			minioEndpoint, err := c.Endpoint(ctx, "")
			if err != nil {
				return fmt.Errorf("failed to get %s endpoint: %w", ContainerPrettyName, err)
			}
			slog.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", minioEndpoint)
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
	"minio",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"minio"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"minio"`),
		),
	),
)
