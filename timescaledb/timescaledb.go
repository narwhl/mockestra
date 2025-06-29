package timescaledb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/fx"
)

const (
	Tag   = "timescaledb"
	Image = "timescale/timescaledb"
	Port  = "5432/tcp"

	ContainerPrettyName = "TimescaleDB"
)

var (
	WithUsername = postgres.WithUsername
	WithPassword = postgres.WithPassword
	WithDatabase = postgres.WithDatabase
)

type migration func(string) error

func WithMigration(fn migration) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					addr, err := container.Endpoint(ctx, "")
					if err != nil {
						return fmt.Errorf("encounter error getting addr while running migration: %w", err)
					}
					return fn(
						fmt.Sprintf(
							"postgres://%s:%s@%s/%s?sslmode=disable",
							req.Env["POSTGRES_USER"],
							req.Env["POSTGRES_PASSWORD"],
							addr,
							req.Env["POSTGRES_DB"],
						),
					)
				},
			},
		})
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"timescaledb_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"timescaledb"`
}

// New is a constructor that returns a testcontainers.GenericContainerRequest
// and takes its group tagged testcontainers.ContainerCustomizer as options.
// it is part of tri-phase process with Actualize and Run to create
// a testcontainers.Container.
func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port},
			Env:          make(map[string]string),
		},
		Started: true,
	}

	for _, opt := range append(p.Opts, postgres.BasicWaitStrategies()) {
		if err := opt.Customize(&r); err != nil {
			return nil, err
		}
	}

	return &r, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"timescaledb"`
}

// Actualize is a constructor that returns a testcontainers.Container
// it consumes previously instantiated testcontainers.GenericContainerRequest
// as part of its inputs, alongside with other tag specified testcontainers.GenericContainerRequest
// in order to reconcile its lifecycle dependencies before creating a testcontainers.Container.
func Actualize(p ContainerParams) (testcontainers.Container, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			postgresPort, err := c.MappedPort(ctx, Port)
			if err != nil {
				return fmt.Errorf("an error occurred while querying %s container mapped port: %w", ContainerPrettyName, err)
			}
			slog.Info(fmt.Sprintf("%s container is running", ContainerPrettyName), "addr", fmt.Sprintf("localhost:%s", postgresPort.Port()))
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
			fx.ResultTags(`name:"timescaledb"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"timescaledb"`),
		),
	),
)
