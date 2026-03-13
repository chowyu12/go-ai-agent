package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "etc/config.yaml"
	}
	return filepath.Join(home, ".go-agent", "config.yaml")
}

func ConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	p := DefaultConfigPath()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if _, err := os.Stat("etc/config.yaml"); err == nil {
		log.WithField("path", "etc/config.yaml").Info("using legacy config path")
		return "etc/config.yaml"
	}
	return p
}

type Config struct {
	Workspace string         `yaml:"workspace,omitempty"`
	Server    ServerConfig   `yaml:"server,omitempty"`
	Database  DatabaseConfig `yaml:"database,omitempty"`
	Log       LogConfig      `yaml:"log,omitempty"`
	JWT       JWTConfig      `yaml:"jwt,omitempty"`
	Upload    UploadConfig   `yaml:"upload,omitempty"`
	Browser   BrowserConfig  `yaml:"browser,omitempty"`
}

type BrowserConfig struct {
	Visible   bool   `yaml:"visible,omitempty"`
	Width     int    `yaml:"width,omitempty"`
	Height    int    `yaml:"height,omitempty"`
	UserAgent string `yaml:"user_agent,omitempty"`
	Proxy     string `yaml:"proxy,omitempty"`
}

type UploadConfig struct {
	Dir     string `yaml:"dir,omitempty"`
	MaxSize int64  `yaml:"max_size,omitempty"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret,omitempty"`
	ExpireHours int    `yaml:"expire_hours,omitempty"`
}

type ServerConfig struct {
	Host string `yaml:"host,omitempty"`
	Port int    `yaml:"port,omitempty"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver,omitempty"`
	DSN          string `yaml:"dsn,omitempty"`
	MaxOpenConns int    `yaml:"max_open_conns,omitempty"`
	MaxIdleConns int    `yaml:"max_idle_conns,omitempty"`
}

type LogConfig struct {
	Level string `yaml:"level,omitempty"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}
	setDefaults(&cfg)
	return &cfg, nil
}

func (c *Config) NeedsDatabaseSetup() bool {
	return c.Database.Driver == "" || c.Database.DSN == ""
}

func (c *Config) Save(path string) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out := *c
	data, err := yaml.Marshal(&out)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func setDefaults(cfg *Config) {
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
}
