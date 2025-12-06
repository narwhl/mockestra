package rustfs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag         = "rustfs"
	Image       = "rustfs/rustfs"
	Port        = "9000/tcp"
	ConsolePort = "9001/tcp"

	ContainerPrettyName = "RustFS"
)

type RustFSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

func WithBucket(bucketName string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, ctr testcontainers.Container) error {
					endpoint, err := ctr.PortEndpoint(ctx, Port, "")
					if err != nil {
						return fmt.Errorf("encounter error getting endpoint while creating bucket: %w", err)
					}
					var creds *credentials.Credentials
					if accessKeyID, ok := req.Env["RUSTFS_ACCESS_KEY"]; ok {
						if secretAccessKey, ok := req.Env["RUSTFS_SECRET_KEY"]; ok {
							creds = credentials.NewStaticV4(accessKeyID, secretAccessKey, "")
						} else {
							return fmt.Errorf("missing RUSTFS_SECRET_KEY environment variable")
						}
					} else {
						creds = credentials.NewStaticV4("rustfsadmin", "rustfsadmin", "")
					}
					client, err := minio.New(endpoint, &minio.Options{
						Creds:  creds,
						Secure: false,
					})
					if err != nil {
						return fmt.Errorf("failed to create rustfs client: %w", err)
					}
					exists, err := client.BucketExists(ctx, bucketName)
					if err != nil {
						return fmt.Errorf("failed to check if bucket %s exists: %w", bucketName, err)
					}
					if !exists {
						err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
						if err != nil {
							return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
						}
					}
					return nil
				},
			},
		})
		return nil
	}
}

func WithObjectStorageCredentials(credentials RustFSCredentials) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["RUSTFS_ACCESS_KEY"] = credentials.AccessKeyID
		req.Env["RUSTFS_SECRET_KEY"] = credentials.SecretAccessKey
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"rustfs_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"rustfs"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			Cmd: []string{
				"/data",
				"--console-address",
				fmt.Sprintf(":%s", strings.TrimSuffix(ConsolePort, "/tcp")),
			},
			WaitingFor: wait.ForHTTP("/health").WithPort(Port),
			ExposedPorts: []string{
				Port,
				ConsolePort,
			},
			Env: make(map[string]string),
			User: "10001",
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
	Request   *testcontainers.GenericContainerRequest `name:"rustfs"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"rustfs"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			rustfsEndpoint, err := c.Endpoint(ctx, "")
			if err != nil {
				return fmt.Errorf("failed to get %s endpoint: %w", ContainerPrettyName, err)
			}
			slog.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", rustfsEndpoint)
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
			fx.ResultTags(`name:"rustfs"`),
		),
		Actualize,
	),
)