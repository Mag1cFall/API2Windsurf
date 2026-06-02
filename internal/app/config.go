package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	BaseURL       string `json:"base_url"`
	APIKey        string `json:"api_key"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Port          int    `json:"port"`
	ShowReasoning *bool  `json:"show_reasoning,omitempty"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
}

func defaultConfig() Config {
	on := true
	return Config{
		BaseURL:       "http://127.0.0.1:8317",
		APIKey:        "",
		Provider:      "openai",
		Model:         "claude-opus-4.7",
		Port:          443,
		ShowReasoning: &on,
	}
}

func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".api2windsurf")
	return dir, os.MkdirAll(dir, 0o755)
}

func configPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultConfig()
			_ = saveConfig(cfg)
			return cfg, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg.normalized(), nil
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	cfg = cfg.normalized()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c Config) normalized() Config {
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	if c.Provider != "anthropic" && c.Provider != "google" {
		c.Provider = "openai"
	}
	c.Model = strings.TrimSpace(c.Model)
	if c.Port == 0 {
		c.Port = 443
	}
	if c.ShowReasoning == nil {
		on := true
		c.ShowReasoning = &on
	}
	if c.MaxTokens < 0 {
		c.MaxTokens = 0
	}
	return c
}

func (c Config) validate() error {
	switch {
	case c.BaseURL == "":
		return errors.New("base_url is required")
	case c.APIKey == "":
		return errors.New("api_key is required")
	case c.Model == "":
		return errors.New("model is required")
	}
	return nil
}

func (c Config) showReasoning() bool {
	return c.ShowReasoning == nil || *c.ShowReasoning
}
