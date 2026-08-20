package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestLoadConfig_Base(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `
port: "9000"
control: skyq
sky_q:
  host: 10.0.0.5
  port: 49160
`)

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9000")
	}
	if cfg.Control != "skyq" {
		t.Errorf("Control = %q, want %q", cfg.Control, "skyq")
	}
	if cfg.SkyQ.Host != "10.0.0.5" {
		t.Errorf("SkyQ.Host = %q, want %q", cfg.SkyQ.Host, "10.0.0.5")
	}
}

func TestLoadConfig_DefaultsControlToHarmony(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `port: "8080"`)

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Control != "harmony" {
		t.Errorf("Control = %q, want default %q", cfg.Control, "harmony")
	}
}

func TestLoadConfig_MissingBaseConfigErrors(t *testing.T) {
	dir := t.TempDir() // empty, no config.yaml

	if _, err := loadConfig(dir); err == nil {
		t.Fatal("expected an error for a missing base config.yaml, got nil")
	}
}

func TestLoadConfig_DevOverlayMergesOnTopOfBase(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `
port: "9000"
control: harmony
dev:
  enabled: true
`)
	writeFile(t, filepath.Join(dir, "config-dev.yaml"), `port: "9100"`)

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Port != "9100" {
		t.Errorf("Port = %q, want dev overlay value %q", cfg.Port, "9100")
	}
	// Keys only set in the base config must survive the merge.
	if cfg.Control != "harmony" {
		t.Errorf("Control = %q, want base value %q to survive the dev merge", cfg.Control, "harmony")
	}
}

func TestLoadConfig_DevOverlaySkippedWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `
port: "9000"
dev:
  enabled: false
`)
	writeFile(t, filepath.Join(dir, "config-dev.yaml"), `port: "9100"`)

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Port != "9000" {
		t.Errorf("Port = %q, want base value %q since dev.enabled is false", cfg.Port, "9000")
	}
}

func TestLoadConfig_ConfigFileEnvOverridesEverything(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `
port: "9000"
dev:
  enabled: true
`)
	writeFile(t, filepath.Join(dir, "config-dev.yaml"), `port: "9100"`)

	overridePath := filepath.Join(dir, "override.yaml")
	writeFile(t, overridePath, `port: "9200"`)
	t.Setenv("CONFIG_FILE", overridePath)

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Port != "9200" {
		t.Errorf("Port = %q, want CONFIG_FILE override value %q", cfg.Port, "9200")
	}
}

func TestLoadConfig_EnvVarOverridesFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `port: "9000"`)

	t.Setenv("PORT", "9300")

	cfg, err := loadConfig(dir)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Port != "9300" {
		t.Errorf("Port = %q, want env var override value %q (README documents PORT as an override)", cfg.Port, "9300")
	}
}

func TestLoadConfig_UnknownControlModeErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), `control: not-a-real-mode`)

	if _, err := loadConfig(dir); err == nil {
		t.Fatal("expected an error for an unknown control mode, got nil")
	}
}
