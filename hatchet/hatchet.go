package hatchet

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag           = "hatchet"
	Image         = "ghcr.io/hatchet-dev/hatchet/hatchet-dashboard"
	Port          = "80/tcp"
	GRPCPort      = "7070/tcp"
	HealthPort    = "8733/tcp"

	ContainerPrettyName = "Hatchet"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix" optional:"true"`
	Version string                               `name:"hatchet_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"hatchet"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	if p.Prefix == "" {
		p.Prefix = mockestra.GeneratePrefix()
	}
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, GRPCPort, HealthPort},
			Env:          make(map[string]string),
			Cmd: []string{
				"sh",
				"./entrypoint.sh",
				"--config",
				"/hatchet/config",
			},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort(Port),
				wait.ForHTTP("/").WithPort(Port).WithStatusCodeMatcher(func(status int) bool {
					return status == http.StatusOK || status == http.StatusNotFound
				}),
			).WithDeadline(5 * time.Minute),
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
	Request   *testcontainers.GenericContainerRequest `name:"hatchet"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"hatchet"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			dashboardPort, err := c.MappedPort(ctx, Port)
			if err != nil {
				return fmt.Errorf("unable to get %s dashboard port: %w", ContainerPrettyName, err)
			}

			grpcPort, err := c.MappedPort(ctx, GRPCPort)
			if err != nil {
				return fmt.Errorf("unable to get %s gRPC port: %w", ContainerPrettyName, err)
			}

			healthPort, err := c.MappedPort(ctx, HealthPort)
			if err != nil {
				return fmt.Errorf("unable to get %s healthcheck port: %w", ContainerPrettyName, err)
			}

			slog.Info(
				fmt.Sprintf("%s container is running", ContainerPrettyName),
				"dashboard", fmt.Sprintf("http://localhost:%s", dashboardPort.Port()),
				"grpc", fmt.Sprintf("localhost:%s", grpcPort.Port()),
				"health", fmt.Sprintf("localhost:%s", healthPort.Port()),
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

// WithDatabaseURL sets the DATABASE_URL environment variable for Hatchet
func WithDatabaseURL(databaseURL string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env["DATABASE_URL"] = databaseURL
		return nil
	}
}

// WithPostgresConfig configures Hatchet to connect to a PostgreSQL container
func WithPostgresConfig(host, port, user, password, dbname string) testcontainers.CustomizeRequestOption {
	return WithDatabaseURL(
		fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
			user, password, host, port, dbname),
	)
}

// WithServerURL sets the SERVER_URL environment variable
func WithServerURL(serverURL string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env["SERVER_URL"] = serverURL
		return nil
	}
}

// WithGRPCInsecure enables insecure gRPC connections
func WithGRPCInsecure() testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env["SERVER_GRPC_INSECURE"] = "true"
		return nil
	}
}

// WithEnv sets an arbitrary environment variable for Hatchet
func WithEnv(key, value string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env[key] = value
		return nil
	}
}

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	"hatchet",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"hatchet"`),
		),
		Actualize,
	),
)
