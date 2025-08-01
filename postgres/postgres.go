package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/fx"
)

const (
	Tag   = "postgres"
	Image = "postgres"
	Port  = "5432/tcp"

	ContainerPrettyName = "Postgres"
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

func WithExtraDatabase(databaseName, username, password string) testcontainers.CustomizeRequestOption {
	initScript := fmt.Sprintf(`
CREATE USER %[2]s WITH PASSWORD '%[3]s';
CREATE DATABASE %[1]s WITH OWNER %[2]s;
GRANT ALL PRIVILEGES ON DATABASE %[1]s TO %[2]s;
`, databaseName, username, password)
	tempInitFile, err := os.CreateTemp("", fmt.Sprintf("%s-db-init.*.sql", databaseName))
	if err != nil {
		slog.Error("failed to create temp init file", "err", err)
		return nil
	}
	defer tempInitFile.Close()
	if _, err := tempInitFile.Write([]byte(initScript)); err != nil {
		slog.Error("failed to write to temp init file", "err", err)
		return nil
	}
	return postgres.WithInitScripts(tempInitFile.Name())
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"postgres_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"postgres"`
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
	Request   *testcontainers.GenericContainerRequest `name:"postgres"`
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
			fx.ResultTags(`name:"postgres"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"postgres"`),
		),
	),
)
