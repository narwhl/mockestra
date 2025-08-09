package minio

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
	Tag         = "minio"
	Image       = "minio/minio"
	Port        = "9000/tcp"
	ConsolePort = "9001/tcp"

	ContainerPrettyName = "Minio"
)

type MinioCredentials struct {
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
					if accessKeyID, ok := req.Env["MINIO_ROOT_USER"]; ok {
						if secretAccessKey, ok := req.Env["MINIO_ROOT_PASSWORD"]; ok {
							creds = credentials.NewStaticV4(accessKeyID, secretAccessKey, "")
						} else {
							return fmt.Errorf("missing MINIO_ROOT_PASSWORD environment variable")
						}
					} else {
						creds = credentials.NewStaticV4("minioadmin", "minioadmin", "")
					}
					client, err := minio.New(endpoint, &minio.Options{
						Creds:  creds,
						Secure: false,
					})
					if err != nil {
						return fmt.Errorf("failed to create minio client: %w", err)
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
			Name:  fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			Cmd: []string{
				"server",
				"/data",
				"--console-address",
				fmt.Sprintf(":%s", strings.TrimSuffix(ConsolePort, "/tcp")),
			},
			WaitingFor: wait.ForHTTP("/minio/health/live").WithPort(Port),
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

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"minio"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
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
			fx.ResultTags(`name:"minio"`),
		),
		Actualize,
	),
)
