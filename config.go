package main

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GRPCPort        int           `yaml:"grpc_port"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// LoadConfig читает YAML‑файл и возвращает структуру.
func LoadConfig(path string) (Config, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(f, &cfg); err != nil {
		return Config{}, err
	}

	if cfg.GRPCPort == 0 {
		cfg.GRPCPort = 50051
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}
	return cfg, nil
}
