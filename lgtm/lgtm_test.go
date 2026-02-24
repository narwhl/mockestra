package lgtm_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func TestWithDashboard(t *testing.T) {
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
			fmt.Sprintf("lgtm-dashboard-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithDashboard(
				container.Dashboard{
					Name: "test-dashboard",
					JSON: `{
						"uid": "test-dashboard",
						"title": "Test Dashboard",
						"panels": [],
						"schemaVersion": 30
					}`,
				},
				container.Dashboard{
					Name: "test-dashboard-2",
					JSON: `{
						"uid": "test-dashboard-2",
						"title": "Test Dashboard 2",
						"panels": [],
						"schemaVersion": 30
					}`,
				},
			),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"lgtm"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.GrafanaPort, "")
			if err != nil {
				t.Fatalf("Failed to get Grafana endpoint: %v", err)
			}

			grafanaURL := fmt.Sprintf("http://%s/api/search", endpoint)

			var resp *http.Response
			for range 10 {
				resp, err = http.Get(grafanaURL)
				if err == nil && resp.StatusCode == http.StatusOK {
					break
				}
				time.Sleep(2 * time.Second)
			}
			if err != nil {
				t.Fatalf("Failed to query Grafana API: %v", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read Grafana API response: %v", err)
			}

			var dashboards []map[string]any
			if err := json.Unmarshal(body, &dashboards); err != nil {
				t.Fatalf("Failed to parse Grafana API response: %v", err)
			}

			titles := make(map[string]bool)
			for _, d := range dashboards {
				if title, ok := d["title"].(string); ok {
					titles[title] = true
				}
			}

			for _, expected := range []string{"Test Dashboard", "Test Dashboard 2"} {
				if !titles[expected] {
					t.Fatalf("Dashboard %q not found in Grafana, got: %s", expected, string(body))
				}
			}
		}),
	)
	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
