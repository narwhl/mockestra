package mockestra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
)

const (
	LoopbackAddress = "127.0.0.1"
)

// ContainerModule is a representation of the returned
// higher order function for wrapping testcontainers.ContainerCustomizer with fx.Option.
type ContainerModule func(values ...testcontainers.ContainerCustomizer) fx.Option

// ContainerPostReadyHook is a representation of the returned
// higher order function for hooking function after the container is ready.
type ContainerPostReadyHook func(endpoints map[string]string) error

// BuildContainerModule decorates the fx.Option with the testcontainers.ContainerCustomizer.
// {label} is for tagging incoming testcontainers.ContainerCustomizer with ResultTags.
func BuildContainerModule(label string, options ...fx.Option) ContainerModule {
	return func(values ...testcontainers.ContainerCustomizer) fx.Option {
		for _, v := range values {
			if v == nil {
				continue
			}
			options = append(options, fx.Supply(
				fx.Annotate(
					v,
					fx.As(new(testcontainers.ContainerCustomizer)),
					fx.ResultTags(fmt.Sprintf(`group:"%s"`, label)),
				),
			))
		}
		return fx.Options(options...)
	}
}

// WithPostReadyHook generalizes the use case for hooking function
// after the container is ready. It extrapolates exposed ports specified
// in testcontainers.ContainerRequest and transform them into a map of host:port.
func WithPostReadyHook(fn ContainerPostReadyHook) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				func(ctx context.Context, container testcontainers.Container) error {
					endpoints := make(map[string]string)
					for _, port := range req.ExposedPorts {
						p, err := container.MappedPort(ctx, nat.Port(port))
						if err != nil {
							return fmt.Errorf("encounter error getting addr: %w", err)
						}
						endpoints[port] = fmt.Sprintf("localhost:%s", p.Port())
					}
					return fn(endpoints)
				},
			},
		})
		return nil
	}
}

// Versions takes a map of string key and value as container name and its corresponding
// published image version. It returns a slice of fx.Option that can be used
// to supply the version of the container image.
func Versions(m map[string]string) []fx.Option {
	var opts []fx.Option
	for k, v := range m {
		opts = append(opts, fx.Supply(
			fx.Annotate(
				v,
				fx.ResultTags(fmt.Sprintf(`name:"%s_version"`, k)),
			),
		))
	}
	return opts
}

func RandomPassword(length uint) (string, error) {
	passwordBytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, passwordBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(passwordBytes), nil
}

func Secrets(spec map[string]uint) (map[string]string, error) {
	secrets := make(map[string]string)
	for name, length := range spec {
		secret, err := RandomPassword(length)
		if err != nil {
			return nil, fmt.Errorf("failed to generate secret %s: %w", name, err)
		}
		secrets[name] = secret
	}
	return secrets, nil
}
