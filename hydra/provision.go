package hydra

import (
	"context"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/openfga/go-sdk/oauth2/clientcredentials"
	hydraclient "github.com/ory/hydra-client-go"
	"github.com/testcontainers/testcontainers-go"
)

// OAuthClientOptions configures the OAuth2 client to be created in Hydra.
// It defines the client's name, redirect URIs, and additional scopes beyond
// the default scopes (openid, profile, email, offline_access).
type OAuthClientOptions struct {
	Name             string   // name of the OAuth client
	RedirectURIs     []string // redirect URIs for the client
	AdditionalScopes []string // additional scopes to be allowed along with default openid, profile, email, offline_access scopes
}

// GenerateClientCredentialsHook is a callback function invoked after creating an OAuth2 client in Hydra.
// It receives a pre-configured clientcredentials.Config with the client ID, secret, token URL, and scopes,
// allowing custom handling such as storing credentials, obtaining tokens, or performing validation.
type GenerateClientCredentialsHook func(client *clientcredentials.Config) error

// WithGenerateClientCredentialsHook creates a testcontainers PostReadyHook that provisions an OAuth2 client
// in Hydra after the container is ready. It creates a client with the specified options including
// authorization_code, refresh_token, and client_credentials grant types. After successful creation,
// it invokes the provided hook with a clientcredentials.Config for custom processing such as token
// validation or credential storage.
//
// The function uses the Hydra Admin API to create the client and constructs the token endpoint URL
// from the container's public port mapping.
func WithGenerateClientCredentialsHook(hook GenerateClientCredentialsHook, options OAuthClientOptions) testcontainers.CustomizeRequestOption {
	return WithPostReadyHook(func(endpoints map[string]string) error {
		ctx := context.Background()
		hydraClientConfiguration := hydraclient.NewConfiguration()
		hydraClientConfiguration.Servers = []hydraclient.ServerConfiguration{
			{
				URL: "http://" + endpoints[AdminPort] + "/admin",
			},
		}
		tokenEndpointAuthMethod := "client_secret_post"
		scope := append(options.AdditionalScopes, oidc.ScopeOpenID, "profile", "email", oidc.ScopeOfflineAccess)
		scopes := strings.Join(scope, " ")
		oauth2Client := hydraclient.NewOAuth2Client()
		oauth2Client.ClientName = &options.Name
		oauth2Client.RedirectUris = options.RedirectURIs
		oauth2Client.Scope = &scopes
		oauth2Client.GrantTypes = []string{"authorization_code", "refresh_token", "client_credentials"}
		oauth2Client.TokenEndpointAuthMethod = &tokenEndpointAuthMethod

		hydraApiClient := hydraclient.NewAPIClient(hydraClientConfiguration)
		resp, _, err := hydraApiClient.AdminApi.CreateOAuth2Client(ctx).OAuth2Client(*oauth2Client).Execute()
		if err != nil {
			return err
		}

		return hook(&clientcredentials.Config{
			ClientID:     *resp.ClientId,
			ClientSecret: *resp.ClientSecret,
			TokenURL:     "http://" + endpoints[Port] + "/oauth2/token",
			Scopes:       scope,
		})
	})
}
