package concourse_test

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	// Note that this is not referencing v7 for the module path
	// As upstream has not specified v7 in go.mod metadata, go fails to import the
	// package by semver tag. This is imported by commit, and should be the v7
	// latest version of the API, but annotated as v1 in go.mod
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/concourse/concourse/vars"
	"github.com/docker/go-connections/nat"

	"github.com/narwhl/mockestra"
	container "github.com/narwhl/mockestra/concourse"
	"github.com/narwhl/mockestra/postgres"
	"github.com/narwhl/mockestra/proxy"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"
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
			container.WithMainTeamUser("testuser", "testpass"),
			container.WithSecret("Y29uY291cnNlLXdvcmtlcgo="), // default credentials in quickstart, TODO: figure out how to generate this
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

			t.Run("user authentication", func(t *testing.T) {
				// Assert user is in the main team
				_, err := client.FindTeam(atc.DefaultTeamName)
				if err != nil {
					t.Fatalf("Failed to find %s team: %v", atc.DefaultTeamName, err)
				}
			})

			t.Run("create pipeline", func(t *testing.T) {
				// this corresponds to set-pipeline command in fly CLI
				// https://github.com/concourse/concourse/blob/ff09ee64fccee8f174e061ddfe33a3d46c5f5ee5/fly/commands/set_pipeline.go#L55
				// https://github.com/concourse/concourse/blob/ff09ee64fccee8f174e061ddfe33a3d46c5f5ee5/fly/commands/internal/setpipelinehelpers/atc_config.go#L49
				// https://github.com/concourse/concourse/blob/ff09ee64fccee8f174e061ddfe33a3d46c5f5ee5/fly/commands/internal/templatehelpers/yaml_template.go#L37

				pipelineRef := atc.PipelineRef{Name: "hello-world"}

				// allow empty is set to false, matching the default behavior in fly CLI
				// https://github.com/concourse/concourse/blob/master/fly/commands/internal/setpipelinehelpers/atc_config.go#L49
				// TODO: figure out variable files parsing
				evaluatedTemplate, err := vars.NewTemplateResolver([]byte(fixture), []vars.Variables{}).Resolve(false, false)
				if err != nil {
					t.Fatalf("Failed to evaluate pipeline config: %v", err)
				}

				var newConfig atc.Config
				err = yaml.Unmarshal([]byte(evaluatedTemplate), &newConfig)
				if err != nil {
					t.Fatalf("Failed to unmarshal pipeline config: %v", err)
				}

				// this should work if previous test was successful
				team, err := client.FindTeam(atc.DefaultTeamName)
				if err != nil {
					t.Fatalf("Failed to find %s team: %v", atc.DefaultTeamName, err)
				}

				_, existingConfigVersion, _, err := team.PipelineConfig(pipelineRef)
				if err != nil {
					t.Fatalf("Failed to get existing config version: %v", err)
				}

				created, updated, warnings, err := team.CreateOrUpdatePipelineConfig(
					atc.PipelineRef{Name: "hello-world"},
					existingConfigVersion,
					evaluatedTemplate,
					false, // --check-creds flag in fly CLI
				)
				if err != nil {
					t.Fatalf("Failed to update pipeline config: %v", err)
				}

				for _, warning := range warnings {
					t.Logf("WARNING: updating pipeline config: %s", warning.Message)
				}

				if !created {
					t.Errorf("Expected pipeline config to be created")
				}
				if updated {
					t.Errorf("Expected pipeline config to not be updated but created")
				}

				// unpause pipeline for job execution
				// this corresponds to unpause-pipeline command in fly CLI
				// https://github.com/concourse/concourse/blob/master/fly/commands/unpause_pipeline.go
				_, err = team.UnpausePipeline(pipelineRef)
				if err != nil {
					t.Fatalf("Failed to unpause pipeline: %v", err)
				}
			})

			t.Run("run pipeline job", func(t *testing.T) {
				// this test creates a job and actually executes it
				// this ensures that the container is configured correctly to allow
				// container jobs to run
				pipelineRef := atc.PipelineRef{Name: "hello-world"}
				jobName := "hello-world-job"

				team, err := client.FindTeam(atc.DefaultTeamName)
				if err != nil {
					t.Fatalf("Failed to find %s team: %v", atc.DefaultTeamName, err)
				}

				build, err := team.CreateJobBuild(pipelineRef, jobName)
				if err != nil {
					t.Fatalf("Failed to create job build: %v", err)
				}

				// watches the event stream for job succeeded event
				// uses the underlying event stream from watch command
				// https://github.com/concourse/concourse/blob/ff09ee64fccee8f174e061ddfe33a3d46c5f5ee5/fly/commands/watch.go#L77
				eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))
				if err != nil {
					t.Fatalf("Failed to get build events: %v", err)
				}
				defer eventSource.Close()

				// event loop implementation modified from event stream rendering in fly CLI
				// https://github.com/concourse/concourse/blob/ff09ee64fccee8f174e061ddfe33a3d46c5f5ee5/fly/eventstream/render.go#L19
			eventLoop:
				for {
					ev, err := eventSource.NextEvent()
					if err != nil {
						if err == io.EOF {
							t.Errorf("reached end of event stream without finding job succeeded event")
							break
						} else {
							t.Errorf("Failed to parse event: %v", err)
						}
					}
					switch e := ev.(type) {
					case event.Log:
						// log job output
						t.Log(e.Payload)
					case event.Status:
						switch e.Status {
						case "succeeded":
							// successfully completed job, break out of event loop
							break eventLoop
						}
					}
				}
			})

			t.Run("across step enabled", func(t *testing.T) {
				// Test that the across step feature is enabled
				// This creates a pipeline with an across step that runs multiple tasks
				const acrossPipeline = `
jobs:
  - name: across-test-job
    plan:
      - across:
        - var: message
          values: ["First", "Second"]
        task: echo-task
        config:
          platform: linux
          image_resource:
            type: registry-image
            source:
              repository: busybox
          run:
            path: echo
            args: ["((.:message))"]
`

				pipelineRef := atc.PipelineRef{Name: "across-test"}

				// Parse and create the pipeline with across step
				evaluatedTemplate, err := vars.NewTemplateResolver([]byte(acrossPipeline), []vars.Variables{}).Resolve(false, false)
				if err != nil {
					t.Fatalf("Failed to evaluate pipeline config: %v", err)
				}

				var newConfig atc.Config
				err = yaml.Unmarshal([]byte(evaluatedTemplate), &newConfig)
				if err != nil {
					t.Fatalf("Failed to unmarshal pipeline config: %v", err)
				}

				team, err := client.FindTeam(atc.DefaultTeamName)
				if err != nil {
					t.Fatalf("Failed to find %s team: %v", atc.DefaultTeamName, err)
				}

				_, existingConfigVersion, _, err := team.PipelineConfig(pipelineRef)
				if err != nil {
					t.Fatalf("Failed to get existing config version: %v", err)
				}

				created, _, warnings, err := team.CreateOrUpdatePipelineConfig(
					pipelineRef,
					existingConfigVersion,
					evaluatedTemplate,
					false,
				)
				if err != nil {
					t.Fatalf("Failed to create pipeline with across step: %v", err)
				}

				for _, warning := range warnings {
					t.Logf("WARNING: updating pipeline config: %s", warning.Message)
				}

				if !created {
					t.Errorf("Expected pipeline config to be created")
				}

				// Unpause the pipeline
				_, err = team.UnpausePipeline(pipelineRef)
				if err != nil {
					t.Fatalf("Failed to unpause pipeline: %v", err)
				}

				// Run the job and verify both across iterations execute
				build, err := team.CreateJobBuild(pipelineRef, "across-test-job")
				if err != nil {
					t.Fatalf("Failed to create job build: %v", err)
				}

				eventSource, err := client.BuildEvents(fmt.Sprintf("%d", build.ID))
				if err != nil {
					t.Fatalf("Failed to get build events: %v", err)
				}
				defer eventSource.Close()

				// Track which messages we've seen
				messagesFound := map[string]bool{
					"First":  false,
					"Second": false,
				}

			eventLoop2:
				for {
					ev, err := eventSource.NextEvent()
					if err != nil {
						if err == io.EOF {
							break
						} else {
							t.Errorf("Failed to parse event: %v", err)
						}
					}
					switch e := ev.(type) {
					case event.Log:
						payload := e.Payload
						// Check if we see our expected messages using substring search
						if strings.Contains(payload, "First") {
							messagesFound["First"] = true
						}
						if strings.Contains(payload, "Second") {
							messagesFound["Second"] = true
						}
					case event.Status:
						switch e.Status {
						case "succeeded":
							// Job completed successfully
							break eventLoop2
						case "failed":
							t.Errorf("Job failed unexpectedly")
							break eventLoop2
						}
					}
				}

				// Verify both messages were found
				if !messagesFound["First"] {
					t.Errorf("Expected to find 'First' in output, but didn't")
				}
				if !messagesFound["Second"] {
					t.Errorf("Expected to find 'Second' in output, but didn't")
				}
			})
		}),
	)

	app.RequireStart()

	t.Cleanup(app.RequireStop)
}

