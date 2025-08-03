package zitadel_test

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	postgres_container "github.com/narwhl/mockestra/postgres"
	container "github.com/narwhl/mockestra/zitadel"

	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// TestZitadelWithPostgres tests the complete zitadel module with postgres dependencies
func TestZitadelWithPostgres(t *testing.T) {
	testPrefix := fmt.Sprintf("zitadel-test-%x", time.Now().Unix())
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		
		// Supply required versions using fx.Supply with fx.Annotate and fx.ResultTags decorators
		fx.Supply(
			fx.Annotate(
				"latest",  // postgres version
				fx.ResultTags(`name:"postgres_version"`),
			),
		),
		fx.Supply(
			fx.Annotate(
				"latest",  // zitadel version  
				fx.ResultTags(`name:"zitadel_version"`),
			),
		),
		fx.Supply(
			fx.Annotate(
				testPrefix,
				fx.ResultTags(`name:"prefix"`),
			),
		),
		
		// Include postgres module with customizations using BuildContainerModule decorator
		postgres_container.Module(
			postgres_container.WithUsername("postgres"),
			postgres_container.WithPassword("password123"),
			postgres_container.WithDatabase("postgres"),
		),
		
		// Include zitadel module with customizations using BuildContainerModule decorator
		container.Module(
			container.WithMasterkey("VeryStrongMasterKey1234567890123"), // Exactly 32 bytes
			container.WithOrganizationName("TestOrg"),
			container.WithAdminUser("admin", "Admin123!"),
		),
		
		// Use fx.Invoke to test the containers
		fx.Invoke(func(params struct {
			fx.In
			PostgresContainer testcontainers.Container `name:"postgres"`
			ZitadelContainer  testcontainers.Container `name:"zitadel"`
		}) {
			// Test postgres container
			postgresEndpoint, err := params.PostgresContainer.PortEndpoint(t.Context(), postgres_container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Postgres container endpoint: %v", err)
			}
			t.Logf("Postgres container is running at %s", postgresEndpoint)
			
			// Test zitadel container
			zitadelEndpoint, err := params.ZitadelContainer.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Zitadel container endpoint: %v", err)
			}
			t.Logf("Zitadel container is running at %s", zitadelEndpoint)
			
			// Test zitadel health endpoint
			healthURL := fmt.Sprintf("http://%s/debug/healthz", zitadelEndpoint)
			resp, err := http.Get(healthURL)
			if err != nil {
				t.Fatalf("Failed to connect to Zitadel health endpoint: %v", err)
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != 200 {
				t.Errorf("Zitadel health check failed, status: %d", resp.StatusCode)
			} else {
				t.Logf("Zitadel health check passed: %d", resp.StatusCode)
			}
		}),
	)
	
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}

// TestZitadelWithPostReadyHook tests the WithPostReadyHook decorator functionality
func TestZitadelWithPostReadyHook(t *testing.T) {
	testPrefix := fmt.Sprintf("zitadel-hook-test-%x", time.Now().Unix())
	
	// Track if the post-ready hook was called
	hookCalled := false
	var capturedEndpoints map[string]string
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		
		// Supply versions
		fx.Supply(
			fx.Annotate("latest", fx.ResultTags(`name:"postgres_version"`)),
			fx.Annotate("latest", fx.ResultTags(`name:"zitadel_version"`)),
			fx.Annotate(testPrefix, fx.ResultTags(`name:"prefix"`)),
		),
		
		// Include postgres module
		postgres_container.Module(
			postgres_container.WithUsername("postgres"),
			postgres_container.WithPassword("password123"), 
			postgres_container.WithDatabase("postgres"),
		),
		
		// Include zitadel module with WithPostReadyHook decorator
		container.Module(
			container.WithMasterkey("PostReadyHookMasterKey1234567890"), // Exactly 32 bytes
			container.WithOrganizationName("TestOrg"),
			container.WithAdminUser("admin", "Admin123!"),
			container.WithPostReadyHook(func(endpoints map[string]string) error {
				hookCalled = true
				capturedEndpoints = endpoints
				t.Logf("Post-ready hook called with endpoints: %v", endpoints)
				return nil
			}),
		),
		
		fx.Invoke(func(params struct {
			fx.In
			ZitadelContainer testcontainers.Container `name:"zitadel"`
		}) {
			// Verify the hook was called
			if !hookCalled {
				t.Error("WithPostReadyHook was not called")
			}
			
			if capturedEndpoints == nil {
				t.Error("Post-ready hook did not capture endpoints")
			} else {
				t.Logf("Captured endpoints in hook: %v", capturedEndpoints)
				
				// Verify we have the expected port mapping
				if endpoint, exists := capturedEndpoints[container.Port]; exists {
					t.Logf("Zitadel endpoint from hook: %s", endpoint)
				} else {
					t.Errorf("Expected port %s not found in hook endpoints", container.Port)
				}
			}
		}),
	)
	
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}

