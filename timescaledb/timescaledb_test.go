package timescaledb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/narwhl/mockestra/timescaledb"
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

	opt := timescaledb.WithMigration(migrationFn)
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

func TestTimescaleDBModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.Supply(
			fx.Annotate(
				"latest-pg17",
				fx.ResultTags(`name:"timescaledb_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("timescaledb-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		timescaledb.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"timescaledb"`
		}) {
			t.Log("To be implemented: TimescaleDB module test")
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
