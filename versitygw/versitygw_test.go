package versitygw_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/versitygw"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestWithAccessKey(t *testing.T) {
	// Test that WithAccessKey sets the ROOT_ACCESS_KEY environment variable
	opt := container.WithAccessKey("testkey123")

	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	err := opt.Customize(req)
	if err != nil {
		t.Fatalf("Customize failed: %v", err)
	}

	if req.Env["ROOT_ACCESS_KEY"] != "testkey123" {
		t.Errorf("Expected ROOT_ACCESS_KEY to be 'testkey123', got '%s'", req.Env["ROOT_ACCESS_KEY"])
	}
}

func TestWithSecretKey(t *testing.T) {
	// Test that WithSecretKey sets the ROOT_SECRET_KEY environment variable
	opt := container.WithSecretKey("supersecret")

	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}

	err := opt.Customize(req)
	if err != nil {
		t.Fatalf("Customize failed: %v", err)
	}

	if req.Env["ROOT_SECRET_KEY"] != "supersecret" {
		t.Errorf("Expected ROOT_SECRET_KEY to be 'supersecret', got '%s'", req.Env["ROOT_SECRET_KEY"])
	}
}

func TestWithBackend(t *testing.T) {
	// Test that WithBackend sets the Cmd correctly
	opt := container.WithBackend("s3", "http://minio:9000")

	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{},
	}

	err := opt.Customize(req)
	if err != nil {
		t.Fatalf("Customize failed: %v", err)
	}

	if len(req.Cmd) != 2 {
		t.Fatalf("Expected Cmd to have 2 elements, got %d", len(req.Cmd))
	}

	if req.Cmd[0] != "s3" {
		t.Errorf("Expected Cmd[0] to be 's3', got '%s'", req.Cmd[0])
	}

	if req.Cmd[1] != "http://minio:9000" {
		t.Errorf("Expected Cmd[1] to be 'http://minio:9000', got '%s'", req.Cmd[1])
	}
}

func TestWithPOSIXBackend(t *testing.T) {
	// Test that WithPOSIXBackend is a convenience wrapper
	opt := container.WithPOSIXBackend("/custom/path")

	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{},
	}

	err := opt.Customize(req)
	if err != nil {
		t.Fatalf("Customize failed: %v", err)
	}

	if len(req.Cmd) != 2 {
		t.Fatalf("Expected Cmd to have 2 elements, got %d", len(req.Cmd))
	}

	if req.Cmd[0] != "posix" {
		t.Errorf("Expected Cmd[0] to be 'posix', got '%s'", req.Cmd[0])
	}

	if req.Cmd[1] != "/custom/path" {
		t.Errorf("Expected Cmd[1] to be '/custom/path', got '%s'", req.Cmd[1])
	}
}

func TestVersityGWModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"versitygw_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("versitygw-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithAccessKey("testuser"),
			container.WithSecretKey("testsecret"),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"versitygw"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}

			// Verify the S3 gateway is accessible by checking the endpoint responds
			// VersityGW returns XML response for root path
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://%s/", endpoint))
			if err != nil {
				t.Errorf("failed to connect to VersityGW endpoint: %v", err)
			}
			defer resp.Body.Close()

			// S3 API typically returns 403 or 400 for unauthenticated requests to root
			// Any response indicates the gateway is running
			if resp.StatusCode != http.StatusForbidden &&
				resp.StatusCode != http.StatusBadRequest &&
				resp.StatusCode != http.StatusOK {
				t.Logf("Warning: Unexpected status code %d from VersityGW (expected 200, 400, or 403)", resp.StatusCode)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
