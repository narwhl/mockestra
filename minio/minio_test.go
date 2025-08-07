package minio_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	container "github.com/narwhl/mockestra/minio"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMinioModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"minio_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("minio-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"minio"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client, err := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
				Secure: false,
			})
			if err != nil {
				t.Errorf("failed to create minio client: %v", err)
			}
			_, err = client.ListBuckets(t.Context())
			if err != nil {
				t.Errorf("failed to list buckets: %v", err)
				return
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}

func TestMinioModule_WithBucket(t *testing.T) {
	expectedBucket := "test-bucket"
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"minio_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("minio-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithBucket(expectedBucket),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"minio"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client, err := minio.New(endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
				Secure: false,
			})
			if err != nil {
				t.Errorf("failed to create minio client: %v", err)
			}
			buckets, err := client.ListBuckets(t.Context())
			if err != nil {
				t.Errorf("failed to list buckets: %v", err)
				return
			}
			found := false
			for _, bucket := range buckets {
				if bucket.Name == expectedBucket {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected bucket '%s' to be created, got: %v", expectedBucket, buckets)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
