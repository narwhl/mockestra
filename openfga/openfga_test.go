package openfga_test

import (
	"fmt"
	"testing"
	"time"

	container "github.com/narwhl/mockestra/openfga"
	. "github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

const fixture = `
model
  schema 1.1

type employee
  relations
    define can_manage: manager or can_manage from manager
    define manager: [employee]

type report
  relations
    define approver: can_manage from submitter
    define submitter: [employee]

`

func TestOpenFGAModule_SmokeTest(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"openfga_version"`),
			),
			fx.Annotate(
				fmt.Sprintf("openfga-test-%x", time.Now().Unix()),
				fx.ResultTags(`name:"prefix"`),
			),
		),
		container.Module(),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"openfga"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.HttpPort, "")
			if err != nil {
				t.Fatalf("Failed to get OpenFGA container endpoint: %v", err)
			}
			fgaClient, err := NewSdkClient(&ClientConfiguration{
				ApiUrl: fmt.Sprintf("http://%s", endpoint),
			})
			if err != nil {
				t.Fatalf("Failed to create OpenFGA client: %v", err)
			}
			_, err = fgaClient.ListStores(t.Context()).Options(ClientListStoresOptions{}).Execute()
			if err != nil {
				t.Fatalf("Failed to list OpenFGA stores: %v", err)
			}
		}),
	)

	app.RequireStart()

	t.Cleanup(app.RequireStop)
}

func TestOpenFGAModule_WithPresharedKey(t *testing.T) {
	token := "random-test-token"
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"openfga_version"`),
			),
			fx.Annotate(
				fmt.Sprintf("openfga-test-%x", time.Now().Unix()),
				fx.ResultTags(`name:"prefix"`),
			),
		),
		container.Module(
			container.WithPresharedKey(token),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"openfga"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.HttpPort, "")
			if err != nil {
				t.Fatalf("Failed to get OpenFGA container endpoint: %v", err)
			}
			fgaClient, err := NewSdkClient(&ClientConfiguration{
				ApiUrl: fmt.Sprintf("http://%s", endpoint),
				Credentials: &credentials.Credentials{
					Method: credentials.CredentialsMethodApiToken,
					Config: &credentials.Config{
						ApiToken: token,
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create OpenFGA client: %v", err)
			}
			_, err = fgaClient.ListStores(t.Context()).Options(ClientListStoresOptions{}).Execute()
			if err != nil {
				t.Fatalf("Failed to list OpenFGA stores: %v", err)
			}
		}),
	)
	app.RequireStart()
	t.Cleanup(app.RequireStop)
}

func TestOpenFGAModule_WithAuthorizationModel(t *testing.T) {
	token := "random-test-token"
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"latest",
				fx.ResultTags(`name:"openfga_version"`),
			),
			fx.Annotate(
				fmt.Sprintf("openfga-test-%x", time.Now().Unix()),
				fx.ResultTags(`name:"prefix"`),
			),
		),
		container.Module(
			container.WithPresharedKey(token),
			container.WithAuthorizationModel(fixture, func(storeID, authModelID string) error {
				if storeID == "" || authModelID == "" {
					return fmt.Errorf("store ID or authorization model ID is empty")
				}
				// Here you can add additional checks or operations with the store and auth model IDs
				return nil
			}),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"openfga"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.HttpPort, "")
			if err != nil {
				t.Fatalf("Failed to get OpenFGA container endpoint: %v", err)
			}
			fgaClient, err := NewSdkClient(&ClientConfiguration{
				ApiUrl: fmt.Sprintf("http://%s", endpoint),
				Credentials: &credentials.Credentials{
					Method: credentials.CredentialsMethodApiToken,
					Config: &credentials.Config{
						ApiToken: token,
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create OpenFGA client: %v", err)
			}
			stores, err := fgaClient.ListStores(t.Context()).Options(ClientListStoresOptions{}).Execute()
			if err != nil {
				t.Fatalf("Failed to list OpenFGA stores: %v", err)
			}
			if len(stores.Stores) == 0 {
				t.Fatalf("No stores found in OpenFGA")
			}
		}),
	)
	app.RequireStart()
	t.Cleanup(app.RequireStop)
}