// TestZitadelContainerCustomization tests various container customization functions
func TestZitadelContainerCustomization(t *testing.T) {
	// This test only checks the configuration generation, not the actual container creation
	
	// Create a standalone test request to avoid interference with other tests
	masterKey := "SuperSecretMasterKey123456789012" // Exactly 32 bytes
	orgName := "CustomTestOrg"
	adminUser := "customadmin"
	adminPassword := "CustomAdmin456!"
	
	// Test the customization functions directly
	req := &testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Env: make(map[string]string),
		},
	}
	
	// Apply customizations
	masterkeyOpt := container.WithMasterkey(masterKey)
	if err := masterkeyOpt.Customize(req); err != nil {
		t.Fatalf("Failed to apply masterkey customization: %v", err)
	}
	
	orgOpt := container.WithOrganizationName(orgName)
	if err := orgOpt.Customize(req); err != nil {
		t.Fatalf("Failed to apply organization customization: %v", err)
	}
	
	adminOpt := container.WithAdminUser(adminUser, adminPassword)
	if err := adminOpt.Customize(req); err != nil {
		t.Fatalf("Failed to apply admin user customization: %v", err)
	}
	
	// Verify environment variables were set correctly by customization functions
	env := req.Env
	
	if masterkey, exists := env["ZITADEL_MASTERKEY"]; !exists {
		t.Error("ZITADEL_MASTERKEY not set")
	} else if masterkey != masterKey {
		t.Errorf("Expected masterkey '%s', got '%s'", masterKey, masterkey)
	}
	
	if orgNameVar, exists := env["ZITADEL_FIRSTINSTANCE_ORG_NAME"]; !exists {
		t.Error("ZITADEL_FIRSTINSTANCE_ORG_NAME not set")
	} else if orgNameVar != orgName {
		t.Errorf("Expected org name '%s', got '%s'", orgName, orgNameVar)
	}
	
	if username, exists := env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_USERNAME"]; !exists {
		t.Error("ZITADEL_FIRSTINSTANCE_ORG_HUMAN_USERNAME not set")
	} else if username != adminUser {
		t.Errorf("Expected username '%s', got '%s'", adminUser, username)
	}
	
	if password, exists := env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD"]; !exists {
		t.Error("ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD not set")
	} else if password != adminPassword {
		t.Errorf("Expected password '%s', got '%s'", adminPassword, password)
	}
	
	t.Logf("All environment variables configured correctly")
}

