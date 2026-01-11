package registry

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
	Tag   = "registry"
	Image = "registry"
	Port  = "5000/tcp"

	ContainerPrettyName = "Docker Registry"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"registry_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"registry"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port},
			Env:          make(map[string]string),
			WaitingFor:   wait.ForHTTP("/v2/").WithPort(Port),
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

// WithDeleteEnabled enables image deletion in the registry
func WithDeleteEnabled() testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["REGISTRY_STORAGE_DELETE_ENABLED"] = "true"
		return nil
	}
}

// WithBasicAuth configures basic authentication for the registry
func WithBasicAuth(htpasswdPath string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["REGISTRY_AUTH"] = "htpasswd"
		req.Env["REGISTRY_AUTH_HTPASSWD_REALM"] = "Registry Realm"
		req.Env["REGISTRY_AUTH_HTPASSWD_PATH"] = "/auth/htpasswd"
		req.Files = append(req.Files, testcontainers.ContainerFile{
			HostFilePath:      htpasswdPath,
			ContainerFilePath: "/auth/htpasswd",
			FileMode:          0o644,
		})
		return nil
	}
}

// WithStoragePath configures a custom storage path for the registry
func WithStoragePath(path string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY"] = path
		return nil
	}
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"registry"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"registry"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			registryEndpoint, err := c.Endpoint(ctx, "")
			if err != nil {
				return fmt.Errorf("an error occurred while querying %s endpoint: %w", ContainerPrettyName, err)
			}
			slog.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", registryEndpoint)
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
			fx.ResultTags(`name:"registry"`),
		),
		Actualize,
	),
)
