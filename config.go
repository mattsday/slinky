package main

type Config struct {
	Port       string     `mapstructure:"port"`
	HarmonyApi HarmonyApi `mapstructure:"harmony_api"`
	Dev        Dev        `mapstructure:"dev"`
	Stream     Stream     `mapstructure:"stream"`
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
	HQ     string `mapstructure:"hq"`
	Stable string `mapstructure:"stable"`
}
