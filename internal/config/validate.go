package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Runtime struct {
	DatabaseURL string
	HTTPAddr    string
	Shutdown    time.Duration
	WorkerCount int
	LogLevel    string
}

func LoadRuntime(getenv func(string) string) (Runtime, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	r := Runtime{DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")), HTTPAddr: strings.TrimSpace(getenv("HTTP_ADDR")), Shutdown: 10 * time.Second, WorkerCount: 2, LogLevel: "info"}
	if r.DatabaseURL == "" {
		return Runtime{}, errors.New("DATABASE_URL is required")
	}
	if parsed, err := url.Parse(r.DatabaseURL); err != nil || parsed.Scheme != "postgres" || parsed.Host == "" {
		return Runtime{}, errors.New("DATABASE_URL must be a postgres URL")
	}
	if r.HTTPAddr == "" {
		r.HTTPAddr = ":8080"
	}
	if err := validateListenAddr(r.HTTPAddr); err != nil {
		return Runtime{}, err
	}
	if raw := strings.TrimSpace(getenv("WORKER_COUNT")); raw != "" {
		count, err := strconv.Atoi(raw)
		if err != nil || count < 1 || count > 64 {
			return Runtime{}, errors.New("WORKER_COUNT must be between 1 and 64")
		}
		r.WorkerCount = count
	}
	if raw := strings.TrimSpace(getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		duration, err := time.ParseDuration(raw)
		if err != nil || duration <= 0 || duration > 5*time.Minute {
			return Runtime{}, errors.New("SHUTDOWN_TIMEOUT is invalid")
		}
		r.Shutdown = duration
	}
	if level := strings.ToLower(strings.TrimSpace(getenv("LOG_LEVEL"))); level != "" {
		if level != "debug" && level != "info" && level != "warn" && level != "error" {
			return Runtime{}, errors.New("LOG_LEVEL is invalid")
		}
		r.LogLevel = level
	}
	return r, nil
}

func (r Runtime) Validate() error {
	if strings.TrimSpace(r.DatabaseURL) == "" || strings.TrimSpace(r.HTTPAddr) == "" || r.Shutdown <= 0 || r.WorkerCount <= 0 || r.LogLevel == "" {
		return errors.New("runtime configuration is incomplete")
	}
	if err := ValidateDatabaseURL(r.DatabaseURL); err != nil {
		return err
	}
	if err := validateListenAddr(r.HTTPAddr); err != nil {
		return err
	}
	return nil
}
