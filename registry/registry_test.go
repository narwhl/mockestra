package registry_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/registry"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"golang.org/x/crypto/bcrypt"
)

func TestRegistryModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"2",
				fx.ResultTags(`name:"registry_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("registry-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"registry"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "http")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			resp, err := http.Get(fmt.Sprintf("%s/v2/", endpoint))
			if err != nil {
				t.Errorf("failed to reach registry: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("unexpected status code: %d", resp.StatusCode)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestRegistryWithBasicAuth(t *testing.T) {
	username := "testuser"
	password := "testpass"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	htpasswdPath := filepath.Join(t.TempDir(), "htpasswd")
	if err := os.WriteFile(htpasswdPath, []byte(fmt.Sprintf("%s:%s\n", username, hash)), 0o644); err != nil {
		t.Fatalf("failed to write htpasswd file: %v", err)
	}

	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"2",
				fx.ResultTags(`name:"registry_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("registry-auth-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithBasicAuth(htpasswdPath),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"registry"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "http")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}

			// Unauthenticated request should return 401
			resp, err := http.Get(fmt.Sprintf("%s/v2/", endpoint))
			if err != nil {
				t.Errorf("failed to reach registry: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401 for unauthenticated request, got: %d", resp.StatusCode)
			}

			// Authenticated request should return 200
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/", endpoint), nil)
			if err != nil {
				t.Errorf("failed to create request: %v", err)
			}
			req.SetBasicAuth(username, password)
			authResp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("failed to reach registry with auth: %v", err)
			}
			defer authResp.Body.Close()
			if authResp.StatusCode != http.StatusOK {
				t.Errorf("expected 200 for authenticated request, got: %d", authResp.StatusCode)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
