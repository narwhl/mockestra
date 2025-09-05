package redis_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	container "github.com/narwhl/mockestra/redis"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestRedisModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"8-alpine",
				fx.ResultTags(`name:"redis_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("redis-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"redis"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Errorf("failed to get endpoint: %v", err)
			}
			client := redis.NewClient(&redis.Options{
				Addr:     endpoint,
				Password: "", // no password set
				DB:       0,  // use default DB
			})
			_, err = client.Ping(t.Context()).Result()
			if err != nil {
				t.Errorf("failed to ping redis: %v", err)
			}
			defer client.Close()
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
