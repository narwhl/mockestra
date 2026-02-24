package lgtm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag            = "lgtm"
	Image          = "grafana/otel-lgtm"
	GrafanaPort    = "3000/tcp"
	LokiPort       = "3100/tcp"
	TempoPort      = "3200/tcp"
	OtlpGrpcPort   = "4317/tcp"
	OtlpHttpPort   = "4318/tcp"
	PrometheusPort = "9090/tcp"

	ContainerPrettyName = "LGTM"
)


// WithDashboard provisions a Grafana dashboard from a JSON string.
// Each call adds a separate dashboard to the LGTM Grafana instance.
// Dashboard represents a Grafana dashboard to provision.
type Dashboard struct {
	Name string
	JSON string
}

func WithDashboard(dashboards ...Dashboard) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		provisioningYAML := `apiVersion: 1

providers:
  - name: "custom-dashboards"
    type: file
    options:
      path: /otel-lgtm/dashboards
      foldersFromFilesStructure: false
`
		req.Files = append(req.Files, testcontainers.ContainerFile{
			Reader:            strings.NewReader(provisioningYAML),
			ContainerFilePath: "/otel-lgtm/grafana/conf/provisioning/dashboards/custom-dashboards.yaml",
			FileMode:          0o644,
		})

		for _, d := range dashboards {
			req.Files = append(req.Files, testcontainers.ContainerFile{
				Reader:            strings.NewReader(d.JSON),
				ContainerFilePath: fmt.Sprintf("/otel-lgtm/dashboards/%s.json", d.Name),
				FileMode:          0o644,
			})
		}
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"lgtm_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"lgtm"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{
				GrafanaPort,
				LokiPort,
				TempoPort,
				OtlpGrpcPort,
				OtlpHttpPort,
				PrometheusPort,
			},
			WaitingFor: wait.ForLog(".*The OpenTelemetry collector and the Grafana LGTM stack are up and running.*\\s").AsRegexp().WithOccurrence(1),
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
	Request   *testcontainers.GenericContainerRequest `name:"lgtm"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"lgtm"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, err
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {

			portLabels := map[string]string{
				GrafanaPort:    "grafana",
				LokiPort:       "loki",
				TempoPort:      "tempo",
				OtlpGrpcPort:   "otlp (gRPC)",
				OtlpHttpPort:   "otlp (HTTP)",
				PrometheusPort: "prometheus",
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

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"lgtm"`),
		),
		Actualize,
	),
)
