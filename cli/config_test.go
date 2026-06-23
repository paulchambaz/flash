package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg := loadConfig("")
	if cfg.Model == "" {
		t.Error("default Model should not be empty")
	}
	if cfg.Threshold == 0 {
		t.Error("default Threshold should not be 0")
	}
	if cfg.Pace == 0 {
		t.Error("default Pace should not be 0")
	}
	if cfg.ServePort == 0 {
		t.Error("default ServePort should not be 0")
	}
	if cfg.RemotePort == 0 {
		t.Error("default RemotePort should not be 0")
	}
}

func TestLoadConfigDBFallback(t *testing.T) {
	cfg := loadConfig("/some/path/deck.md")
	want := filepath.Join(xdgDataHome(), "flash", "deck.db")
	if cfg.DB != want {
		t.Errorf("DB = %q, want %q", cfg.DB, want)
	}
}

func TestLoadConfigEnvVars(t *testing.T) {
	t.Setenv("FLASH_OLLAMA_URL", "http://custom-ollama:11434/api/chat")
	t.Setenv("FLASH_OLLAMA_USER", "user1")
	t.Setenv("FLASH_OLLAMA_PASS", "pass1")
	t.Setenv("FLASH_OLLAMA_MODEL", "llama3")
	t.Setenv("FLASH_DB", "/env/db.db")
	t.Setenv("FLASH_PACE", "12h")
	t.Setenv("FLASH_SERVE_PORT", "9999")
	t.Setenv("FLASH_SERVE_TOKEN", "tok123")
	t.Setenv("FLASH_REMOTE_PORT", "8443")
	t.Setenv("FLASH_REMOTE_TOKEN", "remtok")

	cfg := loadConfig("")

	if cfg.OllamaURL != "http://custom-ollama:11434/api/chat" {
		t.Errorf("OllamaURL = %q", cfg.OllamaURL)
	}
	if cfg.Username != "user1" {
		t.Errorf("Username = %q", cfg.Username)
	}
	if cfg.Password != "pass1" {
		t.Errorf("Password = %q", cfg.Password)
	}
	if cfg.Model != "llama3" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.DB != "/env/db.db" {
		t.Errorf("DB = %q", cfg.DB)
	}
	if cfg.Pace != 12*time.Hour {
		t.Errorf("Pace = %v, want 12h", cfg.Pace)
	}
	if cfg.ServePort != 9999 {
		t.Errorf("ServePort = %d, want 9999", cfg.ServePort)
	}
	if cfg.ServeToken != "tok123" {
		t.Errorf("ServeToken = %q", cfg.ServeToken)
	}
	if cfg.RemotePort != 8443 {
		t.Errorf("RemotePort = %d, want 8443", cfg.RemotePort)
	}
	if cfg.RemoteToken != "remtok" {
		t.Errorf("RemoteToken = %q", cfg.RemoteToken)
	}
}

func TestLoadConfigEnvInvalidValues(t *testing.T) {
	defaults := loadConfig("")

	t.Setenv("FLASH_SERVE_PORT", "not-a-number")
	t.Setenv("FLASH_REMOTE_PORT", "not-a-number")
	t.Setenv("FLASH_PACE", "not-a-duration")

	cfg := loadConfig("")
	if cfg.ServePort != defaults.ServePort {
		t.Errorf("invalid FLASH_SERVE_PORT should keep default %d, got %d", defaults.ServePort, cfg.ServePort)
	}
	if cfg.RemotePort != defaults.RemotePort {
		t.Errorf("invalid FLASH_REMOTE_PORT should keep default %d, got %d", defaults.RemotePort, cfg.RemotePort)
	}
	if cfg.Pace != defaults.Pace {
		t.Errorf("invalid FLASH_PACE should keep default %v, got %v", defaults.Pace, cfg.Pace)
	}
}

func TestLoadConfigTOMLFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "flash")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	tomlContent := `
ollama_url = "http://toml-ollama/api/chat"
ollama_model = "toml-model"
pace = "12h"
serve_port = 7777
remote_token = "toml-token"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "flash.cfg"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig("")

	if cfg.OllamaURL != "http://toml-ollama/api/chat" {
		t.Errorf("OllamaURL = %q", cfg.OllamaURL)
	}
	if cfg.Model != "toml-model" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.Pace != 12*time.Hour {
		t.Errorf("Pace = %v, want 12h", cfg.Pace)
	}
	if cfg.ServePort != 7777 {
		t.Errorf("ServePort = %d, want 7777", cfg.ServePort)
	}
	if cfg.RemoteToken != "toml-token" {
		t.Errorf("RemoteToken = %q", cfg.RemoteToken)
	}
}

func TestLoadConfigEnvOverridesToml(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "flash")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := os.WriteFile(filepath.Join(cfgDir, "flash.cfg"), []byte(`ollama_url = "http://toml/api/chat"`), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FLASH_OLLAMA_URL", "http://env/api/chat")

	cfg := loadConfig("")
	if cfg.OllamaURL != "http://env/api/chat" {
		t.Errorf("env should override TOML: OllamaURL = %q", cfg.OllamaURL)
	}
}

func TestLoadConfigEnvDBOverride(t *testing.T) {
	t.Setenv("FLASH_DB", "/env/db.db")
	cfg := loadConfig("deck.md")
	if cfg.DB != "/env/db.db" {
		t.Errorf("env FLASH_DB should override XDG fallback: DB = %q", cfg.DB)
	}
}
