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

func xdgDataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func loadConfig(deckPath string) appConfig {
	// 1. Defaults
	cfg := appConfig{
		Model:      "qwen3:4b-instruct-2507-q4_K_M",
		Threshold:  0.7,
		Step:       24 * time.Hour,
		ServeHost:  "0.0.0.0",
		ServePort:  8765,
		RemotePort: 443,
	}

	// 2. Config file: -config flag, then next to deck, then cwd
	candidates := []string{
		clif.config,
		filepath.Join(filepath.Dir(deckPath), "flash.cfg"),
		"flash.cfg",
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
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
	if v := os.Getenv("FLASH_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Threshold = f
		}
	}
	if v := os.Getenv("FLASH_STEP"); v != "" {
		if d, err := parseDuration(v); err == nil && d > 0 {
			cfg.Step = d
		}
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

	// 4. Flags (highest priority)
	if clif.ollamaURL != "" {
		cfg.OllamaURL = clif.ollamaURL
	}
	if clif.ollamaUser != "" {
		cfg.Username = clif.ollamaUser
	}
	if clif.ollamaPass != "" {
		cfg.Password = clif.ollamaPass
	}
	if clif.model != "" {
		cfg.Model = clif.model
	}
	if clif.threshold != "" {
		if f, err := strconv.ParseFloat(clif.threshold, 64); err == nil {
			cfg.Threshold = f
		}
	}
	if clif.step != "" {
		if d, err := parseDuration(clif.step); err == nil && d > 0 {
			cfg.Step = d
		}
	}
	if clif.db != "" {
		cfg.DB = clif.db
	}
	if clif.serveHost != "" {
		cfg.ServeHost = clif.serveHost
	}
	if clif.servePort != "" {
		if n, err := strconv.Atoi(clif.servePort); err == nil {
			cfg.ServePort = n
		}
	}
	if clif.serveToken != "" {
		cfg.ServeToken = clif.serveToken
	}
	if clif.serveData != "" {
		cfg.ServeData = clif.serveData
	}
	if clif.remotePort != "" {
		if n, err := strconv.Atoi(clif.remotePort); err == nil {
			cfg.RemotePort = n
		}
	}
	if clif.remoteToken != "" {
		cfg.RemoteToken = clif.remoteToken
	}

	// Fallback: XDG data dir (~/.local/share/flash/<deck>.db)
	if cfg.DB == "" && deckPath != "" {
		deckName := strings.TrimSuffix(filepath.Base(deckPath), filepath.Ext(deckPath))
		dataDir := filepath.Join(xdgDataHome(), "flash")
		_ = os.MkdirAll(dataDir, 0o755)
		cfg.DB = filepath.Join(dataDir, deckName+".db")
	}

	return cfg
}
