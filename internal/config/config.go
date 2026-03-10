package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Workspace string         `yaml:"workspace"`
	Server    ServerConfig   `yaml:"server"`
	Database  DatabaseConfig `yaml:"database"`
	Log       LogConfig      `yaml:"log"`
	JWT       JWTConfig      `yaml:"jwt"`
	Upload    UploadConfig   `yaml:"upload"`
	Browser   BrowserConfig  `yaml:"browser"`
}

type BrowserConfig struct {
	Visible bool `yaml:"visible"`
}

type UploadConfig struct {
	Dir     string `yaml:"dir"`
	MaxSize int64  `yaml:"max_size"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver"`
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type LogConfig struct {
	Level string `yaml:"level"`
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
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.JWT.Secret == "" {
		cfg.JWT.Secret = "go-ai-agent-default-jwt-secret"
	}
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 24
	}
	if cfg.Upload.Dir == "" {
		cfg.Upload.Dir = "./uploads"
	}
	if cfg.Upload.MaxSize == 0 {
		cfg.Upload.MaxSize = 20 << 20 // 20MB
	}
	return &cfg, nil
}
