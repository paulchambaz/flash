package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type appConfig struct {
	DB        string
	OllamaURL string
	Username  string
	Password  string
	Model     string
	Threshold float64

	ServeHost  string
	ServePort  int
	ServeToken string
	ServeData  string

	RemotePort  int
	RemoteToken string

	Aliases map[string]string
}

type fileConfig struct {
	DB        string  `toml:"db"`
	OllamaURL string  `toml:"ollama_url"`
	Username  string  `toml:"ollama_user"`
	Password  string  `toml:"ollama_pass"`
	Model     string  `toml:"ollama_model"`
	Threshold float64 `toml:"threshold"`

	ServeHost  string `toml:"serve_host"`
	ServePort  int    `toml:"serve_port"`
	ServeToken string `toml:"serve_token"`
	ServeData  string `toml:"serve_data"`

	RemotePort  int    `toml:"remote_port"`
	RemoteToken string `toml:"remote_token"`

	Aliases map[string]string `toml:"aliases"`
}

func loadConfig(deckPath, cliDB string) appConfig {
	// 1. Defaults
	cfg := appConfig{
		OllamaURL:  "https://ollama.chambaz.xyz/api/chat",
		Username:   "paulchambaz",
		Password:   "TPDCS0RG9zI2TjyGFo0pABvvoyK6iDFb",
		Model:      "qwen3:4b-instruct-2507-q4_K_M",
		Threshold:  0.7,
		ServeHost:  "0.0.0.0",
		ServePort:  8765,
		RemotePort: 8765,
	}

	// 2. Config file (flash.cfg next to deck, then cwd)
	candidates := []string{
		filepath.Join(filepath.Dir(deckPath), "flash.cfg"),
		"flash.cfg",
	}
	for _, path := range candidates {
		var fc fileConfig
		if _, err := toml.DecodeFile(path, &fc); err == nil {
			if fc.DB != "" {
				cfg.DB = fc.DB
			}
			if fc.OllamaURL != "" {
				cfg.OllamaURL = fc.OllamaURL
			}
			if fc.Username != "" {
				cfg.Username = fc.Username
			}
			if fc.Password != "" {
				cfg.Password = fc.Password
			}
			if fc.Model != "" {
				cfg.Model = fc.Model
			}
			if fc.Threshold > 0 {
				cfg.Threshold = fc.Threshold
			}
			if fc.ServeHost != "" {
				cfg.ServeHost = fc.ServeHost
			}
			if fc.ServePort != 0 {
				cfg.ServePort = fc.ServePort
			}
			if fc.ServeToken != "" {
				cfg.ServeToken = fc.ServeToken
			}
			if fc.ServeData != "" {
				cfg.ServeData = fc.ServeData
			}
			if fc.RemotePort != 0 {
				cfg.RemotePort = fc.RemotePort
			}
			if fc.RemoteToken != "" {
				cfg.RemoteToken = fc.RemoteToken
			}
			if len(fc.Aliases) > 0 {
				cfg.Aliases = fc.Aliases
			}
			break
		}
	}

	// 3. Env vars
	if v := os.Getenv("FLASH_OLLAMA_URL"); v != "" {
		cfg.OllamaURL = v
	}
	if v := os.Getenv("FLASH_OLLAMA_USER"); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("FLASH_OLLAMA_PASS"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("FLASH_OLLAMA_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("FLASH_DB"); v != "" {
		cfg.DB = v
	}
	if v := os.Getenv("FLASH_SERVE_HOST"); v != "" {
		cfg.ServeHost = v
	}
	if v := os.Getenv("FLASH_SERVE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ServePort = n
		}
	}
	if v := os.Getenv("FLASH_SERVE_TOKEN"); v != "" {
		cfg.ServeToken = v
	}
	if v := os.Getenv("FLASH_SERVE_DATA"); v != "" {
		cfg.ServeData = v
	}
	if v := os.Getenv("FLASH_REMOTE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RemotePort = n
		}
	}
	if v := os.Getenv("FLASH_REMOTE_TOKEN"); v != "" {
		cfg.RemoteToken = v
	}

	// 4. CLI args
	if cliDB != "" {
		cfg.DB = cliDB
	}

	// Fallback: derive DB path from deck filename
	if cfg.DB == "" && deckPath != "" {
		cfg.DB = strings.TrimSuffix(deckPath, filepath.Ext(deckPath)) + ".db"
	}

	return cfg
}
