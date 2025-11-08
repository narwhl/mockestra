package versitygw

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
	Tag   = "versitygw"
	Image = "versity/versitygw"
	Port  = "7070/tcp"

	ContainerPrettyName = "VersityGW S3 Gateway"
)

// WithAccessKey sets the ROOT_ACCESS_KEY environment variable for S3 API authentication
func WithAccessKey(key string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ROOT_ACCESS_KEY"] = key
		return nil
	}
}

// WithSecretKey sets the ROOT_SECRET_KEY environment variable for S3 API authentication
func WithSecretKey(secret string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ROOT_SECRET_KEY"] = secret
		return nil
	}
}

// WithBackend configures the storage backend and its argument
// Supported backends: posix, scoutfs, azure, s3
func WithBackend(backend, arg string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if arg != "" {
			req.Cmd = []string{backend, arg}
		} else {
			req.Cmd = []string{backend}
		}
		return nil
	}
}

// WithPOSIXBackend is a convenience function to configure a POSIX filesystem backend
func WithPOSIXBackend(path string) testcontainers.CustomizeRequestOption {
	return WithBackend("posix", path)
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"versitygw_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"versitygw"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	// Generate random credentials for S3 API access
	accessKey, err := mockestra.RandomPassword(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access key: %w", err)
	}
	secretKey, err := mockestra.RandomPassword(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret key: %w", err)
	}

	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port},
			Env: map[string]string{
				"ROOT_ACCESS_KEY": accessKey,
				"ROOT_SECRET_KEY": secretKey,
			},
			Cmd: []string{"posix", "--nometa", "/data"},
			Tmpfs: map[string]string{
				"/data": "rw",
			},
			WaitingFor: wait.ForListeningPort(Port).WithStartupTimeout(time.Second * 10),
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
	Request   *testcontainers.GenericContainerRequest `name:"versitygw"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"versitygw"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			endpoint, err := c.Endpoint(ctx, "")
			if err != nil {
				return fmt.Errorf("an error occurred while querying %s endpoint: %w", ContainerPrettyName, err)
			}
			slog.Info(
				fmt.Sprintf("%s container is running", ContainerPrettyName),
				"endpoint", endpoint,
				"access_key", p.Request.Env["ROOT_ACCESS_KEY"],
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
			fx.ResultTags(`name:"versitygw"`),
		),
		Actualize,
	),
)