func TestConcourseModule_Proxy(t *testing.T) {
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
			container.WithMainTeamUser("testuser", "testpass"),
			container.WithSecret("Y29uY291cnNlLXdvcmtlcgo="), // default credentials in quickstart, TODO: figure out how to generate this
		),
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"concourse"`
			Proxy   *proxy.TCPProxy                         `name:"concourse"`
		}) {
			t.Run("proxy listen address", func(t *testing.T) {
				expectedHostPort := net.JoinHostPort(mockestra.LoopbackAddress, nat.Port(container.Port).Port())
				if params.Proxy.ListenAddress != expectedHostPort {
					t.Fatalf("Expected proxy to be listening on %s, got %s", expectedHostPort, params.Proxy.TargetAddress)
				}
			})

			// this test is the same as TestConcourseModule, but uses the access proxy IP and port
			endpoint := fmt.Sprintf("http://%s", params.Proxy.ListenAddress)

			t.Run("container environment variable", func(t *testing.T) {
				if params.Request.Env["CONCOURSE_EXTERNAL_URL"] != endpoint {
					t.Fatalf("Expected container environment variable CONCOURSE_EXTERNAL_URL to be %s, got %s", endpoint, params.Request.Env["CONCOURSE_EXTERNAL_URL"])
				}
			})

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
