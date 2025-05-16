package zitadel

import (
	"context"
	"fmt"

	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/app"
	"github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/management"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

func GenerateClientCredentials(ctx context.Context, name, host, port, keyPath, projectName, redirectURI string) (*management.AddOIDCAppResponse, error) {
	conf := zitadel.New(host, zitadel.WithInsecure(port))
	zitadelClient, err := client.New(ctx, conf, client.WithAuth(client.DefaultServiceUserAuthentication(keyPath, client.ScopeZitadelAPI())))
	if err != nil {
		return nil, fmt.Errorf("failed to create zitadel client: %w", err)
	}
	project, err := zitadelClient.ManagementService().AddProject(ctx, &management.AddProjectRequest{
		Name: projectName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add project: %w", err)
	}
	oidcApp, err := zitadelClient.ManagementService().AddOIDCApp(ctx, &management.AddOIDCAppRequest{
		ProjectId: project.Id,
		Name:      name,
		GrantTypes: []app.OIDCGrantType{
			app.OIDCGrantType_OIDC_GRANT_TYPE_AUTHORIZATION_CODE,
			app.OIDCGrantType_OIDC_GRANT_TYPE_REFRESH_TOKEN,
		},
		RedirectUris: []string{redirectURI},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add oidc app: %w", err)
	}
	return oidcApp, nil
}
