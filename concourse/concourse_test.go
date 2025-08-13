package concourse_test

import (
	"fmt"
	"testing"
	"time"

	// Note that this is not referencing v7 for the module path
	// As upstream has not specified v7 in go.mod metadata, go fails to import the
	// package by semver tag. This is imported by commit, and should be the v7
	// latest version of the API, but annotated as v1 in go.mod
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"

	container "github.com/narwhl/mockestra/concourse"
	"github.com/narwhl/mockestra/postgres"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"golang.org/x/oauth2"
)

// example pipeline from https://concourse-ci.org/tutorial-hello-world.html
const fixture = `
jobs:
  - name: hello-world-job
    plan:
      - task: hello-world-task
        config:
          # Tells Concourse which type of worker this task should run on
          platform: linux
          # This is one way of telling Concourse which container image to use for a
          # task. We'll explain this more when talking about resources
          image_resource:
            type: registry-image
            source:
              repository: busybox # images are pulled from docker hub by default
          # The command Concourse will run inside the container
          # echo "Hello world!"
          run:
            path: echo
            args: ["Hello world!"]
`

func TestConcourseModule(t *testing.T) {
	app := fxtest.New(
		t,
		fx.NopLogger,
		fx.Supply(
			fx.Annotate(
				"17",
				fx.ResultTags(`name:"postgres_version"`),
			),
			fx.Annotate(
				"7.14.0",
				fx.ResultTags(`name:"concourse_version"`),
			),
			fx.Annotate(
				fmt.Sprintf("concourse-test-%x", time.Now().Unix()),
				fx.ResultTags(`name:"prefix"`),
			),
		),
		postgres.Module(
			postgres.WithUsername("pgtestuser"),
			postgres.WithPassword("pgtestpass"),
			postgres.WithDatabase(container.DatabaseName),
		),
		container.Module(
			container.WithUserAndTeam("testuser", "testpass", atc.DefaultTeamName),
			container.WithSecret("Y29uY291cnNlLXdvcmtlcgo="),
		),
		fx.Invoke(func(params struct {
			fx.In
			Container testcontainers.Container `name:"concourse"`
		}) {
			endpoint, err := params.Container.PortEndpoint(t.Context(), container.Port, "http")
			if err != nil {
				t.Fatalf("Failed to get %s container endpoint: %v", container.ContainerPrettyName, err)
			}

			// Oauth client configuration for local username password login in Fly CLI
			oauth2Config := oauth2.Config{
				ClientID:     "fly",
				ClientSecret: "Zmx5",
				Endpoint:     oauth2.Endpoint{TokenURL: endpoint + "/sky/issuer/token"},
				Scopes:       []string{"openid", "profile", "email", "federated:id", "groups"},
			}

			// Obtain a token with password credentials flow, claiming to be the Fly CLI
			// Note that the token is valid only for 86400 seconds
			token, err := oauth2Config.PasswordCredentialsToken(t.Context(), "testuser", "testpass")
			if err != nil {
				t.Fatalf("Failed to get %s login token: %v", container.ContainerPrettyName, err)
			}

			// Concourse uses standard OAuth2 authorization, which checks for a bearer
			// token in the Authorization header. So, we only need to create a http
			// client from the OAuth2 config and token with the standard oauth2 lib
			// and pass it to the concourse client
			httpClient := oauth2Config.Client(t.Context(), token)
			client := concourse.NewClient(endpoint, httpClient, false)

			t.Run("smoke test", func(t *testing.T) {
				// Assert concourse works by getting info with the authenticated client
				info, err := client.GetInfo()
				if err != nil {
					t.Fatalf("Failed to get %s info: %v", container.ContainerPrettyName, err)
				}
				if info.Version != "7.14.0" {
					t.Errorf("Expected %s version %s, got %s", container.ContainerPrettyName, "7.14.0", info.Version)
				}
			})
		}),
	)

	app.RequireStart()

	t.Cleanup(app.RequireStop)
}
