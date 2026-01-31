package kanidm

// RadiusConfig represents the configuration for a Kanidm RADIUS server.
// Note: Kanidm RADIUS runs as a separate container (kanidm/radius).
// This configuration can be used to generate a radius.toml configuration file.
type RadiusConfig struct {
	// URI is the URL to the Kanidm server
	URI string `toml:"uri"`
	// AuthToken is the API token for the RADIUS service account
	AuthToken string `toml:"auth_token"`
	// VerifyHostnames enables hostname verification for the Kanidm server
	VerifyHostnames bool `toml:"verify_hostnames"`
	// VerifyCA enables strict CA verification
	VerifyCA bool `toml:"verify_ca"`
	// RadiusDefaultVLAN is the default VLAN for groups that don't specify one
	RadiusDefaultVLAN int `toml:"radius_default_vlan"`
	// RadiusRequiredGroups is a list of Kanidm groups which must be a member
	// before they can authenticate via RADIUS
	RadiusRequiredGroups []string `toml:"radius_required_groups"`
	// RadiusGroups is a mapping between Kanidm groups and VLANs
	RadiusGroups []RadiusGroup `toml:"radius_groups"`
	// RadiusClients is a mapping of clients and their authentication tokens
	RadiusClients []RadiusClient `toml:"radius_clients"`
	// RadiusCertPath is the path to the RADIUS TLS certificate
	RadiusCertPath string `toml:"radius_cert_path,omitempty"`
	// RadiusKeyPath is the path to the RADIUS TLS key
	RadiusKeyPath string `toml:"radius_key_path,omitempty"`
	// RadiusCAPath is the path to the Kanidm CA certificate
	RadiusCAPath string `toml:"radius_ca_path,omitempty"`
}

// RadiusGroup represents a mapping between a Kanidm group and a VLAN
type RadiusGroup struct {
	// SPN is the Security Principal Name of the group (e.g., "radius_access_allowed@idm.example.com")
	SPN string `toml:"spn"`
	// VLAN is the VLAN ID to assign to members of this group
	VLAN int `toml:"vlan"`
}

// RadiusClient represents a RADIUS client configuration
type RadiusClient struct {
	// Name is the name of the client
	Name string `toml:"name"`
	// IPAddr is the IP address or CIDR range of the client
	IPAddr string `toml:"ipaddr"`
	// Secret is the shared secret for the client
	Secret string `toml:"secret"`
}
