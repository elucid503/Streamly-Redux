package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {

	DiscordClientID string
	DiscordClientSecret string

	ListenAddr string
	StaticDir string

	AllowAnyOrigin bool

}

var ErrMissingCredentials = errors.New("DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET are required")

func Load() (*Config, error) {

	_ = godotenv.Load()

	cfg := &Config{

		DiscordClientID: os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),

		ListenAddr: envOr("LISTEN_ADDR", ":8080"),
		StaticDir: envOr("STATIC_DIR", "web/dist"),

		AllowAnyOrigin: os.Getenv("ALLOW_ANY_ORIGIN") == "1",

	}

	if cfg.DiscordClientID == "" || cfg.DiscordClientSecret == "" {

		return nil, ErrMissingCredentials

	}

	return cfg, nil

}

func envOr(key string, fallback string) string {

	if value := os.Getenv(key); value != "" {

		return value

	}

	return fallback

}
