package kratos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra"
	"github.com/narwhl/mockestra/hydra"
	"github.com/narwhl/mockestra/mailslurper"
	"github.com/narwhl/mockestra/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/fx"
)

const (
	Tag       = "kratos"
	Image     = "oryd/kratos"
	Port      = "4433/tcp"
	AdminPort = "4434/tcp"

	DatabaseName = "kratos"

	ContainerPrettyName = "Ory Kratos"

	// Default identity schema stub from Kratos repository
	DefaultIdentitySchema = `{
  "$id": "https://example.com/registration.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Person",
  "type": "object",
  "properties": {
    "traits": {
      "type": "object",
      "properties": {
        "bar": {
          "type": "string"
        },
        "email": {
          "type": "string",
          "ory.sh/kratos": {
            "credentials": {
              "password": {
                "identifier": true
              }
            }
          }
        }
      }
    }
  }
}`
)

type KratosRegistrationHook struct {
	URL     string
	Method  string
	Headers map[string]string
}

func WithPostgres(dsn string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["DSN"] = dsn
		return nil
	}
}

func WithURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["SERVE_PUBLIC_BASE_URL"] = url
		return nil
	}
}

func WithAdminURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["SERVE_ADMIN_BASE_URL"] = url
		return nil
	}
}

func WithRootDomain(domain string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["SESSION_COOKIE_DOMAIN"] = domain
		req.Env["COOKIES_DOMAIN"] = domain
		req.Env["SELFSERVICE_METHODS_WEBAUTHN_CONFIG_RP_ID"] = domain
		req.Env["SELFSERVICE_METHODS_PASSKEY_CONFIG_RP_ID"] = domain
		return nil
	}
}

func WithRegistrationHook(hook KratosRegistrationHook) testcontainers.CustomizeRequestOption {
	webhookRequestBody := "function(ctx) { user_id: ctx.identity.id, email: ctx.identity.traits.email, name: ctx.identity.traits.name.name }"
	return func(req *testcontainers.GenericContainerRequest) error {
		for _, method := range []string{"PASSWORD", "WEBAUTHN", "PASSKEY", "OIDC"} {
			prefix := fmt.Sprintf("SELFSERVICE_FLOWS_REGISTRATION_AFTER_%s_HOOKS_0", method)
			req.Env[fmt.Sprintf("%s_HOOK", prefix)] = "web_hook"
			req.Env[fmt.Sprintf("%s_CONFIG_URL", prefix)] = hook.URL
			req.Env[fmt.Sprintf("%s_CONFIG_METHOD", prefix)] = hook.Method
			req.Env[fmt.Sprintf("%s_CONFIG_BODY", prefix)] = fmt.Sprintf("base64://%s", base64.URLEncoding.EncodeToString([]byte(webhookRequestBody)))
			headerBytes, err := json.Marshal(hook.Headers)
			if err != nil {
				return fmt.Errorf("failed to marshal headers to JSON bytes: %w", err)
			}
			req.Env[fmt.Sprintf("%s_CONFIG_HEADERS", prefix)] = string(headerBytes)
		}
		return nil
	}
}

func WithSettingsHook(hook KratosRegistrationHook) testcontainers.CustomizeRequestOption {
	webhookRequestBody := "function(ctx) { user_id: ctx.identity.id, email: ctx.identity.traits.email, name: ctx.identity.traits.name.name, uid: ctx.identity.metadata_public.uid }"
	return func(req *testcontainers.GenericContainerRequest) error {
		prefix := "SELFSERVICE_FLOWS_SETTINGS_AFTER_HOOKS_0"
		req.Env[fmt.Sprintf("%s_HOOK", prefix)] = "web_hook"
		req.Env[fmt.Sprintf("%s_CONFIG_URL", prefix)] = hook.URL
		req.Env[fmt.Sprintf("%s_CONFIG_METHOD", prefix)] = hook.Method
		req.Env[fmt.Sprintf("%s_CONFIG_BODY", prefix)] = fmt.Sprintf("base64://%s", base64.URLEncoding.EncodeToString([]byte(webhookRequestBody)))
		headerBytes, err := json.Marshal(hook.Headers)
		if err != nil {
			return fmt.Errorf("failed to marshal headers to JSON bytes: %w", err)
		}
		req.Env[fmt.Sprintf("%s_CONFIG_HEADERS", prefix)] = string(headerBytes)
		return nil
	}
}

func WithSelfServiceUIURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["SELFSERVICE_DEFAULT_BROWSER_RETURN_URL"] = fmt.Sprintf("%s/", url)
		req.Env["SELFSERVICE_ALLOWED_RETURN_URLS_0"] = url
		req.Env["SELFSERVICE_FLOWS_ERROR_UI_URL"] = fmt.Sprintf("%s/error", url)
		req.Env["SELFSERVICE_FLOWS_SETTINGS_UI_URL"] = fmt.Sprintf("%s/settings", url)
		req.Env["SELFSERVICE_FLOWS_LOGOUT_AFTER_DEFAULT_BROWSER_RETURN_URL"] = fmt.Sprintf("%s/login", url)
		req.Env["SELFSERVICE_FLOWS_LOGIN_UI_URL"] = fmt.Sprintf("%s/login", url)
		req.Env["SELFSERVICE_FLOWS_RECOVERY_UI_URL"] = fmt.Sprintf("%s/recovery", url)
		req.Env["SELFSERVICE_FLOWS_VERIFICATION_UI_URL"] = fmt.Sprintf("%s/verification", url)
		req.Env["SELFSERVICE_FLOWS_VERIFICATION_AFTER_DEFAULT_BROWSER_RETURN_URL"] = fmt.Sprintf("%s/", url)
		req.Env["SELFSERVICE_FLOWS_REGISTRATION_UI_URL"] = fmt.Sprintf("%s/registration", url)
		req.Env["SELFSERVICE_METHODS_WEBAUTHN_CONFIG_RP_ORIGIN"] = url
		req.Env["SELFSERVICE_METHODS_PASSKEY_CONFIG_RP_ORIGINS_0"] = url
		return nil
	}
}

func WithHydraPublicURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		return nil
	}
}

func WithHydraAdminURL(url string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["OAUTH2_PROVIDER_URL"] = url
		return nil
	}
}

func WithIdentitySchema(path string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["IDENTITY_DEFAULT_SCHEMA_ID"] = "default"
		req.Env["IDENTITY_SCHEMAS_0_ID"] = "default"
		req.Env["IDENTITY_SCHEMAS_0_URL"] = "file:///etc/config/kratos/identity.schema.json"

		// If no path provided, use the default stub schema
		if path == "" {
			// Create a temporary file with the default schema
			tempFile, err := os.CreateTemp("", "kratos-identity-schema-*.json")
			if err != nil {
				return fmt.Errorf("failed to create temp file for identity schema: %w", err)
			}
			defer tempFile.Close()

			if _, err := tempFile.Write([]byte(DefaultIdentitySchema)); err != nil {
				return fmt.Errorf("failed to write default identity schema: %w", err)
			}

			path = tempFile.Name()
		}

		containerFile := testcontainers.ContainerFile{
			HostFilePath:      path,
			ContainerFilePath: "/etc/config/kratos/identity.schema.json",
			FileMode:          0o644,
		}
		req.Files = append(req.Files, containerFile)
		return nil
	}
}

func WithSmtpURI(uri string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["COURIER_SMTP_CONNECTION_URI"] = uri
		return nil
	}
}

type OIDCConfig struct {
	ID            string   `json:"id"`
	Provider      string   `json:"provider"`
	IssuerURL     string   `json:"issuer_url"`
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	Scopes        []string `json:"scopes"`
	MapperJsonnet string   `json:"mapper_jsonnet"`
}

func WithOIDCConfig(config []*OIDCConfig) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Env["SELFSERVICE_METHODS_OIDC_ENABLED"] = "true"
		for i, conf := range config {
			envPrefix := fmt.Sprintf("SELFSERVICE_METHODS_OIDC_CONFIG_PROVIDERS_%d_", i)
			req.Env[envPrefix+"ID"] = conf.ID
			req.Env[envPrefix+"PROVIDER"] = conf.Provider
			req.Env[envPrefix+"ISSUER_URL"] = conf.IssuerURL
			req.Env[envPrefix+"CLIENT_ID"] = conf.ClientID
			req.Env[envPrefix+"CLIENT_SECRET"] = conf.ClientSecret
			scopeBytes, err := json.Marshal(conf.Scopes)
			if err != nil {
				return fmt.Errorf("failed to marshal scopes to JSON bytes: %w", err)
			}
			req.Env[envPrefix+"SCOPE"] = string(scopeBytes)
			req.Env[envPrefix+"MAPPER_URL"] = fmt.Sprintf("base64://%s", base64.URLEncoding.EncodeToString([]byte(conf.MapperJsonnet)))
		}
		return nil
	}
}

