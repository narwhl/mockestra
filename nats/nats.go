package nats

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
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

	// TLS-related container labels for PostReady hooks
	tlsEnabledLabel      = "mockestra.nats.tls.enabled"
	tlsInsecureSkipLabel = "mockestra.nats.tls.insecure_skip_verify"
	tlsVerifyLabel       = "mockestra.nats.tls.verify"
	tlsCAPathLabel       = "mockestra.nats.tls.ca_path"
	tlsCertPathLabel     = "mockestra.nats.tls.cert_path"
	tlsKeyPathLabel      = "mockestra.nats.tls.key_path"

	// Container paths for TLS certificates
	containerCertPath = "/etc/nats/certs/server.pem"
	containerKeyPath  = "/etc/nats/certs/server-key.pem"
	containerCAPath   = "/etc/nats/certs/ca.pem"
)

// TLSConfig holds TLS certificate configuration for the NATS server
type TLSConfig struct {
	// Certificate - either CertFile (path) or CertReader (io.Reader)
	CertFile   string
	CertReader io.Reader

	// Private key - either KeyFile (path) or KeyReader (io.Reader)
	KeyFile   string
	KeyReader io.Reader

	// CA certificate (optional) - either CAFile (path) or CAReader (io.Reader)
	CAFile   string
	CAReader io.Reader

	// Verify requires and verifies client certificates (mTLS)
	Verify bool

	// InsecureSkipVerify for client connections in PostReady hooks
	// Useful when using self-signed certificates
	InsecureSkipVerify bool
}

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

// WithTLS configures TLS for the NATS server by mounting certificates and enabling TLS mode
func WithTLS(config TLSConfig) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		// Initialize labels map if nil
		if req.Labels == nil {
			req.Labels = make(map[string]string)
		}
		req.Labels[tlsEnabledLabel] = "true"
		if config.InsecureSkipVerify {
			req.Labels[tlsInsecureSkipLabel] = "true"
		}

		// Mount certificate (Reader or File)
		if config.CertReader != nil {
			req.Files = append(req.Files, testcontainers.ContainerFile{
				Reader:            config.CertReader,
				ContainerFilePath: containerCertPath,
				FileMode:          0644,
			})
		} else if config.CertFile != "" {
			req.Files = append(req.Files, testcontainers.ContainerFile{
				HostFilePath:      config.CertFile,
				ContainerFilePath: containerCertPath,
				FileMode:          0644,
			})
		}

		// Mount key (Reader or File)
		if config.KeyReader != nil {
			req.Files = append(req.Files, testcontainers.ContainerFile{
				Reader:            config.KeyReader,
				ContainerFilePath: containerKeyPath,
				FileMode:          0600,
			})
		} else if config.KeyFile != "" {
			req.Files = append(req.Files, testcontainers.ContainerFile{
				HostFilePath:      config.KeyFile,
				ContainerFilePath: containerKeyPath,
				FileMode:          0600,
			})
		}

		// Add TLS arguments
		req.Cmd = append(req.Cmd, "--tls", "--tlscert="+containerCertPath, "--tlskey="+containerKeyPath)

		// Store cert and key paths for hooks (used for verification and mTLS client auth)
		req.Labels[tlsCertPathLabel] = containerCertPath
		req.Labels[tlsKeyPathLabel] = containerKeyPath

		// Optional CA certificate
		if config.CAReader != nil || config.CAFile != "" {
			if config.CAReader != nil {
				req.Files = append(req.Files, testcontainers.ContainerFile{
					Reader:            config.CAReader,
					ContainerFilePath: containerCAPath,
					FileMode:          0644,
				})
			} else {
				req.Files = append(req.Files, testcontainers.ContainerFile{
					HostFilePath:      config.CAFile,
					ContainerFilePath: containerCAPath,
					FileMode:          0644,
				})
			}
			req.Cmd = append(req.Cmd, "--tlscacert="+containerCAPath)
			req.Labels[tlsCAPathLabel] = containerCAPath
		}

		if config.Verify {
			req.Cmd = append(req.Cmd, "--tlsverify")
			req.Labels[tlsVerifyLabel] = "true"
		}

		return nil
	}
}

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

// connectToNATS creates a NATS connection, auto-detecting TLS from container labels.
// If TLS is enabled, it copies certs from the container for verification.
// If mTLS (Verify) is enabled, it also presents the server cert as client credentials.
func connectToNATS(ctx context.Context, container testcontainers.Container) (*natsgo.Conn, error) {
	endpoint, err := container.PortEndpoint(ctx, Port, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get NATS endpoint: %w", err)
	}

	inspect, err := container.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}
	labels := inspect.Config.Labels

	var opts []natsgo.Option
	if labels[tlsEnabledLabel] == "true" {
		endpoint = "tls://" + endpoint

		if labels[tlsInsecureSkipLabel] == "true" {
			opts = append(opts, natsgo.Secure(&tls.Config{InsecureSkipVerify: true}))
		} else {
			tlsConfig := &tls.Config{}

			// Copy CA cert (or server cert for self-signed) from container for server verification
			caCertPath := labels[tlsCAPathLabel]
			if caCertPath == "" {
				caCertPath = labels[tlsCertPathLabel] // Fall back to server cert if no CA
			}

			if caCertPath != "" {
				caCertPEM, err := copyFileFromContainer(ctx, container, caCertPath)
				if err != nil {
					return nil, fmt.Errorf("failed to copy CA cert: %w", err)
				}

				certPool := x509.NewCertPool()
				if !certPool.AppendCertsFromPEM(caCertPEM) {
					return nil, fmt.Errorf("failed to parse CA cert")
				}
				tlsConfig.RootCAs = certPool
			}

			// If mTLS is enabled, present server cert as client credentials
			if labels[tlsVerifyLabel] == "true" {
				certPath := labels[tlsCertPathLabel]
				keyPath := labels[tlsKeyPathLabel]

				certPEM, err := copyFileFromContainer(ctx, container, certPath)
				if err != nil {
					return nil, fmt.Errorf("failed to copy client cert: %w", err)
				}

				keyPEM, err := copyFileFromContainer(ctx, container, keyPath)
				if err != nil {
					return nil, fmt.Errorf("failed to copy client key: %w", err)
				}

				clientCert, err := tls.X509KeyPair(certPEM, keyPEM)
				if err != nil {
					return nil, fmt.Errorf("failed to load client cert: %w", err)
				}
				tlsConfig.Certificates = []tls.Certificate{clientCert}
			}

			opts = append(opts, natsgo.Secure(tlsConfig))
		}
	}

	return natsgo.Connect(endpoint, opts...)
}

// copyFileFromContainer copies a file from the container and returns its contents
func copyFileFromContainer(ctx context.Context, container testcontainers.Container, path string) ([]byte, error) {
	reader, err := container.CopyFileFromContainer(ctx, path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// WithStream creates a JetStream stream after the container starts
func WithStream(config StreamConfig) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					nc, err := connectToNATS(ctx, container)
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
					nc, err := connectToNATS(ctx, container)
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