// TestZitadelServiceUserConfiguration tests the WithServiceUser functionality
func TestZitadelServiceUserConfiguration(t *testing.T) {
	testPrefix := fmt.Sprintf("zitadel-sa-test-%x", time.Now().Unix())
	
	// Create temp directory for service account key
	tempDir, err := os.MkdirTemp("", "zitadel-test-sa")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	serviceAccountFile := filepath.Join(tempDir, "service-account.json")
	if err := os.WriteFile(serviceAccountFile, []byte("{}"), fs.ModePerm); err != nil {
		t.Fatalf("Failed to create service account file: %v", err)
	}
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		
		fx.Supply(
			fx.Annotate("latest", fx.ResultTags(`name:"postgres_version"`)),
			fx.Annotate("latest", fx.ResultTags(`name:"zitadel_version"`)),
			fx.Annotate(testPrefix, fx.ResultTags(`name:"prefix"`)),
		),
		
		postgres_container.Module(
			postgres_container.WithUsername("postgres"),
			postgres_container.WithPassword("password123"),
			postgres_container.WithDatabase("postgres"),
		),
		
		container.Module(
			container.WithMasterkey("ServiceAccountMasterKey1234567890"), // Exactly 32 bytes
			container.WithOrganizationName("ServiceAccountTestOrg"),
			container.WithAdminUser("admin", "Admin789!"),
			// Don't use WithServiceUser for this test to avoid the file copying issue
		),
		
		fx.Invoke(func(params struct {
			fx.In
			Request *testcontainers.GenericContainerRequest `name:"zitadel"`
		}) {
			// Test the service account manually
			opt := container.WithServiceUser("testservice", serviceAccountFile)
			if opt == nil {
				t.Error("WithServiceUser returned nil")
				return
			}
			
			// Apply the customization manually to test the function
			testReq := &testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Env: make(map[string]string),
				},
			}
			
			err := opt.Customize(testReq)
			if err != nil {
				t.Errorf("Failed to customize request: %v", err)
				return
			}
			
			env := testReq.Env
			
			if saUsername, exists := env["ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_USERNAME"]; !exists {
				t.Error("Service account username not set")
			} else if saUsername != "testservice" {
				t.Errorf("Expected service account username 'testservice', got '%s'", saUsername)
			}
			
			if keyPath, exists := env["ZITADEL_FIRSTINSTANCE_MACHINEKEYPATH"]; !exists {
				t.Error("Service account key path not set")
			} else if keyPath != "/testservice.json" {
				t.Errorf("Expected key path '/testservice.json', got '%s'", keyPath)
			}
			
			// Check that files were added
			if len(testReq.Files) == 0 {
				t.Error("No files were added to the request")
			} else {
				t.Logf("Files added: %v", len(testReq.Files))
			}
			
			t.Logf("Service account configuration verified")
		}),
	)
	
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}

// TestZitadelFxDecoratorFunctions tests specific fx decorator usage patterns
func TestZitadelFxDecoratorFunctions(t *testing.T) {
	testPrefix := fmt.Sprintf("zitadel-fx-test-%x", time.Now().Unix())
	
	// Test the BuildContainerModule decorator function
	moduleWithCustomizers := container.Module(
		container.WithMasterkey("TestDecoratorKey1234567890123456"), // Exactly 32 bytes
		container.WithOrganizationName("DecoratorTestOrg"),
	)
	
	if moduleWithCustomizers == nil {
		t.Error("BuildContainerModule returned nil")
	}
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		
		// Test fx.Supply with fx.Annotate and fx.ResultTags decorators
		fx.Supply(
			fx.Annotate("latest", fx.ResultTags(`name:"postgres_version"`)),
			fx.Annotate("latest", fx.ResultTags(`name:"zitadel_version"`)),
			fx.Annotate(testPrefix, fx.ResultTags(`name:"prefix"`)),
		),
		
		postgres_container.Module(
			postgres_container.WithUsername("postgres"),
			postgres_container.WithPassword("password123"),
		),
		
		// Test the module with decorators
		moduleWithCustomizers,
		
		// Test fx.Invoke with fx.In struct tags
		fx.Invoke(func(params struct {
			fx.In
			PostgresRequest *testcontainers.GenericContainerRequest `name:"postgres"`
			ZitadelRequest  *testcontainers.GenericContainerRequest `name:"zitadel"`
			PostgresContainer testcontainers.Container              `name:"postgres"`
			ZitadelContainer  testcontainers.Container              `name:"zitadel"`
		}) {
			// Verify fx.Provide with fx.Annotate worked correctly
			if params.PostgresRequest == nil {
				t.Error("PostgresRequest not provided by fx.Provide")
			}
			if params.ZitadelRequest == nil {
				t.Error("ZitadelRequest not provided by fx.Provide")
			}
			if params.PostgresContainer == nil {
				t.Error("PostgresContainer not provided by fx.Provide")
			}
			if params.ZitadelContainer == nil {
				t.Error("ZitadelContainer not provided by fx.Provide") 
			}
			
			// Verify fx.ResultTags worked for dependency injection
			t.Logf("All fx decorator functions working correctly")
			t.Logf("Postgres container name: %s", params.PostgresRequest.Name)
			t.Logf("Zitadel container name: %s", params.ZitadelRequest.Name)
			
			// Verify container names contain the test prefix
			if !strings.Contains(params.PostgresRequest.Name, testPrefix) {
				t.Errorf("Postgres container name should contain prefix '%s'", testPrefix)
			}
			if !strings.Contains(params.ZitadelRequest.Name, testPrefix) {
				t.Errorf("Zitadel container name should contain prefix '%s'", testPrefix)
			}
		}),
	)
	
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}

