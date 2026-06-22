package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type appConfig struct {
	DB        string
	OllamaURL string
	Username  string
	Password  string
	Model     string
	Threshold float64
	Step      time.Duration

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
	Step      string  `toml:"step"`

	ServeHost  string `toml:"serve_host"`
	ServePort  int    `toml:"serve_port"`
	ServeToken string `toml:"serve_token"`
	ServeData  string `toml:"serve_data"`

	RemotePort  int    `toml:"remote_port"`
	RemoteToken string `toml:"remote_token"`

	Aliases map[string]string `toml:"aliases"`
}

// parseDuration extends time.ParseDuration with 'd' (day) and 'w' (week) units.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "w") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "w"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n * float64(7*24*time.Hour)), nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}

func loadConfig(deckPath, cliDB string) appConfig {
	// 1. Defaults
	cfg := appConfig{
		Model:      "qwen3:4b-instruct-2507-q4_K_M",
		Threshold:  0.7,
		Step:       24 * time.Hour,
		ServeHost:  "0.0.0.0",
		ServePort:  8765,
		RemotePort: 443,
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
			if fc.Step != "" {
				if d, err := parseDuration(fc.Step); err == nil && d > 0 {
					cfg.Step = d
				}
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
	if v := os.Getenv("FLASH_STEP"); v != "" {
		if d, err := parseDuration(v); err == nil && d > 0 {
			cfg.Step = d
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
