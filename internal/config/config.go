package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	WorkerCount     int
	RetryLimit      int
	ShutdownTimeout time.Duration
}

func FromEnv() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), HTTPAddr: envOr("HTTP_ADDR", ":8080"), WorkerCount: intEnv("WORKER_COUNT", 2), RetryLimit: intEnv("RETRY_LIMIT", 5), ShutdownTimeout: time.Duration(intEnv("SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if c.WorkerCount < 1 || c.RetryLimit < 1 || c.ShutdownTimeout <= 0 {
		return Config{}, errors.New("invalid worker configuration")
	}
	return c, nil
}
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value == 0 {
		return fallback
	}
	return value
}
