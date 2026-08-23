package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL      string
	HTTPAddr         string
	WorkerCount      int
	RetryLimit       int
	OutboxWebhookURL string
	ShutdownTimeout  time.Duration
}

func FromEnv() (Config, error) {
	workerCount, err := boundedIntEnv("WORKER_COUNT", 2, 1, 64)
	if err != nil {
		return Config{}, err
	}
	retryLimit, err := boundedIntEnv("RETRY_LIMIT", 5, 1, 100)
	if err != nil {
		return Config{}, err
	}
	shutdownSeconds, err := boundedIntEnv("SHUTDOWN_TIMEOUT_SECONDS", 15, 1, 300)
	if err != nil {
		return Config{}, err
	}
	c := Config{
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		HTTPAddr:         strings.TrimSpace(envOr("HTTP_ADDR", ":8080")),
		WorkerCount:      workerCount,
		RetryLimit:       retryLimit,
		OutboxWebhookURL: strings.TrimSpace(os.Getenv("OUTBOX_WEBHOOK_URL")),
		ShutdownTimeout:  time.Duration(shutdownSeconds) * time.Second,
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if err := ValidateDatabaseURL(c.DatabaseURL); err != nil {
		return Config{}, err
	}
	if err := validateListenAddr(c.HTTPAddr); err != nil {
		return Config{}, err
	}
	if c.OutboxWebhookURL != "" {
		parsed, err := url.Parse(c.OutboxWebhookURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return Config{}, errors.New("OUTBOX_WEBHOOK_URL must be an absolute HTTPS URL")
		}
	}
	return c, nil
}
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func boundedIntEnv(key string, fallback, minimum, maximum int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func validateListenAddr(raw string) error {
	_, port, err := net.SplitHostPort(raw)
	if err != nil {
		return errors.New("HTTP_ADDR must be a host:port address")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return errors.New("HTTP_ADDR port must be between 1 and 65535")
	}
	return nil
}
