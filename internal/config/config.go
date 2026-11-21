package config

import (
	"os"
	"time"
)

type Config struct {
	ListenAddr, DataPath, AdminToken, OllamaURL, Model string
	PollInterval                                       time.Duration
	QueueSize                                          int
}

func Load() Config {
	c := Config{ListenAddr: env("PRIYA_LISTEN_ADDR", "127.0.0.1:8080"), DataPath: env("PRIYA_DATA_PATH", "data/priya.db"), AdminToken: os.Getenv("PRIYA_ADMIN_TOKEN"), OllamaURL: env("PRIYA_OLLAMA_URL", "http://127.0.0.1:11434"), Model: env("PRIYA_MODEL", "gemma3:1b"), PollInterval: 45 * time.Second, QueueSize: 50}
	if d, err := time.ParseDuration(env("PRIYA_POLL_INTERVAL", "45s")); err == nil && d >= 15*time.Second {
		c.PollInterval = d
	}
	return c
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
