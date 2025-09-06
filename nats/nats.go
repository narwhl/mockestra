package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/nats"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag       = "nats"
	Image     = "nats"
	Port      = "4222/tcp"
	HttpPort  = "8222/tcp"
	RoutePort = "6222/tcp"

	ContainerPrettyName = "NATS Server"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"nats_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"nats"`
}

var WithUsername = nats.WithUsername
var WithPassword = nats.WithPassword
var WithArgument = nats.WithArgument
var WithConfigFile = nats.WithConfigFile

// WithJetStreamStorageDir configures the storage directory for JetStream
func WithJetStreamStorageDir(dir string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Cmd = append(req.Cmd, "-sd", dir)
		return nil
	}
}

// WithJetStreamDomain configures the JetStream domain for isolation
func WithJetStreamDomain(domain string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Cmd = append(req.Cmd, "-jetstream_domain", domain)
		return nil
	}
}

// StreamConfig defines the configuration for a JetStream stream
type StreamConfig struct {
	Name         string
	Subjects     []string
	Retention    jetstream.RetentionPolicy
	MaxAge       time.Duration
	MaxBytes     int64
	MaxMsgs      int64
	MaxMsgSize   int32
	Replicas     int
	NoAck        bool
	Discard      jetstream.DiscardPolicy
	MaxConsumers int
	Storage      jetstream.StorageType
	Description  string
}

// WithStream creates a JetStream stream after the container starts
func WithStream(config StreamConfig) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					endpoint, err := container.PortEndpoint(ctx, Port, "")
					if err != nil {
						return fmt.Errorf("failed to get NATS endpoint: %w", err)
					}

					nc, err := natsgo.Connect(endpoint)
					if err != nil {
						return fmt.Errorf("failed to connect to NATS: %w", err)
					}
					defer nc.Close()

					js, err := jetstream.New(nc)
					if err != nil {
						return fmt.Errorf("failed to create JetStream context: %w", err)
					}

					streamConfig := jetstream.StreamConfig{
						Name:        config.Name,
						Subjects:    config.Subjects,
						Description: config.Description,
					}

					// Set optional fields only if they have non-zero values
					if config.Retention != 0 {
						streamConfig.Retention = config.Retention
					}
					if config.MaxAge > 0 {
						streamConfig.MaxAge = config.MaxAge
					}
					if config.MaxBytes > 0 {
						streamConfig.MaxBytes = config.MaxBytes
					}
					if config.MaxMsgs > 0 {
						streamConfig.MaxMsgs = config.MaxMsgs
					}
					if config.MaxMsgSize > 0 {
						streamConfig.MaxMsgSize = config.MaxMsgSize
					}
					if config.Replicas > 0 {
						streamConfig.Replicas = config.Replicas
					}
					if config.NoAck {
						streamConfig.NoAck = config.NoAck
					}
					if config.Discard != 0 {
						streamConfig.Discard = config.Discard
					}
					if config.MaxConsumers > 0 {
						streamConfig.MaxConsumers = config.MaxConsumers
					}
					if config.Storage != 0 {
						streamConfig.Storage = config.Storage
					}

					_, err = js.CreateStream(ctx, streamConfig)
					if err != nil {
						return fmt.Errorf("failed to create JetStream stream %q: %w", config.Name, err)
					}

					slog.Info("JetStream stream created", "stream", config.Name, "subjects", config.Subjects)
					return nil
				},
			},
		})
		return nil
	}
}

// WithJetStreamCallback allows custom JetStream setup after the container starts
func WithJetStreamCallback(fn func(context.Context, jetstream.JetStream) error) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					endpoint, err := container.PortEndpoint(ctx, Port, "")
					if err != nil {
						return fmt.Errorf("failed to get NATS endpoint: %w", err)
					}

					nc, err := natsgo.Connect(endpoint)
					if err != nil {
						return fmt.Errorf("failed to connect to NATS: %w", err)
					}
					defer nc.Close()

					js, err := jetstream.New(nc)
					if err != nil {
						return fmt.Errorf("failed to create JetStream context: %w", err)
					}

					return fn(ctx, js)
				},
			},
		})
		return nil
	}
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, HttpPort, RoutePort},
			Env:          map[string]string{},
			Cmd:          []string{"-DV", "-js"},
			WaitingFor:   wait.ForListeningPort(Port),
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
	Request   *testcontainers.GenericContainerRequest `name:"nats"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"nats"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port:      "client",
				HttpPort:  "http",
				RoutePort: "route",
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

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"nats"`),
		),
		Actualize,
	),
)
