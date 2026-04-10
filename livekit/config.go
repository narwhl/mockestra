package livekit

type livekitConfig struct {
	Port          int               `yaml:"port"`
	BindAddresses []string          `yaml:"bind_addresses"`
	RTC           rtcConfig         `yaml:"rtc"`
	Keys          map[string]string `yaml:"keys"`
	Room          roomConfig        `yaml:"room"`
	Webhook       *webhookConfig    `yaml:"webhook,omitempty"`
	Logging       loggingConfig     `yaml:"logging"`
}

type rtcConfig struct {
	TCPPort        int    `yaml:"tcp_port"`
	PortRangeStart int    `yaml:"port_range_start"`
	PortRangeEnd   int    `yaml:"port_range_end"`
	NodeIP         string `yaml:"node_ip,omitempty"`
	UseExternalIP  bool   `yaml:"use_external_ip"`
	// UDP is disabled by setting PortRangeStart/End to 0. Omitting them
	// leaves LiveKit's defaults (50000/60000) in place, which is NOT what
	// we want for the simulate environment.
	//
	// NodeIP controls the IP advertised in ICE candidates. Set to
	// 127.0.0.1 so browsers on the host can reach the RTC TCP proxy.
	// UseExternalIP must be false for NodeIP to take effect.
}

type roomConfig struct {
	AutoCreate   bool `yaml:"auto_create"`
	EmptyTimeout int  `yaml:"empty_timeout"`
}

type webhookConfig struct {
	APIKey string   `yaml:"api_key"`
	URLs   []string `yaml:"urls"`
}

type loggingConfig struct {
	Level string `yaml:"level"`
}

func defaultConfig() *livekitConfig {
	return &livekitConfig{
		Port:          7880,
		BindAddresses: []string{""}, // matches LiveKit's DefaultConfig — binds all interfaces
		RTC: rtcConfig{
			TCPPort:        7881,
			PortRangeStart: 0, // disable UDP media range
			PortRangeEnd:   0,
			NodeIP:         "127.0.0.1", // advertised in ICE candidates — must match proxy listen address
			UseExternalIP:  false,       // use NodeIP as-is without STUN lookup
		},
		Keys: map[string]string{
			DefaultAPIKey: DefaultAPISecret,
		},
		Room: roomConfig{
			AutoCreate:   true,
			EmptyTimeout: 300,
		},
		Logging: loggingConfig{
			Level: "info",
		},
	}
}
