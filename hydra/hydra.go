package hydra

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/narwhl/mockestra/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Port      = "4444/tcp"
	AdminPort = "4445/tcp"

	DatabaseName = "hydra"

	ContainerPrettyName = "Ory Hydra"
)

func WithPostgres(dsn string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["DSN"] = dsn
		return nil
	}
}

func WithURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["URLS_SELF_ISSUER"] = url
		return nil
	}
}

func WithSelfServiceUIURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["URLS_CONSENT"] = fmt.Sprintf("%s/consent", url)
		req.Env["URLS_LOGIN"] = fmt.Sprintf("%s/login", url)
		req.Env["URLS_LOGOUT"] = fmt.Sprintf("%s/logout", url)
		return nil
	}
}

func WithKratosPublicURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["URLS_IDENTITY_PROVIDER_PUBLICURL"] = url
		return nil
	}
}

func WithKratosURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["URLS_IDENTITY_PROVIDER_URL"] = url
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"hydra_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"hydra"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	hydraCookieSecret, err := mockestra.RandomPassword(32)
	if err != nil {
		return nil, err
	}
	hydraSystemSecret, err := mockestra.RandomPassword(32)
	if err != nil {
		return nil, err
	}
	oidcPairwiseSalt, err := mockestra.RandomPassword(32)
	if err != nil {
		return nil, err
	}
	req := testcontainers.ContainerRequest{
		Name:         fmt.Sprintf("mock-%s-hydra", p.Prefix),
		Image:        fmt.Sprintf("oryd/hydra:%s", p.Version),
		ExposedPorts: []string{Port, AdminPort},
		Env: map[string]string{
			"SERVE_COOKIES_SAME_SITE_MODE":               "Lax",
			"SECRETS_COOKIE_0":                           hydraCookieSecret,
			"SECRETS_SYSTEM_0":                           hydraSystemSecret,
			"OIDC_SUBJECT_IDENTIFIERS_SUPPORTED_TYPES_0": "pairwise",
			"OIDC_SUBJECT_IDENTIFIERS_SUPPORTED_TYPES_1": "public",
			"OIDC_SUBJECT_IDENTIFIERS_PAIRWISE_SALT":     oidcPairwiseSalt,
		},
		WaitingFor: wait.ForHTTP("/health/ready").WithPort(Port),
		Cmd:        []string{"serve", "all", "--dev"},
	}
	genericContainerReq := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&genericContainerReq); err != nil {
			return nil, err
		}
	}

	return &genericContainerReq, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle                fx.Lifecycle
	Prefix                   string                                  `name:"prefix"`
	PostgresContainerRequest *testcontainers.GenericContainerRequest `name:"postgres"`
	PostgresContainer        testcontainers.Container                `name:"postgres"`
	Request                  *testcontainers.GenericContainerRequest `name:"hydra"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	postgresIP, err := p.PostgresContainer.ContainerIP(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s container IP: %w", postgres.ContainerPrettyName, err)
	}
	_, postgresPort := nat.SplitProtoPort(postgres.Port)

	if err := WithPostgres(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		p.PostgresContainerRequest.Env["POSTGRES_USER"], // TODO: use database specific user instead of admin user
		p.PostgresContainerRequest.Env["POSTGRES_PASSWORD"],
		postgresIP,
		postgresPort,
		DatabaseName,
	)).Customize(p.Request); err != nil {
		return nil, err
	}

	migrateGenericContainerReq := *p.Request
	migrateGenericContainerReq.ContainerRequest.Name = fmt.Sprintf("mock-%s-hydra-migrate", p.Prefix)
	migrateGenericContainerReq.ContainerRequest.Cmd = []string{"migrate", "sql", "-e", "--yes"}
	migrateGenericContainerReq.ContainerRequest.WaitingFor = wait.ForExit()

	migrateContainer, err := testcontainers.GenericContainer(context.Background(), migrateGenericContainerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to run %s migration: %w", ContainerPrettyName, err)
	}
	if err := migrateContainer.Terminate(context.Background()); err != nil {
		slog.Warn(fmt.Sprintf("an error occurred while terminating %s migration container", ContainerPrettyName), "error", err)
	}

	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port:      "API",
				AdminPort: "Admin API",
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
	"hydra",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"hydra"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"hydra"`),
		),
		fx.Annotate(
			NewProxy("Public API", nat.Port(Port)),
			fx.ResultTags(`name:"hydra"`),
		),
		fx.Annotate(
			NewProxy("Admin API", nat.Port(AdminPort)),
			fx.ResultTags(`name:"hydraadmin"`),
		),
	),
)
