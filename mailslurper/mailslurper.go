package mailslurper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag      = "mailslurper"
	Image    = "oryd/mailslurper"
	Port     = "4436/tcp"
	SMTPPort = "1025/tcp"

	ContainerPrettyName = "Mailslurper"

	configFilePath = "/go/src/github.com/mailslurper/mailslurper/cmd/mailslurper/config.json"
)

type mailslurperConfig struct {
	WwwAddress       string `json:"wwwAddress"`
	WwwPort          int    `json:"wwwPort"`
	ServiceAddress   string `json:"serviceAddress"`
	ServicePort      int    `json:"servicePort"`
	SmtpAddress      string `json:"smtpAddress"`
	SmtpPort         int    `json:"smtpPort"`
	DbEngine         string `json:"dbEngine"`
	DbHost           string `json:"dbHost"`
	DbPort           int    `json:"dbPort"`
	DbDatabase       string `json:"dbDatabase"`
	DbUserName       string `json:"dbUserName"`
	DbPassword       string `json:"dbPassword"`
	MaxWorkers       int    `json:"maxWorkers"`
	AutoStartBrowser bool   `json:"autoStartBrowser"`
	KeyFile          string `json:"keyFile"`
	CertFile         string `json:"certFile"`
	AdminKeyFile     string `json:"adminKeyFile"`
	AdminCertFile    string `json:"adminCertFile"`
}

func allocateAPIProxyPort() (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(mockestra.LoopbackAddress, "0"))
	if err != nil {
		return 0, fmt.Errorf("failed to allocate free port for %s API proxy: %w", ContainerPrettyName, err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	slog.Info(fmt.Sprintf("Allocated dynamic API proxy port for %s", ContainerPrettyName), "port", port)
	return port, nil
}

type RequestParams struct {
	fx.In
	Prefix       string                               `name:"prefix"`
	Version      string                               `name:"mailslurper_version"`
	APIProxyPort int                                  `name:"mailslurper_api_proxy_port"`
	Opts         []testcontainers.ContainerCustomizer `group:"mailslurper"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	cfg := mailslurperConfig{
		WwwAddress:     "0.0.0.0",
		WwwPort:        4436,
		ServiceAddress: "0.0.0.0",
		ServicePort:    p.APIProxyPort,
		SmtpAddress:    "0.0.0.0",
		SmtpPort:       1025,
		DbEngine:       "SQLite",
		DbDatabase:     "mailslurper.db",
		MaxWorkers:     1000,
		KeyFile:        "mailslurper-key.pem",
		CertFile:       "mailslurper-cert.pem",
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mailslurper config: %w", err)
	}

	apiPort := fmt.Sprintf("%d/tcp", p.APIProxyPort)
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, apiPort, SMTPPort},
			WaitingFor:   wait.ForHTTP("/").WithPort(Port),
			Files: []testcontainers.ContainerFile{
				{
					Reader:            bytes.NewReader(configJSON),
					ContainerFilePath: configFilePath,
					FileMode:          0644,
				},
			},
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
	Lifecycle    fx.Lifecycle
	Request      *testcontainers.GenericContainerRequest `name:"mailslurper"`
	APIProxyPort int                                     `name:"mailslurper_api_proxy_port"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"mailslurper"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("an error occurred while instantiating mailslurper container: %w", err)
	}
	apiPort := fmt.Sprintf("%d/tcp", p.APIProxyPort)
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port:     "dashboard",
				apiPort:  "api",
				SMTPPort: "SMTP",
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
			allocateAPIProxyPort,
			fx.ResultTags(`name:"mailslurper_api_proxy_port"`),
		),
		fx.Annotate(
			New,
			fx.ResultTags(`name:"mailslurper"`),
		),
		Actualize,
		fx.Annotate(
			NewProxy,
			fx.ResultTags(`name:"mailslurper"`),
		),
	),
)
