package concourse

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	"github.com/narwhl/mockestra"
	"github.com/narwhl/mockestra/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag   = "concourse"
	Image = "concourse/concourse"
	Port  = "8080/tcp"

	DatabaseName = "concourse"

	ContainerPrettyName = "Concourse"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"concourse_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"concourse"`
}

func WithPostgres(dsn string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			return err
		}
		req.Env["CONCOURSE_POSTGRES_HOST"] = cfg.Host
		req.Env["CONCOURSE_POSTGRES_PORT"] = fmt.Sprintf("%d", cfg.Port)
		req.Env["CONCOURSE_POSTGRES_USER"] = cfg.User
		req.Env["CONCOURSE_POSTGRES_PASSWORD"] = cfg.Password
		req.Env["CONCOURSE_POSTGRES_DATABASE"] = cfg.Database
		return nil
	}
}

func WithUserAndTeam(user, password, team string) testcontainers.CustomizeRequestOption {
	return testcontainers.WithEnv(map[string]string{
		"CONCOURSE_ADD_LOCAL_USER":       fmt.Sprintf("%s:%s", user, password),
		"CONCOURSE_MAIN_TEAM_LOCAL_USER": team,
	})
}

func WithSecret(secret string) testcontainers.CustomizeRequestOption {
	return testcontainers.WithEnv(map[string]string{
		"CONCOURSE_CLIENT_SECRET":     secret,
		"CONCOURSE_TSA_CLIENT_SECRET": secret,
	})
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			ExposedPorts: []string{Port},
			Env: map[string]string{
				"CONCOURSE_EXTERNAL_URL":                 "http://localhost:8080",
				"CONCOURSE_WORKER_BAGGAGECLAIM_DRIVER":   "overlay",
				"CONCOURSE_X_FRAME_OPTIONS":              "allow",
				"CONCOURSE_CONTENT_SECURITY_POLICY":      "frame-ancestors *;",
				"CONCOURSE_CLUSTER_NAME":                 fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
				"CONCOURSE_WORKER_CONTAINERD_DNS_SERVER": "1.1.1.1",
				"CONCOURSE_WORKER_RUNTIME":               "containerd",
			},
			Privileged: true,
			Cmd:        []string{"quickstart"},
			WaitingFor: wait.ForHTTP("/api/v1/info").WithPort(Port).WithStatusCodeMatcher(func(status int) bool { return status == 200 }).WithStartupTimeout(time.Second * 20),
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&r); err != nil {
			return nil, fmt.Errorf("failed to apply customization to concourse container: %w", err)
		}
	}

	return &r, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle                fx.Lifecycle
	PostgresContainerRequest *testcontainers.GenericContainerRequest `name:"postgres"`
	PostgresContainer        testcontainers.Container                `name:"postgres"`
	Request                  *testcontainers.GenericContainerRequest `name:"concourse"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"concourse"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	postgresIP, err := p.PostgresContainer.ContainerIP(context.Background())
	if err != nil {
		return Result{}, fmt.Errorf("failed to get %s container IP: %w", postgres.ContainerPrettyName, err)
	}
	_, postgresPort := nat.SplitProtoPort(postgres.Port)

	if err := WithPostgres(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		p.PostgresContainerRequest.Env["POSTGRES_USER"], // TODO: use database specific user instead of admin user
		p.PostgresContainerRequest.Env["POSTGRES_PASSWORD"],
		postgresIP,
		postgresPort,
		DatabaseName,
	)).Customize(p.Request); err != nil {
		return Result{}, fmt.Errorf("failed to set postgres url: %w", err)
	}

	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, err
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := c.Start(ctx); err != nil {
				return fmt.Errorf("failed to start %s container: %w", ContainerPrettyName, err)
			}
			portLabels := map[string]string{
				Port: "http",
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

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"concourse"`),
		),
		Actualize,
	),
)
