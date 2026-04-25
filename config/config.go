package config

import (
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Redis    RedisConfig    `yaml:"redis"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DatabaseConfig struct {
	Type string `yaml:"type"`
	DSN  string `yaml:"dsn"`
}

type JWTConfig struct {
	Secret    string `yaml:"secret"`
	ExpireSec int64  `yaml:"expire_sec"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
