// Package livekit provides an fx-managed testcontainers module for the
// LiveKit open-source WebRTC SFU (https://livekit.io). It is intended for the
// simulate / integration-test environment: the container is configured in
// TCP-only mode (no UDP media transport), auto-creates rooms, and accepts a
// single API key pair for token minting.
//
// Limitations:
//   - TCP-only media via port 7881. Simulcast/Dynacast behavior differs from
//     production UDP paths; use this module for control-plane (token issuance,
//     room lifecycle, webhook) testing only. For media load testing, run
//     livekit-cli load-test against a real cluster.
//   - No TURN server, no recording/egress, no cross-node signalling.
//   - API secrets must be at least 32 characters (LiveKit logs a warning
//     below that; WithAPIKey enforces it as an error).
//
// Config delivery uses LiveKit's native LIVEKIT_CONFIG env var, which accepts
// inline YAML. This intentionally deviates from the req.Files + --config
// pattern used by sibling modules (kratos, nats) because each WithX option
// needs to mutate the config incrementally, and the env var provides a
// shared staging ground across customizers.
package livekit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/narwhl/mockestra"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
	"gopkg.in/yaml.v3"
)

const (
	Tag        = "livekit"
	Image      = "livekit/livekit-server"
	SignalPort = "7880/tcp"
	RTCTCPPort = "7881/tcp"

	ContainerPrettyName = "LiveKit"

	DefaultAPIKey    = "devkey"
	DefaultAPISecret = "devkeysecretdevkeysecretdevkeysecret" // 36 chars — exceeds LiveKit's 32-char minimum.
)

// mutateConfig loads the current LIVEKIT_CONFIG env var, invokes fn to mutate
// it, and writes the result back. Returns an error if (un)marshaling fails.
func mutateConfig(req *testcontainers.GenericContainerRequest, fn func(*livekitConfig)) error {
	cfg := defaultConfig()
	if raw, ok := req.Env["LIVEKIT_CONFIG"]; ok && raw != "" {
		cfg = &livekitConfig{}
		if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
			return fmt.Errorf("failed to unmarshal existing livekit config: %w", err)
		}
	}
	fn(cfg)
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal livekit config: %w", err)
	}
	if req.Env == nil {
		req.Env = make(map[string]string)
	}
	req.Env["LIVEKIT_CONFIG"] = string(out)
	return nil
}

// WithAPIKey overrides the default API key/secret pair. LiveKit itself only
// logs a warning for secrets shorter than 32 characters, but we enforce it
// as an error here to surface misconfiguration early.
func WithAPIKey(apiKey, apiSecret string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if len(apiSecret) < 32 {
			return fmt.Errorf("livekit api secret must be at least 32 characters, got %d", len(apiSecret))
		}
		return mutateConfig(req, func(cfg *livekitConfig) {
			cfg.Keys = map[string]string{apiKey: apiSecret}
		})
	}
}

// WithWebhookURL registers a webhook receiver URL. The API key supplied must
// already be registered (via WithAPIKey or the default devkey); LiveKit uses
// it to sign webhook deliveries.
func WithWebhookURL(apiKey, url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		return mutateConfig(req, func(cfg *livekitConfig) {
			if cfg.Webhook == nil {
				cfg.Webhook = &webhookConfig{APIKey: apiKey}
			}
			cfg.Webhook.URLs = append(cfg.Webhook.URLs, url)
		})
	}
}

var WithPostReadyHook = mockestra.WithPostReadyHook

type RequestParams struct {
	fx.In
	Prefix       string                               `name:"prefix"`
	Version      string                               `name:"livekit_version"`
	RTCProxyPort int                                  `name:"livekit_rtc_proxy_port"`
	Opts         []testcontainers.ContainerCustomizer `group:"livekit"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	// Seed env with default config so options can mutate via mutateConfig.
	defaultYAML, err := yaml.Marshal(defaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default livekit config: %w", err)
	}

	r := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
			Image:        fmt.Sprintf("%s:%s", Image, p.Version),
			ExposedPorts: []string{SignalPort, RTCTCPPort},
			Env: map[string]string{
				"LIVEKIT_CONFIG": string(defaultYAML),
			},
			WaitingFor: wait.ForHTTP("/").
				WithPort(SignalPort).
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&r); err != nil {
			return nil, err
		}
	}

	// Apply the allocated RTC proxy port to the LiveKit config after all
	// user-supplied customizers, so ICE candidates advertise the correct
	// proxy address regardless of other WithListenPort calls.
	if err := mutateConfig(&r, func(cfg *livekitConfig) {
		cfg.RTC.TCPPort = p.RTCProxyPort
	}); err != nil {
		return nil, fmt.Errorf("failed to set RTC proxy port in livekit config: %w", err)
	}

	return &r, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Request   *testcontainers.GenericContainerRequest `name:"livekit"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"livekit"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("failed to create %s container: %w", ContainerPrettyName, err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			endpoint, err := c.PortEndpoint(ctx, SignalPort, "")
			if err != nil {
				return fmt.Errorf("failed to get %s endpoint: %w", ContainerPrettyName, err)
			}
			slog.Info(fmt.Sprintf("%s container is running at", ContainerPrettyName), "addr", endpoint)
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

	return Result{Container: c, ContainerGroup: c}, nil
}

var Module = mockestra.BuildContainerModule(
	Tag,
	fx.Provide(
		fx.Annotate(
			allocateRTCProxyPort,
			fx.ResultTags(`name:"livekit_rtc_proxy_port"`),
		),
		fx.Annotate(New, fx.ResultTags(`name:"livekit"`)),
		Actualize,
		fx.Annotate(
			NewProxy,
			fx.ResultTags(`name:"livekit"`),
		),
	),
)
