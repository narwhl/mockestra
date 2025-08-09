package postgres_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	container "github.com/narwhl/mockestra/postgres"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// mockContainer implements testcontainers.Container for Endpoint mocking
type mockContainer struct {
	testcontainers.Container
	endpoint string
}

func (m *mockContainer) Endpoint(ctx context.Context, _ string) (string, error) {
	return m.endpoint, nil
}

func TestWithMigration(t *testing.T) {
	// Plan:
	// 1. Create a dummy migration function that records the dsn it receives and returns nil.
	// 2. Create a GenericContainerRequest with required Env fields.
	// 3. Call WithMigration with the dummy migration, apply it to the request.
	// 4. Simulate the PostReadies hook and verify the migration function is called with the correct DSN.

	called := false
	var receivedDSN string
	migrationFn := func(dsn string) error {
		called = true
		receivedDSN = dsn
		return nil
	}

	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_DB":       "testdb",
			},
		},
	}

	opt := container.WithMigration(migrationFn)
	err := opt.Customize(req)
	if err != nil {
		t.Fatalf("Customize failed: %v", err)
	}

	if len(req.LifecycleHooks) == 0 || len(req.LifecycleHooks[0].PostReadies) == 0 {
		t.Fatalf("LifecycleHooks or PostReadies not set")
	}

	// Mock container.Endpoint to return a fake address
	mockContainer := &mockContainer{
		endpoint: "localhost:5432",
	}

	ctx := context.Background()
	hook := req.LifecycleHooks[0].PostReadies[0]
	err = hook(ctx, mockContainer)
	if err != nil {
		t.Fatalf("PostReady hook failed: %v", err)
	}

	if !called {
		t.Error("Migration function was not called")
	}

	expectedDSN := "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
	if receivedDSN != expectedDSN {
		t.Errorf("Expected DSN %q, got %q", expectedDSN, receivedDSN)
	}
}
func TestWithExtraDatabase(t *testing.T) {
	// Plan:
	// 1. Call WithExtraDatabase with test values.
	// 2. Ensure the returned CustomizeRequestOption is not nil.
	// 3. Apply the option to a GenericContainerRequest.
	// 4. Check that the InitScripts field is set and points to a file that contains the expected SQL.

	opt := container.WithExtraDatabase("extradb", "extrauser", "extrapass")
	if opt == nil {
		t.Fatal("WithExtraDatabase returned nil")
	}

	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{},
	}

	err := opt.Customize(req)
	if err != nil {
		t.Fatalf("Customize failed: %v", err)
	}

	initScripts := req.Files
	if len(initScripts) == 0 {
		t.Fatal("InitScripts not set by WithExtraDatabase")
	}

	// Read the file and check contents
	content, err := os.ReadFile(initScripts[0].HostFilePath)
	if err != nil {
		t.Fatalf("Failed to read init script: %v", err)
	}

	sql := string(content)
	if !(strings.Contains(sql, "CREATE USER extrauser WITH PASSWORD 'extrapass';") &&
		strings.Contains(sql, "CREATE DATABASE extradb WITH OWNER extrauser;") &&
		strings.Contains(sql, "GRANT ALL PRIVILEGES ON DATABASE extradb TO extrauser;")) {
		t.Errorf("Init script does not contain expected SQL, got:\n%s", sql)
	}
}

func TestPostgresModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("postgres-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithUsername("testuser"),
			container.WithPassword("testpass"),
			container.WithDatabase("testdb"),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"postgres"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			conn, err := pgx.Connect(t.Context(), fmt.Sprintf(
				"postgres://%s:%s@%s/%s?sslmode=disable",
				"testuser", "testpass", endpoint, "testdb",
			))
			if err != nil {
				t.Errorf("failed to connect to postgres: %v", err)
			}
			defer conn.Close(t.Context())
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