type RequestParams struct {
	fx.In
	Prefix  string                               `name:"prefix"`
	Version string                               `name:"kratos_version"`
	Opts    []testcontainers.ContainerCustomizer `group:"kratos"`
}

func New(p RequestParams) (*testcontainers.GenericContainerRequest, error) {
	kratosCookieSecret, err := mockestra.RandomPassword(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate kratos cookie secret: %w", err)
	}
	kratosSystemSecret, err := mockestra.RandomPassword(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate kratos system secret: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Name:         fmt.Sprintf("mock-%s-%s", p.Prefix, Tag),
		Image:        fmt.Sprintf("%s:%s", Image, p.Version),
		ExposedPorts: []string{Port, AdminPort},
		Env: map[string]string{
			"SECRETS_COOKIE_0":                                           kratosCookieSecret,
			"SECRETS_CIPHER_0":                                           kratosSystemSecret,
			"SERVE_PUBLIC_CORS_ENABLED":                                  "true",
			"SESSION_WHOAMI_REQUIRED_AAL":                                "aal1",
			"SELFSERVICE_DEFAULT_BROWSER_RETURN_URL":                     "http://localhost:3000/",
			"SELFSERVICE_METHODS_PASSWORD_ENABLED":                       "true",
			"SELFSERVICE_METHODS_WEBAUTHN_ENABLED":                       "true",
			"SELFSERVICE_METHODS_WEBAUTHN_CONFIG_PASSWORDLESS":           "true",
			"SELFSERVICE_METHODS_WEBAUTHN_CONFIG_RP_DISPLAY_NAME":        "Your Application name",
			"SELFSERVICE_METHODS_WEBAUTHN_CONFIG_RP_ID":                  "localhost",
			"SELFSERVICE_METHODS_PASSKEY_ENABLED":                        "true",
			"SELFSERVICE_METHODS_PASSKEY_CONFIG_RP_DISPLAY_NAME":         "Your Application name",
			"SELFSERVICE_METHODS_PASSKEY_CONFIG_RP_ID":                   "localhost",
			"IDENTITY_DEFAULT_SCHEMA_ID":                                 "default",
			"IDENTITY_SCHEMAS_0_ID":                                      "default",
			"IDENTITY_SCHEMAS_0_URL":                                     "file:///etc/config/kratos/identity.schema.json",
			"SELFSERVICE_FLOWS_SETTINGS_PRIVILEGED_SESSION_MAX_AGE":      "15m",
			"SELFSERVICE_FLOWS_SETTINGS_REQUIRED_AAL":                    "aal1",
			"SELFSERVICE_FLOWS_RECOVERY_ENABLED":                         "true",
			"SELFSERVICE_FLOWS_RECOVERY_USE":                             "code",
			"SELFSERVICE_FLOWS_VERIFICATION_ENABLED":                     "false",
			"SELFSERVICE_FLOWS_VERIFICATION_USE":                         "code",
			"SELFSERVICE_FLOWS_LOGIN_LIFESPAN":                           "10m",
			"SELFSERVICE_FLOWS_REGISTRATION_ENABLE_LEGACY_ONE_STEP":      "false",
			"SELFSERVICE_FLOWS_REGISTRATION_LIFESPAN":                    "10m",
			"SELFSERVICE_FLOWS_REGISTRATION_AFTER_PASSKEY_HOOKS_0_HOOK":  "session",
			"SELFSERVICE_FLOWS_REGISTRATION_AFTER_WEBAUTHN_HOOKS_0_HOOK": "session",
			"SELFSERVICE_FLOWS_REGISTRATION_AFTER_PASSWORD_HOOKS_0_HOOK": "session",
			"SELFSERVICE_FLOWS_REGISTRATION_AFTER_PASSWORD_HOOKS_1_HOOK": "show_verification_ui",
			"LOG_LEVEL":                 "debug",
			"LOG_FORMAT":                "text",
			"LOG_LEAK_SENSITIVE_VALUES": "true",
			"CIPHERS_ALGORITHM":         "xchacha20-poly1305",
			"HASHERS_ALGORITHM":         "bcrypt",
			"HASHERS_BCRYPT_COST":       "8",
		},
		Files:      []testcontainers.ContainerFile{},
		WaitingFor: wait.ForHTTP("/health/ready").WithPort(AdminPort),
		Cmd:        []string{"serve", "--dev", "--watch-courier"},
	}

	genericContainerReq := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	// Apply WithIdentitySchema with default schema before other options
	if err := WithIdentitySchema("").Customize(&genericContainerReq); err != nil {
		return nil, fmt.Errorf("failed to set default identity schema: %w", err)
	}

	for _, opt := range p.Opts {
		if err := opt.Customize(&genericContainerReq); err != nil {
			return nil, err
		}
	}

	return &genericContainerReq, nil
}

