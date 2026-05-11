package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	AppName string
}

type LLMConfig struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

func DefaultLLMConfig() LLMConfig {
	return LLMConfig{Provider: "gpt-5.2-codex-compatible"}
}

func LoadLLMConfig(path string) (LLMConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return LLMConfig{}, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return LLMConfig{}, fmt.Errorf("empty llm config: %s", path)
	}
	var cfg LLMConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return LLMConfig{}, err
	}
	return cfg, nil
}

func EnsureDefaultLLMConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	cfg := DefaultLLMConfig()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
