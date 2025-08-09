package lgtm_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/lgtm"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestLGTMModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.StartTimeout(90*time.Second),
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"lgtm_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("lgtm-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"lgtm"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.OtlpHttpPort, "")
			if err != nil {
				t.Fatalf("Failed to get LGTM container endpoint: %v", err)
			}
			logExporter, err := otlploghttp.New(
				t.Context(),
				otlploghttp.WithInsecure(),
				otlploghttp.WithEndpoint(endpoint),
			)
			if err != nil {
				t.Fatalf("Failed to create OTLP log exporter: %v", err)
			}
			r := resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("lgtm-test"),
			)
			processor := log.NewBatchProcessor(logExporter)
			provider := log.NewLoggerProvider(
				log.WithProcessor(processor),
				log.WithResource(r),
			)
			logger := otelslog.NewLogger(
				"lgtm-test",
				otelslog.WithLoggerProvider(provider),
			)
			logger.Info("LGTM container is ready")
		}),
	)
	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
