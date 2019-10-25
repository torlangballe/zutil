package zazure

import (
	"fmt"
	"strconv"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/gobuffalo/envy"
	"github.com/torlangballe/zutil/zlog"
)

var (
	ConfigClientID               string
	ConfigClientSecret           string
	ConfigTenantID               string
	ConfigSubscriptionID         string
	ConfigLocationDefault        string
	ConfigLocation               string
	ConfigAuthorizationServerURL string
	ConfigUseDeviceFlow          bool
	ConfigKeepResources          bool
	ConfigUserAgent              string
	ConfigCloudName              = "AzurePublicCloud"
	configEnvironment            *azure.Environment
	// BaseGroupName          string
	//	GroupName              string // deprecated, use baseGroupName instead
)

func Init() error {
	envy.Load()
	azureEnv, _ := azure.EnvironmentFromName("AzurePublicCloud") // shouldn't fail
	configEnvironment = &azureEnv
	ConfigAuthorizationServerURL = azureEnv.ActiveDirectoryEndpoint

	// AZURE_GROUP_NAME and `config.GroupName()` are deprecated.
	// Use AZURE_BASE_GROUP_NAME and `config.GenerateGroupName()` instead.
	//	groupName := envy.Get("AZURE_GROUP_NAME", "azure-go-samples")
	//	BaseGroupName = envy.Get("AZURE_BASE_GROUP_NAME", groupName)

	ConfigLocationDefault = envy.Get("AZURE_LOCATION_DEFAULT", "westus2")
	ConfigLocation = ConfigLocationDefault
	var err error
	ConfigUseDeviceFlow, err = strconv.ParseBool(envy.Get("AZURE_USE_DEVICEFLOW", "0"))
	if err != nil {
		zlog.Fatal(err, "invalid value specified for AZURE_USE_DEVICEFLOW, disabling")
		ConfigUseDeviceFlow = false
	}
	ConfigKeepResources, err = strconv.ParseBool(envy.Get("AZURE_SAMPLES_KEEP_RESOURCES", "0"))
	if err != nil {
		zlog.Fatal(err, "invalid value specified for AZURE_SAMPLES_KEEP_RESOURCES, discarding")
		ConfigKeepResources = false
	}

	// these must be provided by environment
	// clientID
	ConfigClientID, err = envy.MustGet("AZURE_CLIENT_ID")
	if err != nil {
		return zlog.Fatal(err, "expected env vars not provided")
	}

	// clientSecret
	ConfigClientSecret, err = envy.MustGet("AZURE_CLIENT_SECRET")
	if err != nil && ConfigUseDeviceFlow != true { // don't need a secret for device flow
		return zlog.Fatal(err, "expected env vars not provided")
	}

	// tenantID (AAD)
	ConfigTenantID, err = envy.MustGet("AZURE_TENANT_ID")
	if err != nil {
		return zlog.Fatal(err, "expected env vars not provided")
	}

	// subscriptionID (ARM)
	ConfigSubscriptionID, err = envy.MustGet("AZURE_SUBSCRIPTION_ID")
	if err != nil {
		return zlog.Fatal(err, "expected env vars not provided")
	}

	return nil
}

// OAuthGrantType specifies which grant type to use.
type OAuthGrantType int

const (
	// OAuthGrantTypeServicePrincipal for client credentials flow
	OAuthGrantTypeServicePrincipal OAuthGrantType = iota
	// OAuthGrantTypeDeviceFlow for device flow
	OAuthGrantTypeDeviceFlow
)

// GrantType returns what grant type has been configured.
func grantType() OAuthGrantType {
	if ConfigUseDeviceFlow {
		return OAuthGrantTypeDeviceFlow
	}
	return OAuthGrantTypeServicePrincipal
}

var armAuthorizer autorest.Authorizer

func getResourceManagementAuthorizer() (autorest.Authorizer, error) {
	if armAuthorizer != nil {
		return armAuthorizer, nil
	}

	var a autorest.Authorizer
	var err error

	a, err = getAuthorizerForResource(grantType(), configEnvironment.ResourceManagerEndpoint)

	if err == nil {
		// cache
		armAuthorizer = a
	} else {
		// clear cache
		armAuthorizer = nil
	}
	return armAuthorizer, err
}

func getAuthorizerForResource(grantType OAuthGrantType, resource string) (autorest.Authorizer, error) {

	var a autorest.Authorizer
	var err error

	switch grantType {

	case OAuthGrantTypeServicePrincipal:
		oauthConfig, err := adal.NewOAuthConfig(configEnvironment.ActiveDirectoryEndpoint, ConfigTenantID)
		if err != nil {
			return nil, err
		}

		token, err := adal.NewServicePrincipalToken(*oauthConfig, ConfigClientID, ConfigClientSecret, resource)
		if err != nil {
			return nil, err
		}
		a = autorest.NewBearerAuthorizer(token)

	case OAuthGrantTypeDeviceFlow:
		deviceconfig := auth.NewDeviceFlowConfig(ConfigClientID, ConfigTenantID)
		deviceconfig.Resource = resource
		a, err = deviceconfig.Authorizer()
		if err != nil {
			return nil, err
		}

	default:
		return a, fmt.Errorf("invalid grant type specified")
	}

	return a, err
}
