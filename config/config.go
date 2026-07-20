package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Jenkins  JenkinsConfig  `yaml:"jenkins"`
}

type ServerConfig struct {
	Port      int    `yaml:"port"`
	SecretKey string `yaml:"secret_key"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type AuthConfig struct {
	AdminUser     string `yaml:"admin_user"`
	AdminPassword string `yaml:"admin_password"`
}

type JenkinsConfig struct {
	URL   string `yaml:"url"`
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

var Global *Config

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
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/platform.db"
	}

	Global = &cfg
	log.Printf("[config] loaded from %s, port=%d, db=%s", path, cfg.Server.Port, cfg.Database.Path)
	return &cfg, nil
}
