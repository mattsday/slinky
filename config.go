package main

type Config struct {
	Port       string     `mapstructure:"port"`
	HarmonyApi HarmonyApi `mapstructure:"harmony_api"`
	SkyQ       SkyQ       `mapstructure:"sky_q"`
	SkyStream  SkyStream  `mapstructure:"sky_stream"`
	Control    string     `mapstructure:"control"` // "harmony", "skyq" or "skystream"
	Dev        Dev        `mapstructure:"dev"`
	Stream     Stream     `mapstructure:"stream"`
}

type SkyQ struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// SkyStream configures the control: skystream backend (skystream.go /
// skystream_transport.go). CertFile/KeyFile point at a local mTLS client
// certificate+key pair - Slinky does not vendor or embed these credentials
// itself (see docs/plans/sky-stream-support.md for why).
type SkyStream struct {
	Host     string `mapstructure:"host"`
	MAC      string `mapstructure:"mac"` // optional; enables Wake-on-LAN in a future iteration
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type HarmonyApi struct {
	Url             string `mapstructure:"url"`
	Hubs            []Hub  `mapstructure:"hubs"`
	DefaultHub      string `mapstructure:"default_hub"`
	DefaultActivity string `mapstructure:"default_activity"`
}

type Hub struct {
	Name       string   `mapstructure:"name"`
	Activities []string `mapstructure:"activities"`
}
type Dev struct {
	Enabled bool   `mapstructure:"enabled"`
	Stream  string `mapstructure:"stream"`
}

type Stream struct {
	Quality []Quality   `mapstructure:"quality"`
	HLS     []HLSStream `mapstructure:"hls"`
}

type Quality struct {
	Name     string `mapstructure:"name"`
	Location string `mapstructure:"location"`
	Default  bool   `mapstructure:"default"`
}

type HLSStream struct {
	Name       string `mapstructure:"name"`
	Location   string `mapstructure:"location"`
	Bandwidth  int    `mapstructure:"bandwidth"`
	Resolution string `mapstructure:"resolution"`
	Codecs     string `mapstructure:"codecs"`
	FrameRate  string `mapstructure:"frame_rate"`
}
