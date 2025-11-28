package hydra_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/narwhl/mockestra/hydra"
	"github.com/narwhl/mockestra/postgres"
	"github.com/narwhl/mockestra/proxy"
	"github.com/openfga/go-sdk/oauth2/clientcredentials"
	hydraclient "github.com/ory/hydra-client-go"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestHydraModule_SmokeTest(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hydra"`
		}) {
			// Test public API endpoint
			publicEndpoint, err := params.Container.PortEndpoint(context.Background(), hydra.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Hydra public API endpoint: %v", err)
			}

			// Test admin API endpoint
			adminEndpoint, err := params.Container.PortEndpoint(context.Background(), hydra.AdminPort, "")
			if err != nil {
				t.Fatalf("Failed to get Hydra admin API endpoint: %v", err)
			}

			// Verify public API is accessible
			resp, err := http.Get(fmt.Sprintf("http://%s/health/ready", publicEndpoint))
			if err != nil {
				t.Fatalf("Failed to reach Hydra public API: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Hydra public API health check failed with status: %d", resp.StatusCode)
			}

			// Verify admin API is accessible
			resp2, err := http.Get(fmt.Sprintf("http://%s/health/ready", adminEndpoint))
			if err != nil {
				t.Fatalf("Failed to reach Hydra admin API: %v", err)
			}
			defer resp2.Body.Close()
			if resp2.StatusCode != http.StatusOK {
				t.Fatalf("Hydra admin API health check failed with status: %d", resp.StatusCode)
			}

			t.Logf("Hydra container is running successfully")
			t.Logf("Public API: %s", publicEndpoint)
			t.Logf("Admin API: %s", adminEndpoint)
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_WithPostgres(t *testing.T) {
	dsn := "postgres://user:pass@localhost:5432/hydra?sslmode=disable"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-postgres-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithPostgres(dsn),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"hydra"`
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

func TestHydraModule_WithURL(t *testing.T) {
	issuerURL := "https://auth.example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-url-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithURL(issuerURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"hydra"`
		}) {
			// Verify URL was set
			if params.Request.Env["URLS_SELF_ISSUER"] != issuerURL {
				t.Fatalf("Expected URLS_SELF_ISSUER to be %s, got %s", issuerURL, params.Request.Env["URLS_SELF_ISSUER"])
			}
			t.Logf("WithURL decorator correctly set issuer URL")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_WithSelfServiceUIURL(t *testing.T) {
	uiURL := "https://ui.example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-ui-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithSelfServiceUIURL(uiURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"hydra"`
		}) {
			// Verify UI URLs
			expectedEnvs := map[string]string{
				"URLS_CONSENT": fmt.Sprintf("%s/consent", uiURL),
				"URLS_LOGIN":   fmt.Sprintf("%s/login", uiURL),
				"URLS_LOGOUT":  fmt.Sprintf("%s/logout", uiURL),
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

func TestHydraModule_WithKratosPublicURL(t *testing.T) {
	kratosPublicURL := "https://kratos.example.com"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-kratospublic-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithKratosPublicURL(kratosPublicURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"hydra"`
		}) {
			// Verify Kratos public URL was set
			if params.Request.Env["URLS_IDENTITY_PROVIDER_PUBLICURL"] != kratosPublicURL {
				t.Fatalf("Expected URLS_IDENTITY_PROVIDER_PUBLICURL to be %s, got %s", kratosPublicURL, params.Request.Env["URLS_IDENTITY_PROVIDER_PUBLICURL"])
			}
			t.Logf("WithKratosPublicURL decorator correctly set Kratos public URL")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_WithKratosURL(t *testing.T) {
	kratosURL := "http://kratos:4434"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-kratos-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithKratosURL(kratosURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"hydra"`
		}) {
			// Verify Kratos URL was set
			if params.Request.Env["URLS_IDENTITY_PROVIDER_URL"] != kratosURL {
				t.Fatalf("Expected URLS_IDENTITY_PROVIDER_URL to be %s, got %s", kratosURL, params.Request.Env["URLS_IDENTITY_PROVIDER_URL"])
			}
			t.Logf("WithKratosURL decorator correctly set Kratos URL")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_WithPostReadyHook(t *testing.T) {
	hookCalled := false
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-hook-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithPostReadyHook(func(endpoints map[string]string) error {
				hookCalled = true
				t.Logf("Post-ready hook called for Hydra container with endpoints: %v", endpoints)
				return nil
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hydra"`
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

func TestHydraModule_ProxyConfiguration(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-proxy-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hydra"`
			PublicProxy *proxy.TCPProxy `name:"hydra"`
			AdminProxy *proxy.TCPProxy `name:"hydraadmin"`
		}) {
			// Test that proxies are created
			if params.PublicProxy == nil {
				t.Fatal("Public API proxy was not created")
			}
			if params.AdminProxy == nil {
				t.Fatal("Admin API proxy was not created")
			}
			
			// Test proxy connectivity via localhost
			// Public API proxy should be available on localhost:4444
			_, portStr := nat.SplitProtoPort(hydra.Port)
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health/ready", portStr))
			if err != nil {
				t.Fatalf("Failed to reach Hydra public API via proxy: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Hydra public API proxy health check failed with status: %d", resp.StatusCode)
			}
			
			// Admin API proxy should be available on localhost:4445
			_, adminPortStr := nat.SplitProtoPort(hydra.AdminPort)
			resp2, err := http.Get(fmt.Sprintf("http://localhost:%s/health/ready", adminPortStr))
			if err != nil {
				t.Fatalf("Failed to reach Hydra admin API via proxy: %v", err)
			}
			defer resp2.Body.Close()
			if resp2.StatusCode != http.StatusOK {
				t.Fatalf("Hydra admin API proxy health check failed with status: %d", resp.StatusCode)
			}
			
			t.Logf("Hydra proxy configuration is working correctly")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_MultipleDecorators(t *testing.T) {
	dsn := "postgres://hydra:hydrapass@postgres:5432/hydra?sslmode=disable"
	issuerURL := "https://auth.example.com"
	uiURL := "https://ui.example.com"
	kratosPublicURL := "https://kratos.example.com"
	kratosURL := "http://kratos:4434"
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-multi-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithPostgres(dsn),
			hydra.WithURL(issuerURL),
			hydra.WithSelfServiceUIURL(uiURL),
			hydra.WithKratosPublicURL(kratosPublicURL),
			hydra.WithKratosURL(kratosURL),
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"hydra"`
		}) {
			// Verify all settings were applied
			expectedEnvs := map[string]string{
				"DSN":                            dsn,
				"URLS_SELF_ISSUER":              issuerURL,
				"URLS_CONSENT":                  fmt.Sprintf("%s/consent", uiURL),
				"URLS_LOGIN":                    fmt.Sprintf("%s/login", uiURL),
				"URLS_LOGOUT":                   fmt.Sprintf("%s/logout", uiURL),
				"URLS_IDENTITY_PROVIDER_PUBLICURL": kratosPublicURL,
				"URLS_IDENTITY_PROVIDER_URL":    kratosURL,
			}
			
			for key, expected := range expectedEnvs {
				if params.Request.Env[key] != expected {
					t.Fatalf("Expected %s to be %s, got %s", key, expected, params.Request.Env[key])
				}
			}
			t.Logf("All decorators correctly applied their configurations")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_WithGenerateClientCredentialsHook(t *testing.T) {
	var capturedConfig *clientcredentials.Config
	hookCalled := false

	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-clientcreds-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithGenerateClientCredentialsHook(
				func(client *clientcredentials.Config) error {
					hookCalled = true
					capturedConfig = client
					t.Logf("Client credentials hook called with ClientID: %s", client.ClientID)

					// Obtain a token using client credentials
					ctx := context.Background()
					token, err := client.Token(ctx)
					if err != nil {
						return fmt.Errorf("failed to obtain token: %w", err)
					}
					t.Logf("Successfully obtained access token")

					if token.AccessToken == "" {
						return fmt.Errorf("access token is empty")
					}

					t.Logf("Access token is valid and non-empty")
					return nil
				},
				hydra.OAuthClientOptions{
					Name:             "test-client",
					RedirectURIs:     []string{"http://localhost:8080/callback"},
					AdditionalScopes: []string{"custom_scope"},
				},
			),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hydra"`
		}) {
			// Hook is synchronous and runs before container is ready
			if !hookCalled {
				t.Fatal("Client credentials hook was not called")
			}

			if capturedConfig == nil {
				t.Fatal("Client config was not captured")
			}

			// Verify client config has expected values
			if capturedConfig.ClientID == "" {
				t.Error("ClientID is empty")
			}
			if capturedConfig.ClientSecret == "" {
				t.Error("ClientSecret is empty")
			}
			if capturedConfig.TokenURL == "" {
				t.Error("TokenURL is empty")
			}

			// Verify scopes include both default and additional
			expectedScopes := []string{"custom_scope", "openid", "profile", "email", "offline_access"}
			if len(capturedConfig.Scopes) != len(expectedScopes) {
				t.Errorf("Expected %d scopes, got %d", len(expectedScopes), len(capturedConfig.Scopes))
			}

			// Now let's perform token introspection using the admin API
			adminEndpoint, err := params.Container.PortEndpoint(context.Background(), hydra.AdminPort, "")
			if err != nil {
				t.Fatalf("Failed to get admin endpoint: %v", err)
			}

			// Get a fresh token to introspect
			ctx := context.Background()
			token, err := capturedConfig.Token(ctx)
			if err != nil {
				t.Fatalf("Failed to obtain token for introspection: %v", err)
			}

			// Create Hydra admin client
			hydraClientConfiguration := hydraclient.NewConfiguration()
			hydraClientConfiguration.Servers = []hydraclient.ServerConfiguration{
				{
					URL: fmt.Sprintf("http://%s/admin", adminEndpoint),
				},
			}
			hydraApiClient := hydraclient.NewAPIClient(hydraClientConfiguration)

			// Introspect the token
			introspectResp, _, err := hydraApiClient.AdminApi.IntrospectOAuth2Token(ctx).
				Token(token.AccessToken).
				Execute()
			if err != nil {
				t.Fatalf("Failed to introspect token: %v", err)
			}

			// Verify token is active
			if !introspectResp.Active {
				t.Error("Token is not active")
			}

			// Verify client ID matches
			if introspectResp.ClientId == nil {
				t.Error("ClientId in introspection response is nil")
			} else if *introspectResp.ClientId != capturedConfig.ClientID {
				t.Errorf("Expected ClientID %s, got %s", capturedConfig.ClientID, *introspectResp.ClientId)
			}

			t.Logf("Token introspection successful - token is active and valid")
			if introspectResp.ClientId != nil {
				clientID := *introspectResp.ClientId
				scope := ""
				if introspectResp.Scope != nil {
					scope = *introspectResp.Scope
				}
				t.Logf("Token details: ClientID=%s, Scope=%s", clientID, scope)
			}
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestHydraModule_FullIntegration(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"hydra_version"`),
			),
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("hydra-integration-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		postgres.Module(
			postgres.WithUsername("dbuser"),
			postgres.WithPassword("dbpass"),
			postgres.WithDatabase("testdb"),
			postgres.WithExtraDatabase(hydra.DatabaseName, "hydrauser", "hydrapass"),
		),
		hydra.Module(
			hydra.WithURL("https://auth.example.com"),
			hydra.WithSelfServiceUIURL("https://ui.example.com"),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"hydra"`
			PublicProxy *proxy.TCPProxy `name:"hydra"`
			AdminProxy *proxy.TCPProxy `name:"hydraadmin"`
		}) {
			// Verify container is running
			state, err := params.Container.State(context.Background())
			if err != nil {
				t.Fatalf("Failed to get container state: %v", err)
			}
			if !state.Running {
				t.Fatal("Hydra container is not running")
			}
			
			// Verify endpoints are accessible
			publicEndpoint, err := params.Container.PortEndpoint(context.Background(), hydra.Port, "")
			if err != nil {
				t.Fatalf("Failed to get public endpoint: %v", err)
			}
			
			adminEndpoint, err := params.Container.PortEndpoint(context.Background(), hydra.AdminPort, "")
			if err != nil {
				t.Fatalf("Failed to get admin endpoint: %v", err)
			}
			
			// Test health endpoints
			endpoints := map[string]string{
				"Public API": fmt.Sprintf("http://%s/health/ready", publicEndpoint),
				"Admin API":  fmt.Sprintf("http://%s/health/ready", adminEndpoint),
			}
			
			for name, url := range endpoints {
				resp, err := http.Get(url)
				if err != nil {
					t.Errorf("%s health check failed: %v", name, err)
					continue
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					t.Errorf("%s health check returned status %d", name, resp.StatusCode)
				}
			}
			
			t.Logf("Hydra full integration test passed")
		}),
	)
	app.RequireStart()

	t.Cleanup(func() {
		app.RequireStop()
	})
}