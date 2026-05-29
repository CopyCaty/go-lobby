package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

const DefaultMineChunkClosureDuration = 5 * time.Minute

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
	Redis    RedisConfig    `yaml:"redis"`
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
	Mine     MineConfig     `yaml:"mine"`
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

type RabbitMQConfig struct {
	URL              string `yaml:"url"`
	Exchange         string `yaml:"exchange"`
	MatchResultQueue string `yaml:"match_result_queue"`
}

type MineConfig struct {
	ChunkClosureDuration string        `yaml:"chunk_closure_duration"`
	chunkClosureDuration time.Duration `yaml:"-"`
}

func (c MineConfig) ChunkClosureDurationValue() time.Duration {
	if c.chunkClosureDuration > 0 {
		return c.chunkClosureDuration
	}
	return DefaultMineChunkClosureDuration
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
	if err := cfg.normalize(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) normalize() error {
	rawDuration := c.Mine.ChunkClosureDuration
	if rawDuration == "" {
		c.Mine.chunkClosureDuration = DefaultMineChunkClosureDuration
		return nil
	}
	duration, err := time.ParseDuration(rawDuration)
	if err != nil {
		return fmt.Errorf("invalid mine.chunk_closure_duration %q: %w", rawDuration, err)
	}
	if duration <= 0 {
		return fmt.Errorf("invalid mine.chunk_closure_duration %q: must be positive", rawDuration)
	}
	c.Mine.chunkClosureDuration = duration
	return nil
}