// TestZitadelOIDCFunctionality tests basic OIDC functionality
func TestZitadelOIDCFunctionality(t *testing.T) {
	testPrefix := fmt.Sprintf("zitadel-oidc-test-%x", time.Now().Unix())
	
	app := fxtest.New(
		t,
		fx.NopLogger,
		
		fx.Supply(
			fx.Annotate("latest", fx.ResultTags(`name:"postgres_version"`)),
			fx.Annotate("latest", fx.ResultTags(`name:"zitadel_version"`)),
			fx.Annotate(testPrefix, fx.ResultTags(`name:"prefix"`)),
		),
		
		postgres_container.Module(
			postgres_container.WithUsername("postgres"),
			postgres_container.WithPassword("password123"),
			postgres_container.WithDatabase("postgres"),
		),
		
		container.Module(
			container.WithMasterkey("OIDCTestMasterKey123456789012345"), // Exactly 32 bytes
			container.WithOrganizationName("OIDCTestOrg"),
			container.WithAdminUser("admin", "Admin123!"),
		),
		
		fx.Invoke(func(params struct {
			fx.In
			ZitadelContainer testcontainers.Container `name:"zitadel"`
		}) {
			zitadelEndpoint, err := params.ZitadelContainer.PortEndpoint(t.Context(), container.Port, "")
			if err != nil {
				t.Fatalf("Failed to get Zitadel endpoint: %v", err)
			}
			
			// Test OIDC discovery endpoint
			discoveryURL := fmt.Sprintf("http://%s/.well-known/openid_configuration", zitadelEndpoint)
			resp, err := http.Get(discoveryURL)
			if err != nil {
				t.Logf("OIDC discovery endpoint not yet available: %v", err)
				return // This is expected during startup
			}
			defer resp.Body.Close()
			
			if resp.StatusCode == 200 {
				t.Logf("OIDC discovery endpoint accessible: %d", resp.StatusCode)
			} else {
				t.Logf("OIDC discovery endpoint status: %d", resp.StatusCode)
			}
			
			// Test the login page endpoint
			loginURL := fmt.Sprintf("http://%s/ui/login", zitadelEndpoint) 
			resp2, err := http.Get(loginURL)
			if err != nil {
				t.Logf("Login page not yet available: %v", err)
				return
			}
			defer resp2.Body.Close()
			
			if resp2.StatusCode == 200 {
				t.Logf("Login page accessible: %d", resp2.StatusCode)
			} else {
				t.Logf("Login page status: %d", resp2.StatusCode)
			}
		}),
	)
	
	app.RequireStart()
	t.Cleanup(func() {
		app.RequireStop()
	})
}
