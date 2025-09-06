package nats_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/nats"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestNATSModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"nats_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("nats-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"nats"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get NATS container endpoint: %v", err)
			}
			nc, err := nats.Connect(endpoint)
			if err != nil {
				t.Fatalf("Failed to connect to NATS server: %v", err)
			}
			defer nc.Close()
			err = nc.Publish("foo", []byte("Hello World"))
			if err != nil {
				t.Fatalf("Failed to publish message to NATS server: %v", err)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestJetStreamWithStream(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"nats_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("nats-jetstream-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithStream(container.StreamConfig{
				Name:        "ORDERS",
				Subjects:    []string{"ORDERS.*"},
				Description: "Orders stream for testing",
				Retention:   jetstream.LimitsPolicy,
				MaxMsgs:     1000,
				MaxAge:      24 * time.Hour,
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"nats"`
		}) {
			ctx := context.Background()
			endpoint, err := params.Container.PortEndpoint(ctx, container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get NATS container endpoint: %v", err)
			}

			nc, err := nats.Connect(endpoint)
			if err != nil {
				t.Fatalf("Failed to connect to NATS server: %v", err)
			}
			defer nc.Close()

			// Create JetStream context
			js, err := jetstream.New(nc)
			if err != nil {
				t.Fatalf("Failed to create JetStream context: %v", err)
			}

			// Verify stream exists
			stream, err := js.Stream(ctx, "ORDERS")
			if err != nil {
				t.Fatalf("Failed to get stream: %v", err)
			}

			// Get stream info
			info, err := stream.Info(ctx)
			if err != nil {
				t.Fatalf("Failed to get stream info: %v", err)
			}

			// Verify stream configuration
			if info.Config.Name != "ORDERS" {
				t.Errorf("Expected stream name ORDERS, got %s", info.Config.Name)
			}
			if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != "ORDERS.*" {
				t.Errorf("Expected subjects [ORDERS.*], got %v", info.Config.Subjects)
			}
			if info.Config.Description != "Orders stream for testing" {
				t.Errorf("Expected description 'Orders stream for testing', got %s", info.Config.Description)
			}

			// Publish a message to the stream
			ack, err := js.Publish(ctx, "ORDERS.new", []byte("Order #123"))
			if err != nil {
				t.Fatalf("Failed to publish message: %v", err)
			}
			if ack.Stream != "ORDERS" {
				t.Errorf("Expected message to be stored in ORDERS stream, got %s", ack.Stream)
			}

			// Create a consumer and fetch the message
			consumer, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
				Durable:   "test-consumer",
				AckPolicy: jetstream.AckExplicitPolicy,
			})
			if err != nil {
				t.Fatalf("Failed to create consumer: %v", err)
			}

			msgs, err := consumer.Fetch(1)
			if err != nil {
				t.Fatalf("Failed to fetch messages: %v", err)
			}

			msgCount := 0
			for msg := range msgs.Messages() {
				if string(msg.Data()) != "Order #123" {
					t.Errorf("Expected message 'Order #123', got %s", string(msg.Data()))
				}
				msg.Ack()
				msgCount++
			}

			if msgCount != 1 {
				t.Errorf("Expected 1 message, got %d", msgCount)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestJetStreamCallback(t *testing.T) {
	streamCreated := false
	consumerCreated := false

	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"nats_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("nats-callback-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithJetStreamCallback(func(ctx context.Context, js jetstream.JetStream) error {
				// Create a stream
				stream, err := js.CreateStream(ctx, jetstream.StreamConfig{
					Name:     "EVENTS",
					Subjects: []string{"events.*"},
				})
				if err != nil {
					return err
				}
				streamCreated = true

				// Create a consumer
				_, err = stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
					Durable:   "event-processor",
					AckPolicy: jetstream.AckAllPolicy,
				})
				if err != nil {
					return err
				}
				consumerCreated = true

				return nil
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"nats"`
		}) {
			ctx := context.Background()
			endpoint, err := params.Container.PortEndpoint(ctx, container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get NATS container endpoint: %v", err)
			}

			nc, err := nats.Connect(endpoint)
			if err != nil {
				t.Fatalf("Failed to connect to NATS server: %v", err)
			}
			defer nc.Close()

			// Create JetStream context
			js, err := jetstream.New(nc)
			if err != nil {
				t.Fatalf("Failed to create JetStream context: %v", err)
			}

			// Verify stream was created
			stream, err := js.Stream(ctx, "EVENTS")
			if err != nil {
				t.Fatalf("Failed to get stream: %v", err)
			}

			// Verify consumer was created
			consumer, err := stream.Consumer(ctx, "event-processor")
			if err != nil {
				t.Fatalf("Failed to get consumer: %v", err)
			}

			// Verify consumer configuration
			info, err := consumer.Info(ctx)
			if err != nil {
				t.Fatalf("Failed to get consumer info: %v", err)
			}
			if info.Config.AckPolicy != jetstream.AckAllPolicy {
				t.Errorf("Expected AckAllPolicy, got %v", info.Config.AckPolicy)
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	if !streamCreated {
		t.Error("Stream was not created in callback")
	}
	if !consumerCreated {
		t.Error("Consumer was not created in callback")
	}
}

func TestJetStreamKeyValue(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"nats_version"`),
			),
		),
		fx.Supply(fx.Annotate(
			fmt.Sprintf("nats-kv-test-%x", time.Now().Unix()),
			fx.ResultTags(`name:"prefix"`),
		)),
		container.Module(
			container.WithJetStreamCallback(func(ctx context.Context, js jetstream.JetStream) error {
				// Create a KV bucket
				_, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
					Bucket: "CONFIG",
				})
				return err
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"nats"`
		}) {
			ctx := context.Background()
			endpoint, err := params.Container.PortEndpoint(ctx, container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get NATS container endpoint: %v", err)
			}

			nc, err := nats.Connect(endpoint)
			if err != nil {
				t.Fatalf("Failed to connect to NATS server: %v", err)
			}
			defer nc.Close()

			// Create JetStream context
			js, err := jetstream.New(nc)
			if err != nil {
				t.Fatalf("Failed to create JetStream context: %v", err)
			}

			// Get KV bucket
			kv, err := js.KeyValue(ctx, "CONFIG")
			if err != nil {
				t.Fatalf("Failed to get KV bucket: %v", err)
			}

			// Put a value
			revision, err := kv.Put(ctx, "app.name", []byte("test-app"))
			if err != nil {
				t.Fatalf("Failed to put KV: %v", err)
			}
			if revision == 0 {
				t.Error("Expected non-zero revision")
			}

			// Get the value
			entry, err := kv.Get(ctx, "app.name")
			if err != nil {
				t.Fatalf("Failed to get KV: %v", err)
			}
			if string(entry.Value()) != "test-app" {
				t.Errorf("Expected value 'test-app', got %s", string(entry.Value()))
			}
		}),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
