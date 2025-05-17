package zitadel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/narwhl/mockestra/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Port         = "8080/tcp"
	DatabaseName = "zitadel"
)

var WithPostReadyHook = mockestra.WithPostReadyHook

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"zitadel_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"zitadel"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        fmt.Sprintf("ghcr.io/zitadel/zitadel:%s", p.Version),
			Name:         fmt.Sprintf("mock-%s-zitadel", p.Prefix),
			ExposedPorts: []string{Port},
			Env: map[string]string{
				"ZITADEL_EXTERNALDOMAIN": mockestra.LoopbackAddress,
				"ZITADEL_EXTERNALPORT":   ProxyPort,
				"ZITADEL_EXTERNALSECURE": "false",
			},
			Cmd:        []string{"start-from-init", "--masterkeyFromEnv", "--tlsMode", "disabled"},
			WaitingFor: wait.ForHTTP("/debug/healthz").WithPort(Port).WithStatusCodeMatcher(func(status int) bool { return status == 200 }).WithStartupTimeout(time.Second * 20),
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&r); err != nil {
			return nil, fmt.Errorf("failed to apply customization to zitadel container: %w", err)
		}
	}

	return &r, nil
}

func WithPostgresConnection(host, port, database, username, password, sslmode string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ZITADEL_DATABASE_POSTGRES_HOST"] = host
		req.Env["ZITADEL_DATABASE_POSTGRES_PORT"] = port
		req.Env["ZITADEL_DATABASE_POSTGRES_DATABASE"] = database
		req.Env["ZITADEL_DATABASE_POSTGRES_USER_USERNAME"] = username
		req.Env["ZITADEL_DATABASE_POSTGRES_USER_PASSWORD"] = password
		req.Env["ZITADEL_DATABASE_POSTGRES_USER_SSL_MODE"] = sslmode
		return nil
	}
}

func WithPostgresAdminConnection(username, password, sslmode string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ZITADEL_DATABASE_POSTGRES_ADMIN_USERNAME"] = username
		req.Env["ZITADEL_DATABASE_POSTGRES_ADMIN_PASSWORD"] = password
		req.Env["ZITADEL_DATABASE_POSTGRES_ADMIN_SSL_MODE"] = sslmode
		return nil
	}
}

func WithMasterkey(key string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ZITADEL_MASTERKEY"] = key
		return nil
	}
}

func WithOrganizationName(name string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ZITADEL_FIRSTINSTANCE_ORG_NAME"] = name
		return nil
	}
}

func WithAdminUser(username, password string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_USERNAME"] = username
		req.Env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD"] = password
		return nil
	}
}

func WithServiceUser(username string, machineKeyPath string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Files = append(req.Files, testcontainers.ContainerFile{
			ContainerFilePath: fmt.Sprintf("/%s.json", username),
			Reader:            bytes.NewBufferString(""),
			FileMode:          0o666,
		})
		req.Env["ZITADEL_FIRSTINSTANCE_MACHINEKEYPATH"] = fmt.Sprintf("/%s.json", username)
		req.Env["ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_USERNAME"] = username
		req.Env["ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_NAME"] = username
		req.Env["ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINEKEY_TYPE"] = "1"
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					saContainerFile, err := container.CopyFileFromContainer(ctx, req.Env["ZITADEL_FIRSTINSTANCE_MACHINEKEYPATH"])
					if err != nil {
						return fmt.Errorf("failed to copy file from container: %w", err)
					}
					defer saContainerFile.Close()
					saFile, err := os.OpenFile(machineKeyPath, os.O_WRONLY, os.ModePerm)
					if err != nil {
						return fmt.Errorf("failed to open host service account file: %w", err)
					}
					defer saFile.Close()
					_, err = io.Copy(saFile, saContainerFile)
					if err != nil {
						return fmt.Errorf("failed to copy file: %w", err)
					}
					return nil
				},
			},
		})
		return nil
	}
}

type ContainerParams struct {
	fx.In
	Lifecycle                fx.Lifecycle
	PostgresContainerRequest *testcontainers.GenericContainerRequest `name:"postgres"`
	PostgresContainer        testcontainers.Container                `name:"postgres"`
	Request                  *testcontainers.GenericContainerRequest `name:"zitadel"`
}

func Actualize(p ContainerParams) (testcontainers.Container, error) {
	postgresIP, err := p.PostgresContainer.ContainerIP(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s container IP: %w", postgres.ContainerPrettyName, err)
	}
	_, postgresPort := nat.SplitProtoPort(postgres.Port)
	if err := WithPostgresConnection(
		postgresIP,
		postgresPort,
		DatabaseName,
		p.PostgresContainerRequest.Env["POSTGRES_USER"], // TODO: use database specific user instead of admin user
		p.PostgresContainerRequest.Env["POSTGRES_PASSWORD"],
		"disable",
	).Customize(p.Request); err != nil {
		return nil, fmt.Errorf("failed to apply zitadel postgres connection: %w", err)
	}
	if err := WithPostgresAdminConnection(
		p.PostgresContainerRequest.Env["POSTGRES_USER"],
		p.PostgresContainerRequest.Env["POSTGRES_PASSWORD"],
		"disable",
	).Customize(p.Request); err != nil {
		return nil, fmt.Errorf("failed to apply zitadel postgres admin connection: %w", err)
	}

	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return nil, fmt.Errorf("failed to create zitadel container: %w", err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			zitadelEndpoint, err := c.Endpoint(context.Background(), "")
			if err != nil {
				return fmt.Errorf("failed to get zitadel endpoint: %w", err)
			}
			slog.Info("Zitadel container is running at", "addr", zitadelEndpoint)
			slog.Info("Zitadel is accessible via admin credentials",
				"username", fmt.Sprintf("%s@%s.%s", p.Request.Env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_USERNAME"], strings.ToLower(p.Request.Env["ZITADEL_FIRSTINSTANCE_ORG_NAME"]), mockestra.LoopbackAddress),
				"password", p.Request.Env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD"],
			)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if err := c.Terminate(ctx); err != nil {
				return fmt.Errorf("failed to terminate zitadel container: %w", err)
			}
			return nil
		},
	})

	return c, nil
}

var Module = mockestra.BuildContainerModule(
	"zitadel",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"zitadel"`),
		),
		fx.Annotate(
			Actualize,
			fx.ResultTags(`name:"zitadel"`),
		),
		fx.Annotate(
			NewProxy,
			fx.ResultTags(`name:"zitadel"`),
		),
	),
)
