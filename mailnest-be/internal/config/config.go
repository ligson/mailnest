package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	OAuth    OAuthConfig    `yaml:"oauth"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type AppConfig struct {
	DataDir           string `yaml:"dataDir"`
	AllowRegistration bool   `yaml:"allowRegistration"`
	JWTSecret         string `yaml:"jwtSecret"`
	JWTExpireHours    int    `yaml:"jwtExpireHours"`
	CredentialSecret  string `yaml:"credentialSecret"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver"`
	DSN          string `yaml:"dsn"`
	Path         string `yaml:"path"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
}

type OAuthConfig struct {
	Microsoft MicrosoftOAuthConfig `yaml:"microsoft"`
}

type MicrosoftOAuthConfig struct {
	ClientID     string `yaml:"clientId"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURL  string `yaml:"redirectUrl"`
	Tenant       string `yaml:"tenant"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		App: AppConfig{
			DataDir:           "./data",
			AllowRegistration: true,
			JWTSecret:         "change-me-in-production",
			JWTExpireHours:    168,
			CredentialSecret:  "change-me-32-byte-secret-value",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "./data/mailnest.db",
		},
		OAuth: OAuthConfig{
			Microsoft: MicrosoftOAuthConfig{
				Tenant:      "consumers",
				RedirectURL: "http://127.0.0.1:5173/oauth/microsoft/callback",
			},
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
