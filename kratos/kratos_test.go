package kratos_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra/hydra"
	"github.com/narwhl/mockestra/kratos"
	"github.com/narwhl/mockestra/mailslurper"
	"github.com/narwhl/mockestra/postgres"
	"github.com/narwhl/mockestra/proxy"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestKratosModule_SmokeTest(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"kratos"`
		}) {
			// Test public API endpoint
			publicEndpoint, err := params.Container.PortEndpoint(context.Background(), kratos.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Kratos public API endpoint: %v", err)
			}

			// Test admin API endpoint
			adminEndpoint, err := params.Container.PortEndpoint(context.Background(), kratos.AdminPort, "")
			if err != nil {
				t.Fatalf("Failed to get Kratos admin API endpoint: %v", err)
			}

			// Verify public API is accessible
			resp, err := http.Get(fmt.Sprintf("http://%s/health/ready", publicEndpoint))
			if err != nil {
				t.Fatalf("Failed to reach Kratos public API: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Kratos public API health check failed with status: %d", resp.StatusCode)
			}

			// Verify admin API is accessible
			resp2, err := http.Get(fmt.Sprintf("http://%s/health/ready", adminEndpoint))
			if err != nil {
				t.Fatalf("Failed to reach Kratos admin API: %v", err)
			}
			defer resp2.Body.Close()
			if resp2.StatusCode != http.StatusOK {
				t.Fatalf("Kratos admin API health check failed with status: %d", resp.StatusCode)
			}

			t.Logf("Kratos container is running successfully")
			t.Logf("Public API: %s", publicEndpoint)
			t.Logf("Admin API: %s", adminEndpoint)
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithPostgres(t *testing.T) {
	dsn := "postgres://user:pass@localhost:5432/kratos?sslmode=disable"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-postgres-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithPostgres(dsn),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify DSN was set
			if params.Request.Env["DSN"] != dsn {
				t.Fatalf("Expected DSN to be %s, got %s", dsn, params.Request.Env["DSN"])
			}
			t.Logf("WithPostgres decorator correctly set DSN")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithURL(t *testing.T) {
	publicURL := "https://auth.example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-url-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithURL(publicURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify URL was set
			if params.Request.Env["SERVE_PUBLIC_BASE_URL"] != publicURL {
				t.Fatalf("Expected SERVE_PUBLIC_BASE_URL to be %s, got %s", publicURL, params.Request.Env["SERVE_PUBLIC_BASE_URL"])
			}
			t.Logf("WithURL decorator correctly set public URL")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithAdminURL(t *testing.T) {
	adminURL := "https://admin.auth.example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-adminurl-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithAdminURL(adminURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify admin URL was set
			if params.Request.Env["SERVE_ADMIN_BASE_URL"] != adminURL {
				t.Fatalf("Expected SERVE_ADMIN_BASE_URL to be %s, got %s", adminURL, params.Request.Env["SERVE_ADMIN_BASE_URL"])
			}
			t.Logf("WithAdminURL decorator correctly set admin URL")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithRootDomain(t *testing.T) {
	domain := "example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-domain-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithRootDomain(domain),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify domain settings
			expectedEnvs := map[string]string{
				"SESSION_COOKIE_DOMAIN": domain,
				"COOKIES_DOMAIN": domain,
				"SELFSERVICE_METHODS_WEBAUTHN_CONFIG_RP_ID": domain,
				"SELFSERVICE_METHODS_PASSKEY_CONFIG_RP_ID": domain,
			}
			
			for key, expected := range expectedEnvs {
				if params.Request.Env[key] != expected {
					t.Fatalf("Expected %s to be %s, got %s", key, expected, params.Request.Env[key])
				}
			}
			t.Logf("WithRootDomain decorator correctly set all domain-related settings")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithRegistrationHook(t *testing.T) {
	hook := kratos.KratosRegistrationHook{
		URL:    "https://api.example.com/webhook",
		Method: "POST",
		Headers: map[string]string{
			"Authorization": "Bearer token123",
			"Content-Type":  "application/json",
		},
	}
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-reghook-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithRegistrationHook(hook),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify registration hooks for different methods
			methods := []string{"PASSWORD", "WEBAUTHN", "PASSKEY", "OIDC"}
			for _, method := range methods {
				prefix := fmt.Sprintf("SELFSERVICE_FLOWS_REGISTRATION_AFTER_%s_HOOKS_0", method)
				
				if params.Request.Env[prefix+"_HOOK"] != "web_hook" {
					t.Fatalf("Expected %s_HOOK to be web_hook", prefix)
				}
				if params.Request.Env[prefix+"_CONFIG_URL"] != hook.URL {
					t.Fatalf("Expected %s_CONFIG_URL to be %s", prefix, hook.URL)
				}
				if params.Request.Env[prefix+"_CONFIG_METHOD"] != hook.Method {
					t.Fatalf("Expected %s_CONFIG_METHOD to be %s", prefix, hook.Method)
				}
			}
			t.Logf("WithRegistrationHook decorator correctly set all registration hooks")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithSelfServiceUIURL(t *testing.T) {
	uiURL := "https://app.example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-ui-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithSelfServiceUIURL(uiURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify UI URLs
			expectedEnvs := map[string]string{
				"SELFSERVICE_DEFAULT_BROWSER_RETURN_URL":                      fmt.Sprintf("%s/", uiURL),
				"SELFSERVICE_ALLOWED_RETURN_URLS_0":                          uiURL,
				"SELFSERVICE_FLOWS_ERROR_UI_URL":                             fmt.Sprintf("%s/error", uiURL),
				"SELFSERVICE_FLOWS_SETTINGS_UI_URL":                          fmt.Sprintf("%s/settings", uiURL),
				"SELFSERVICE_FLOWS_LOGOUT_AFTER_DEFAULT_BROWSER_RETURN_URL":  fmt.Sprintf("%s/login", uiURL),
				"SELFSERVICE_FLOWS_LOGIN_UI_URL":                             fmt.Sprintf("%s/login", uiURL),
				"SELFSERVICE_FLOWS_RECOVERY_UI_URL":                          fmt.Sprintf("%s/recovery", uiURL),
				"SELFSERVICE_FLOWS_VERIFICATION_UI_URL":                      fmt.Sprintf("%s/verification", uiURL),
				"SELFSERVICE_FLOWS_VERIFICATION_AFTER_DEFAULT_BROWSER_RETURN_URL": fmt.Sprintf("%s/", uiURL),
				"SELFSERVICE_FLOWS_REGISTRATION_UI_URL":                      fmt.Sprintf("%s/registration", uiURL),
				"SELFSERVICE_METHODS_WEBAUTHN_CONFIG_RP_ORIGIN":              uiURL,
				"SELFSERVICE_METHODS_PASSKEY_CONFIG_RP_ORIGINS_0":            uiURL,
			}
			
			for key, expected := range expectedEnvs {
				if params.Request.Env[key] != expected {
					t.Fatalf("Expected %s to be %s, got %s", key, expected, params.Request.Env[key])
				}
			}
			t.Logf("WithSelfServiceUIURL decorator correctly set all UI URLs")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithOIDCConfig(t *testing.T) {
	oidcConfigs := []*kratos.OIDCConfig{
		{
			ID:           "google",
			Provider:     "google",
			IssuerURL:    "https://accounts.google.com",
			ClientID:     "google-client-id",
			ClientSecret: "google-client-secret",
			Scopes:       []string{"openid", "profile", "email"},
			MapperJsonnet: `local claims = std.extVar('claims');
{
  identity: {
    traits: {
      email: claims.email,
      name: {
        name: claims.name
      }
    }
  }
}`,
		},
		{
			ID:           "github",
			Provider:     "github",
			IssuerURL:    "https://github.com",
			ClientID:     "github-client-id",
			ClientSecret: "github-client-secret",
			Scopes:       []string{"user:email"},
			MapperJsonnet: `local claims = std.extVar('claims');
{
  identity: {
    traits: {
      email: claims.email,
      name: {
        name: claims.name
      }
    }
  }
}`,
		},
	}
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-oidc-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithOIDCConfig(oidcConfigs),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify OIDC is enabled
			if params.Request.Env["SELFSERVICE_METHODS_OIDC_ENABLED"] != "true" {
				t.Fatal("Expected OIDC to be enabled")
			}
			
			// Verify OIDC providers configuration
			for i, conf := range oidcConfigs {
				prefix := fmt.Sprintf("SELFSERVICE_METHODS_OIDC_CONFIG_PROVIDERS_%d_", i)
				
				if params.Request.Env[prefix+"ID"] != conf.ID {
					t.Fatalf("Expected provider ID to be %s", conf.ID)
				}
				if params.Request.Env[prefix+"PROVIDER"] != conf.Provider {
					t.Fatalf("Expected provider to be %s", conf.Provider)
				}
				if params.Request.Env[prefix+"ISSUER_URL"] != conf.IssuerURL {
					t.Fatalf("Expected issuer URL to be %s", conf.IssuerURL)
				}
				if params.Request.Env[prefix+"CLIENT_ID"] != conf.ClientID {
					t.Fatalf("Expected client ID to be %s", conf.ClientID)
				}
				if params.Request.Env[prefix+"CLIENT_SECRET"] != conf.ClientSecret {
					t.Fatalf("Expected client secret to be %s", conf.ClientSecret)
				}
				
				// Verify scopes
				for j, scope := range conf.Scopes {
					if params.Request.Env[prefix+fmt.Sprintf("SCOPES_%d", j)] != scope {
						t.Fatalf("Expected scope %d to be %s", j, scope)
					}
				}
			}
			t.Logf("WithOIDCConfig decorator correctly set all OIDC configurations")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithPostReadyHook(t *testing.T) {
	hookCalled := false
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-hook-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithPostReadyHook(func(endpoints map[string]string) error {
				hookCalled = true
				t.Logf("Post-ready hook called for Kratos container with endpoints: %v", endpoints)
				return nil
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"kratos"`
		}) {
			// Give some time for the hook to be called
			time.Sleep(100 * time.Millisecond)
			
			if !hookCalled {
				t.Fatal("Post-ready hook was not called")
			}
			t.Logf("WithPostReadyHook decorator successfully executed")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_ProxyConfiguration(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-proxy-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"kratos"`
			PublicProxy *proxy.TCPProxy `name:"kratos"`
			AdminProxy *proxy.TCPProxy `name:"kratosadmin"`
		}) {
			// Test that proxies are created
			if params.PublicProxy == nil {
				t.Fatal("Public API proxy was not created")
			}
			if params.AdminProxy == nil {
				t.Fatal("Admin API proxy was not created")
			}
			
			// Test proxy connectivity via localhost
			// Public API proxy should be available on localhost:4433
			_, portStr := nat.SplitProtoPort(kratos.Port)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health/ready", portStr))
			if err != nil {
				t.Fatalf("Failed to reach Kratos public API via proxy: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Kratos public API proxy health check failed with status: %d", resp.StatusCode)
			}
			
			// Admin API proxy should be available on localhost:4434
			_, adminPortStr := nat.SplitProtoPort(kratos.AdminPort)
			resp2, err := http.Get(fmt.Sprintf("http://localhost:%s/health/ready", adminPortStr))
			if err != nil {
				t.Fatalf("Failed to reach Kratos admin API via proxy: %v", err)
			}
			defer resp2.Body.Close()
			if resp2.StatusCode != http.StatusOK {
				t.Fatalf("Kratos admin API proxy health check failed with status: %d", resp.StatusCode)
			}
			
			t.Logf("Kratos proxy configuration is working correctly")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithIdentitySchema(t *testing.T) {
	// Create a temporary identity schema file
	schemaContent := `{
		"$id": "https://schemas.ory.sh/presets/kratos/identity.email.schema.json",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "Person",
		"type": "object",
		"properties": {
			"traits": {
				"type": "object",
				"properties": {
					"email": {
						"type": "string",
						"format": "email",
						"title": "E-Mail"
					}
				}
			}
		}
	}`
	
	tempFile, err := os.CreateTemp("", "identity-schema-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	
	if _, err := tempFile.WriteString(schemaContent); err != nil {
		t.Fatalf("Failed to write schema content: %v", err)
	}
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-schema-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithIdentitySchema(tempFile.Name()),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify identity schema environment variables
			if params.Request.Env["IDENTITY_DEFAULT_SCHEMA_ID"] != "default" {
				t.Fatal("Expected IDENTITY_DEFAULT_SCHEMA_ID to be 'default'")
			}
			if params.Request.Env["IDENTITY_SCHEMAS_0_ID"] != "default" {
				t.Fatal("Expected IDENTITY_SCHEMAS_0_ID to be 'default'")
			}
			if params.Request.Env["IDENTITY_SCHEMAS_0_URL"] != "file:///etc/config/kratos/identity.schema.json" {
				t.Fatal("Expected IDENTITY_SCHEMAS_0_URL to be set correctly")
			}
			
			// Verify file was added
			if len(params.Request.Files) == 0 {
				t.Fatal("Expected Files to contain the identity schema")
			}
			
			found := false
			for _, file := range params.Request.Files {
				if file.ContainerFilePath == "/etc/config/kratos/identity.schema.json" {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("Identity schema file not found in container files")
			}
			
			t.Logf("WithIdentitySchema decorator correctly configured identity schema")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithSmtpURI(t *testing.T) {
	smtpURI := "smtp://test:test@smtp.example.com:587"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-smtp-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithSmtpURI(smtpURI),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify SMTP URI was set
			if params.Request.Env["COURIER_SMTP_CONNECTION_URI"] != smtpURI {
				t.Fatalf("Expected COURIER_SMTP_CONNECTION_URI to be %s, got %s", smtpURI, params.Request.Env["COURIER_SMTP_CONNECTION_URI"])
			}
			t.Logf("WithSmtpURI decorator correctly set SMTP URI")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestKratosModule_WithSettingsHook(t *testing.T) {
	hook := kratos.KratosRegistrationHook{
		URL:    "https://api.example.com/settings-webhook",
		Method: "PUT",
		Headers: map[string]string{
			"Authorization": "Bearer settings-token",
			"Content-Type":  "application/json",
		},
	}
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kratos_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"latest-smtps",
				fx.ResultTags(`name:"mailslurper_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kratos-settingshook-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
			postgres.WithExtraDatabase(kratos.DatabaseName, "kratosuser", "kratospass"),
		),
		mailslurper.Module(),
		hydra.Module(),
		kratos.Module(
			kratos.WithSettingsHook(hook),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"kratos"`
		}) {
			// Verify settings hook configuration
			prefix := "SELFSERVICE_FLOWS_SETTINGS_AFTER_HOOKS_0"
			
			if params.Request.Env[prefix+"_HOOK"] != "web_hook" {
				t.Fatalf("Expected %s_HOOK to be web_hook", prefix)
			}
			if params.Request.Env[prefix+"_CONFIG_URL"] != hook.URL {
				t.Fatalf("Expected %s_CONFIG_URL to be %s", prefix, hook.URL)
			}
			if params.Request.Env[prefix+"_CONFIG_METHOD"] != hook.Method {
				t.Fatalf("Expected %s_CONFIG_METHOD to be %s", prefix, hook.Method)
			}
			
			// Verify the body contains uid in addition to other fields
			bodyEnv := params.Request.Env[prefix+"_CONFIG_BODY"]
			if !strings.Contains(bodyEnv, "base64://") {
				t.Fatal("Expected body to be base64 encoded")
			}
			
			t.Logf("WithSettingsHook decorator correctly set settings hook")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}