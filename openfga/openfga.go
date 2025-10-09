package openfga

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
	language "github.com/openfga/language/pkg/go/transformer"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	Image               = "openfga/openfga"
	PlaygroundPort      = "3000/tcp"
	HttpPort            = "8080/tcp"
	GrpcPort            = "8081/tcp"
	ContainerPrettyName = "OpenFGA"
)

type callback func(string, string) error

func WithAuthorizationModel(model string, cb callback) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					addr, err := container.PortEndpoint(ctx, HttpPort, "")
					if err != nil {
						return fmt.Errorf("encounter error getting addr: %w", err)
					}
					parsedAuthModel, err := language.TransformDSLToProto(model)
					if err != nil {
						return fmt.Errorf("failed to transform due to %w", err)
					}

					bytes, err := protojson.Marshal(parsedAuthModel)
					if err != nil {
						return fmt.Errorf("failed to transform due to %w", err)
					}

					jsonAuthModel := openfga.AuthorizationModel{}
					err = json.Unmarshal(bytes, &jsonAuthModel)
					if err != nil {
						return fmt.Errorf("failed to transform due to %w", err)
					}
					fgaClient, err := client.NewSdkClient(&client.ClientConfiguration{
						ApiUrl: fmt.Sprintf("http://%s", addr),
						Credentials: &credentials.Credentials{
							Method: credentials.CredentialsMethodApiToken,
							Config: &credentials.Config{
								ApiToken: req.Env["OPENFGA_AUTHN_PRESHARED_KEYS"],
							},
						},
					})
					if err != nil {
						return fmt.Errorf("failed to create OpenFGA client: %w", err)
					}
					writeAuthModelReq := openfga.NewWriteAuthorizationModelRequest(
						jsonAuthModel.TypeDefinitions,
						jsonAuthModel.SchemaVersion,
					)
					storeCreationResp, err := fgaClient.CreateStore(ctx).Body(client.ClientCreateStoreRequest{Name: "default"}).Execute()
					if err != nil {
						return fmt.Errorf("failed to create OpenFGA store: %w", err)
					}
					authModelCreationResp, err := fgaClient.WriteAuthorizationModel(ctx).
						Body(*writeAuthModelReq).
						Options(client.ClientWriteAuthorizationModelOptions{
							StoreId: &storeCreationResp.Id,
						}).Execute()
					if err != nil {
						return fmt.Errorf("failed to write authorization model: %w", err)
					}
					return cb(storeCreationResp.Id, authModelCreationResp.AuthorizationModelId)
				},
			},
		})
		return nil
	}
}

func WithPresharedKey(token string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["OPENFGA_AUTHN_METHOD"] = "preshared"
		req.Env["OPENFGA_AUTHN_PRESHARED_KEYS"] = token
		return nil
	}
}

func WithPlayground() testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Cmd = append(req.Cmd, "--playground-enabled")
		req.ExposedPorts = append(req.ExposedPorts, PlaygroundPort)
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"openfga_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"openfga"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:  fmt.Sprintf("mock-%s-openfga", p.Prefix),
			Image: fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{
				HttpPort,
				GrpcPort,
			},
			Cmd: []string{"run"},
			Env: map[string]string{},
			WaitingFor: wait.ForAll(
				wait.ForHTTP("/healthz").WithPort(HttpPort).WithResponseMatcher(func(r io.Reader) bool {
					bs, err := io.ReadAll(r)
					if err != nil {
						return false
					}

					return (strings.Contains(string(bs), "SERVING"))
				}),
				wait.ForHTTP("/playground").WithPort(PlaygroundPort).WithStatusCodeMatcher(func(status int) bool {
					return status == http.StatusOK
				}),
			),
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&req); err != nil {
			return nil, fmt.Errorf("failed to customize OpenFGA container: %w", err)
		}
	}
	return &req, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"openfga"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"openfga"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(
		context.Background(),
		*p.Request,
	)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create OpenFGA container: %w", err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				PlaygroundPort: "playground",
				GrpcPort:       "gRPC",
				HttpPort:       "HTTP",
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

var WithPostReadyHook = mockestra.WithPostReadyHook

var Module = mockestra.BuildContainerModule(
	"openfga",
	fx.Provide(
		fx.Annotate(
			New,
			fx.ResultTags(`name:"openfga"`),
		),
		Actualize,
	),
)
