package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	UDP struct {
		ReadAddr   string `toml:"read_addr"`
		WriteAddr  string `toml:"write_addr"`
		MaxPayload int    `toml:"max_payload"`
	} `toml:"udp"`
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	if cfg.UDP.ReadAddr == "" || cfg.UDP.WriteAddr == "" {
		return cfg, fmt.Errorf("udp.read_addr and udp.write_addr are required")
	}
	if cfg.UDP.MaxPayload < 64 || cfg.UDP.MaxPayload > 1200 {
		return cfg, fmt.Errorf("udp.max_payload must be between 64 and 1200")
	}
	return cfg, nil
}
