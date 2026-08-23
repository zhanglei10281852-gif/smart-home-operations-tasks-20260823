package config

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

func ValidateDatabaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("database URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" {
		return errors.New("DATABASE_URL must use postgres scheme")
	}
	if u.Host == "" {
		return errors.New("DATABASE_URL host is missing")
	}
	return nil
}
func ValidateHTTPAddr(addr string) error {
	return validateListenAddr(strings.TrimSpace(addr))
}
func ValidateShutdown(timeout time.Duration) error {
	if timeout < time.Second || timeout > 5*time.Minute {
		return errors.New("shutdown timeout outside supported range")
	}
	return nil
}
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid>"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "redacted")
	}
	return u.String()
}
