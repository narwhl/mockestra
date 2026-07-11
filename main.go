package mockestra

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
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
		// Create a copy of the base options to avoid mutating the shared slice
		result := make([]fx.Option, len(options), len(options)+len(values))
		copy(result, options)

		for _, v := range values {
			if v == nil {
				continue
			}
			result = append(result, fx.Supply(
				fx.Annotate(
					v,
					fx.As(new(testcontainers.ContainerCustomizer)),
					fx.ResultTags(fmt.Sprintf(`group:"%s"`, label)),
				),
			))
		}
		return fx.Options(result...)
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
						p, err := container.MappedPort(ctx, port)
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

// GeneratePrefix returns a random, unique prefix suitable for naming containers.
// Use it to auto-generate container names without worrying about collisions:
//
//	fx.Supply(fx.Annotate(mockestra.GeneratePrefix(), fx.ResultTags(`name:"prefix"`)))
//
// When the [prefix] dependency is not supplied to an fx app, each module's New
// function falls back to calling this automatically.
func GeneratePrefix() string {
	b := make([]byte, 4)
	io.ReadFull(rand.Reader, b)
	return fmt.Sprintf("auto-%x", b)
}

// Run creates and starts an fxtest app with the given options, registering
// automatic cleanup via t.Cleanup. It supplies fx.NopLogger so the test
// output stays clean. This eliminates the boilerplate of fxtest.New +
// RequireStart + Cleanup:
//
//	mockestra.Run(t,
//	    redis.Module(),
//	    fx.Invoke(func(c testcontainers.Container) { ... }),
//	)
func Run(t *testing.T, opts ...fx.Option) {
	t.Helper()
	app := fxtest.New(t, append([]fx.Option{fx.NopLogger}, opts...)...)
	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
