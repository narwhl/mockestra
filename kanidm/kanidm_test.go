package kanidm_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/kanidm"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestKanidmModule_SmokeTest(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kanidm_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kanidm-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"kanidm"`
		}) {
			endpoint, err := params.Container.PortEndpoint(context.Background(), container.Port, "https")
			if err != nil {
				t.Fatalf("Failed to get Kanidm endpoint: %v", err)
			}

			// Kanidm uses HTTPS with self-signed certs, so we need to skip TLS verification
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: tr}

			// Test status endpoint
			resp, err := client.Get(fmt.Sprintf("%s/status", endpoint))
			if err != nil {
				t.Fatalf("Failed to reach Kanidm status endpoint: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Kanidm status check failed with status: %d", resp.StatusCode)
			}

			t.Logf("Kanidm container is running successfully at %s", endpoint)
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestKanidmModule_WithDomain(t *testing.T) {
	customDomain := "auth.mycompany.com"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithDomain(customDomain)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_DOMAIN"] != customDomain {
		t.Fatalf("Expected KANIDM_DOMAIN to be %s, got %s", customDomain, req.Env["KANIDM_DOMAIN"])
	}
	t.Logf("WithDomain decorator correctly set domain to %s", customDomain)
}

func TestKanidmModule_WithOrigin(t *testing.T) {
	customOrigin := "https://auth.mycompany.com:8443"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithOrigin(customOrigin)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_ORIGIN"] != customOrigin {
		t.Fatalf("Expected KANIDM_ORIGIN to be %s, got %s", customOrigin, req.Env["KANIDM_ORIGIN"])
	}
	t.Logf("WithOrigin decorator correctly set origin to %s", customOrigin)
}

func TestKanidmModule_WithBindAddress(t *testing.T) {
	bindAddr := "0.0.0.0:443"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithBindAddress(bindAddr)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_BINDADDRESS"] != bindAddr {
		t.Fatalf("Expected KANIDM_BINDADDRESS to be %s, got %s", bindAddr, req.Env["KANIDM_BINDADDRESS"])
	}
	t.Logf("WithBindAddress decorator correctly set bind address to %s", bindAddr)
}

func TestKanidmModule_WithLDAPBindAddress(t *testing.T) {
	ldapBindAddr := "0.0.0.0:3636"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env:          make(map[string]string),
			ExposedPorts: []string{container.Port},
		},
	}

	opt := container.WithLDAPBindAddress(ldapBindAddr)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_LDAPBINDADDRESS"] != ldapBindAddr {
		t.Fatalf("Expected KANIDM_LDAPBINDADDRESS to be %s, got %s", ldapBindAddr, req.Env["KANIDM_LDAPBINDADDRESS"])
	}

	// Verify LDAP port is exposed
	ldapPortExposed := false
	for _, port := range req.ExposedPorts {
		if port == container.LDAPPort {
			ldapPortExposed = true
			break
		}
	}
	if !ldapPortExposed {
		t.Fatal("LDAP port should be exposed when WithLDAPBindAddress is set")
	}
	t.Logf("WithLDAPBindAddress decorator correctly set LDAP bind address and exposed LDAP port")
}

func TestKanidmModule_WithLogLevel(t *testing.T) {
	logLevel := "debug"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithLogLevel(logLevel)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_LOG_LEVEL"] != logLevel {
		t.Fatalf("Expected KANIDM_LOG_LEVEL to be %s, got %s", logLevel, req.Env["KANIDM_LOG_LEVEL"])
	}
	t.Logf("WithLogLevel decorator correctly set log level to %s", logLevel)
}

func TestKanidmModule_WithDBPath(t *testing.T) {
	dbPath := "/custom/path/kanidm.db"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithDBPath(dbPath)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_DB_PATH"] != dbPath {
		t.Fatalf("Expected KANIDM_DB_PATH to be %s, got %s", dbPath, req.Env["KANIDM_DB_PATH"])
	}
	t.Logf("WithDBPath decorator correctly set database path to %s", dbPath)
}

func TestKanidmModule_WithDBFSType(t *testing.T) {
	fsType := "zfs"

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithDBFSType(fsType)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	if req.Env["KANIDM_DB_FS_TYPE"] != fsType {
		t.Fatalf("Expected KANIDM_DB_FS_TYPE to be %s, got %s", fsType, req.Env["KANIDM_DB_FS_TYPE"])
	}
	t.Logf("WithDBFSType decorator correctly set filesystem type to %s", fsType)
}

