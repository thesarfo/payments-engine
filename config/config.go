package config

import (
	"fmt"
	"os"
	"strings"
)

type ServerConfig struct {
	DatabaseURL string
	RedisAddr   string
	ListenAddr  string
}

type SeedConfig struct {
	DatabaseURL string
}

func LoadServerConfig() (ServerConfig, error) {
	databaseURL, err := requiredEnv("DATABASE_URL")
	if err != nil {
		return ServerConfig{}, err
	}

	return ServerConfig{
		DatabaseURL: databaseURL,
		RedisAddr:   optionalEnv("REDIS_ADDR"),
		ListenAddr:  optionalEnvWithDefault("LISTEN_ADDR", ":8080"),
	}, nil
}

func LoadSeedConfig() (SeedConfig, error) {
	databaseURL, err := requiredEnv("DATABASE_URL")
	if err != nil {
		return SeedConfig{}, err
	}

	return SeedConfig{
		DatabaseURL: databaseURL,
	}, nil
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}

	return value, nil
}

func optionalEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func optionalEnvWithDefault(key, fallback string) string {
	value := optionalEnv(key)
	if value == "" {
		return fallback
	}

	return value
}
