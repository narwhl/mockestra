package temporal

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.uber.org/fx"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	Tag    = "temporal"
	Image  = "temporalio/auto-setup"
	Port   = "7233/tcp"
	UIPort = "8233/tcp"

	ContainerPrettyName = "Temporal"
)

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"temporal_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"temporal"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {

	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{Port, UIPort},
			WaitingFor: wait.ForHTTP("/").WithPort(UIPort).WithStatusCodeMatcher(func(status int) bool {
				return status == http.StatusOK
			}),
			Entrypoint: []string{"/usr/local/bin/temporal"},
			Cmd:        []string{"server", "start-dev", "--ip", "0.0.0.0"},
			Env:        make(map[string]string),
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
	Request   *testcontainers.GenericContainerRequest `name:"temporal"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"temporal"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			temporalPort, err := c.MappedPort(ctx, Port)
			if err != nil {
				return fmt.Errorf("unable to get %s port: %w", ContainerPrettyName, err)
			}

			temporalUiPort, err := c.MappedPort(ctx, UIPort)
			if err != nil {
				return fmt.Errorf("unable to get %s ui port: %w", ContainerPrettyName, err)
			}

			slog.Info(
				fmt.Sprintf("%s container is running", ContainerPrettyName),
				"addr", fmt.Sprintf("localhost:%s", temporalPort.Port()),
				"ui", fmt.Sprintf("localhost:%s", temporalUiPort.Port()),
			)
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

// WithNamespace registers a new Temporal namespace after the container starts.
// The namespace is created with a 72-hour workflow execution retention period,
// matching the Temporal CLI default.
func WithNamespace(name string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					addr, err := container.PortEndpoint(ctx, Port, "")
					if err != nil {
						return fmt.Errorf("failed to get temporal endpoint: %w", err)
					}
					namespaceClient, err := client.NewNamespaceClient(client.Options{
						HostPort: addr,
					})
					if err != nil {
						return fmt.Errorf("failed to create temporal namespace client: %w", err)
					}
					defer namespaceClient.Close()
					err = namespaceClient.Register(ctx, &workflowservice.RegisterNamespaceRequest{
						Namespace:                        name,
						WorkflowExecutionRetentionPeriod: durationpb.New(72 * time.Hour), // matches temporal CLI default
					})
					if err != nil {
						return fmt.Errorf("failed to register temporal namespace %s: %w", name, err)
					}
					slog.Info("Temporal namespace created", "namespace", name)
					return nil
				},
			},
		})
		return nil
	}
}

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	"temporal",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"temporal"`),
		),
		Actualize,
	),
)