func TestKanidmModule_WithDBArcSize(t *testing.T) {
	arcSize := 4096

	// Test the option directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opt := container.WithDBArcSize(arcSize)
	if err := opt.Customize(req); err != nil {
		t.Fatalf("Failed to customize request: %v", err)
	}

	expected := fmt.Sprintf("%d", arcSize)
	if req.Env["KANIDM_DB_ARC_SIZE"] != expected {
		t.Fatalf("Expected KANIDM_DB_ARC_SIZE to be %s, got %s", expected, req.Env["KANIDM_DB_ARC_SIZE"])
	}
	t.Logf("WithDBArcSize decorator correctly set arc size to %d", arcSize)
}

func TestKanidmModule_WithPostReadyHook(t *testing.T) {
	hookCalled := false

	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kanidm_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kanidm-hook-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithPostReadyHook(func(endpoints map[string]string) error {
				hookCalled = true
				t.Logf("Post-ready hook called for Kanidm container with endpoints: %v", endpoints)
				return nil
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"kanidm"`
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
	t.Cleanup(app.RequireStop)
}

func TestKanidmModule_MultipleDecorators(t *testing.T) {
	domain := "auth.example.com"
	origin := "https://auth.example.com:8443"
	logLevel := "trace"

	// Test multiple options directly without starting the container
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	opts := []testcontainers.CustomizeRequestOption{
		container.WithDomain(domain),
		container.WithOrigin(origin),
		container.WithLogLevel(logLevel),
	}

	for _, opt := range opts {
		if err := opt.Customize(req); err != nil {
			t.Fatalf("Failed to customize request: %v", err)
		}
	}

	expectedEnvs := map[string]string{
		"KANIDM_DOMAIN":    domain,
		"KANIDM_ORIGIN":    origin,
		"KANIDM_LOG_LEVEL": logLevel,
	}

	for key, expected := range expectedEnvs {
		if req.Env[key] != expected {
			t.Fatalf("Expected %s to be %s, got %s", key, expected, req.Env[key])
		}
	}
	t.Logf("All decorators correctly applied their configurations")
}

func TestKanidmModule_FullIntegration(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"kanidm_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("kanidm-integration-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithDomain("idm.test.local"),
			container.WithOrigin("https://idm.test.local:8443"),
			container.WithLogLevel("info"),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"kanidm"`
		}) {
			// Verify container is running
			state, err := params.Container.State(context.Background())
			if err != nil {
				t.Fatalf("Failed to get container state: %v", err)
			}
			if !state.Running {
				t.Fatal("Kanidm container is not running")
			}

			// Verify endpoint is accessible
			endpoint, err := params.Container.PortEndpoint(context.Background(), container.Port, "https")
			if err != nil {
				t.Fatalf("Failed to get endpoint: %v", err)
			}

			// Test status endpoint with HTTPS
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: tr}

			resp, err := client.Get(fmt.Sprintf("%s/status", endpoint))
			if err != nil {
				t.Fatalf("Failed to reach Kanidm status endpoint: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Kanidm status check returned status %d", resp.StatusCode)
			}

			t.Logf("Kanidm full integration test passed")
			t.Logf("Endpoint: %s", endpoint)
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestKanidmModule_ContainerDefaults(t *testing.T) {
	// Test defaults by directly calling New() function
	// This avoids any state leakage from fxtest
	params := container.RequestParams{
		Prefix:  "test-defaults",
		Version: "latest",
		Opts:    nil,
	}

	req, err := container.New(params)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Verify default values are set
	if req.Env["KANIDM_DOMAIN"] != container.DefaultDomain {
		t.Fatalf("Expected default KANIDM_DOMAIN to be %s, got %s", container.DefaultDomain, req.Env["KANIDM_DOMAIN"])
	}
	if req.Env["KANIDM_ORIGIN"] != container.DefaultOrigin {
		t.Fatalf("Expected default KANIDM_ORIGIN to be %s, got %s", container.DefaultOrigin, req.Env["KANIDM_ORIGIN"])
	}
	if req.Env["KANIDM_TLS_CHAIN"] == "" {
		t.Fatal("Expected KANIDM_TLS_CHAIN to be set")
	}
	if req.Env["KANIDM_TLS_KEY"] == "" {
		t.Fatal("Expected KANIDM_TLS_KEY to be set")
	}
	if req.Env["KANIDM_BINDADDRESS"] == "" {
		t.Fatal("Expected KANIDM_BINDADDRESS to be set")
	}
	if req.Env["KANIDM_DB_PATH"] == "" {
		t.Fatal("Expected KANIDM_DB_PATH to be set")
	}

	// Verify TLS files are added
	if len(req.Files) < 2 {
		t.Fatalf("Expected at least 2 files (cert and key), got %d", len(req.Files))
	}

	t.Logf("Container defaults are correctly configured")
}