type ContainerParams struct {
	fx.In
	Lifecycle                fx.Lifecycle
	Prefix                   string                                  `name:"prefix"`
	HydraContainer           testcontainers.Container                `name:"hydra"`
	MailslurperContainer     testcontainers.Container                `name:"mailslurper"`
	PostgresContainerRequest *testcontainers.GenericContainerRequest `name:"postgres"`
	PostgresContainer        testcontainers.Container                `name:"postgres"`
	Request                  *testcontainers.GenericContainerRequest `name:"kratos"`
}

type Result struct {
	fx.Out
	Container      testcontainers.Container `name:"kratos"`
	ContainerGroup testcontainers.Container `group:"containers"`
}

func Actualize(p ContainerParams) (Result, error) {
	hydraIP, err := p.HydraContainer.ContainerIP(context.Background())
	if err != nil {
		return Result{}, fmt.Errorf("failed to get %s container IP: %w", hydra.ContainerPrettyName, err)
	}
	_, hydraAdminPort := nat.SplitProtoPort(hydra.AdminPort)

	mailslurperIP, err := p.MailslurperContainer.ContainerIP(context.Background())
	if err != nil {
		return Result{}, fmt.Errorf("failed to get %s container IP: %w", mailslurper.ContainerPrettyName, err)
	}
	_, mailslurperPort := nat.SplitProtoPort(mailslurper.SMTPPort)

	postgresIP, err := p.PostgresContainer.ContainerIP(context.Background())
	if err != nil {
		return Result{}, fmt.Errorf("failed to get %s container IP: %w", postgres.ContainerPrettyName, err)
	}
	_, postgresPort := nat.SplitProtoPort(postgres.Port)

	if err := WithHydraAdminURL(fmt.Sprintf("http://%s:%s", hydraIP, hydraAdminPort)).Customize(p.Request); err != nil {
		return Result{}, fmt.Errorf("failed to set hydra url: %w", err)
	}

	if err := WithPostgres(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		p.PostgresContainerRequest.Env["POSTGRES_USER"], // TODO: use database specific user instead of admin user
		p.PostgresContainerRequest.Env["POSTGRES_PASSWORD"],
		postgresIP,
		postgresPort,
		DatabaseName,
	)).Customize(p.Request); err != nil {
		return Result{}, fmt.Errorf("failed to set postgres url: %w", err)
	}

	if err := WithSmtpURI(fmt.Sprintf("smtps://test:test@%s:%s?skip_ssl_verify=true",
		mailslurperIP,
		mailslurperPort,
	)).Customize(p.Request); err != nil {
		return Result{}, fmt.Errorf("failed to set smtp url: %w", err)
	}

	migrateGenericContainerReq := *p.Request
	migrateGenericContainerReq.ContainerRequest.Name = fmt.Sprintf("mock-%s-kratos-migrate", p.Prefix)
	migrateGenericContainerReq.ContainerRequest.Cmd = []string{"migrate", "sql", "-e", "--yes"}
	migrateGenericContainerReq.ContainerRequest.WaitingFor = wait.ForExit()
	migrateGenericContainerReq.LifecycleHooks = []testcontainers.ContainerLifecycleHooks{}

	migrateContainer, err := testcontainers.GenericContainer(context.Background(), migrateGenericContainerReq)
	if err != nil {
		return Result{}, fmt.Errorf("failed to run %s migration: %w", ContainerPrettyName, err)
	}
	if err := migrateContainer.Terminate(context.Background()); err != nil {
		slog.Warn(fmt.Sprintf("an error occurred while terminating %s migration container", ContainerPrettyName), "error", err)
	}

	c, err := testcontainers.GenericContainer(context.Background(), *p.Request)
	if err != nil {
		return Result{}, fmt.Errorf("an error occurred while instantiating %s container: %w", ContainerPrettyName, err)
	}
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			portLabels := map[string]string{
				Port:      "API",
				AdminPort: "Admin API",
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
			New,
			fx.ResultTags(`name:"kratos"`),
		),
		Actualize,
		fx.Annotate(
			NewProxy("Public API", nat.Port(Port)),
			fx.ResultTags(`name:"kratos"`),
		),
		fx.Annotate(
			NewProxy("Admin API", nat.Port(AdminPort)),
			fx.ResultTags(`name:"kratosadmin"`),
		),
	),
)
