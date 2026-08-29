package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	API   string `json:"api"`
	Token string `json:"token"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: home: %w", err)
	}
	return filepath.Join(home, ".kariz", "config.json"), nil
}

func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	return nil
}

func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse: %w", err)
	}
	return cfg, nil
}
