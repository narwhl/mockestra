package hydra

import (
	"context"
	"strings"

	"github.com/coreos/go-oidc"
	hydraclient "github.com/ory/hydra-client-go"
)

func GenerateClientCredentials(ctx context.Context, endpoint, name string, redirectURI string) (*hydraclient.OAuth2Client, error) {
	hydraClientConfiguration := hydraclient.NewConfiguration()
	hydraClientConfiguration.Servers = []hydraclient.ServerConfiguration{
		{
			URL: endpoint,
		},
	}
	tokenEndpointAuthMethod := "client_secret_post"
	scope := strings.Join([]string{oidc.ScopeOpenID, "profile", "email", oidc.ScopeOfflineAccess}, " ")
	oauth2Client := hydraclient.NewOAuth2Client()
	oauth2Client.ClientName = &name
	oauth2Client.RedirectUris = []string{redirectURI}
	oauth2Client.Scope = &scope
	oauth2Client.GrantTypes = []string{"authorization_code", "refresh_token"}
	oauth2Client.TokenEndpointAuthMethod = &tokenEndpointAuthMethod

	hydraApiClient := hydraclient.NewAPIClient(hydraClientConfiguration)
	resp, _, err := hydraApiClient.AdminApi.CreateOAuth2Client(ctx).OAuth2Client(*oauth2Client).Execute()
	if err != nil {
		return nil, err
	}
	return resp, nil
}
