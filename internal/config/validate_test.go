package config

import (
	"strings"
	"testing"
	"time"
)

func TestFromEnvAcceptsValidRuntimeConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost/smart_home?sslmode=disable")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9080")
	t.Setenv("WORKER_COUNT", "4")
	t.Setenv("RETRY_LIMIT", "7")
	t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "20")
	t.Setenv("OUTBOX_WEBHOOK_URL", "https://hooks.example.test/smart-home")

	got, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTPAddr != "127.0.0.1:9080" || got.WorkerCount != 4 || got.RetryLimit != 7 || got.ShutdownTimeout != 20*time.Second {
		t.Fatalf("config=%+v", got)
	}
}

func TestFromEnvRejectsExplicitInvalidValues(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL":             "postgres://localhost/smart_home",
		"HTTP_ADDR":                ":8080",
		"WORKER_COUNT":             "2",
		"RETRY_LIMIT":              "5",
		"SHUTDOWN_TIMEOUT_SECONDS": "15",
		"OUTBOX_WEBHOOK_URL":       "",
	}
	cases := []struct {
		key, value, want string
	}{
		{"WORKER_COUNT", "0", "WORKER_COUNT"},
		{"RETRY_LIMIT", "not-a-number", "RETRY_LIMIT"},
		{"SHUTDOWN_TIMEOUT_SECONDS", "301", "SHUTDOWN_TIMEOUT_SECONDS"},
		{"HTTP_ADDR", "8080", "HTTP_ADDR"},
		{"OUTBOX_WEBHOOK_URL", "http://hooks.example.test", "OUTBOX_WEBHOOK_URL"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			for key, value := range base {
				t.Setenv(key, value)
			}
			t.Setenv(tc.key, tc.value)
			_, err := FromEnv()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLoadRuntimeDefaultsAndOverrides(t *testing.T) {
	values := map[string]string{"DATABASE_URL": "postgres://user:pass@localhost/db?sslmode=disable"}
	r, err := LoadRuntime(func(key string) string { return values[key] })
	if err != nil || r.HTTPAddr != ":8080" || r.WorkerCount != 2 || r.Shutdown <= 0 {
		t.Fatalf("runtime=%+v err=%v", r, err)
	}
	values["WORKER_COUNT"] = "4"
	values["SHUTDOWN_TIMEOUT"] = "30s"
	values["LOG_LEVEL"] = "debug"
	r, err = LoadRuntime(func(key string) string { return values[key] })
	if err != nil || r.WorkerCount != 4 || r.Shutdown.String() != "30s" || r.LogLevel != "debug" {
		t.Fatalf("runtime=%+v err=%v", r, err)
	}
}

func TestLoadRuntimeRejectsInvalidConfiguration(t *testing.T) {
	for _, values := range []map[string]string{{}, {"DATABASE_URL": "http://localhost/db"}, {"DATABASE_URL": "postgres://localhost/db", "WORKER_COUNT": "0"}, {"DATABASE_URL": "postgres://localhost/db", "LOG_LEVEL": "trace"}} {
		if _, err := LoadRuntime(func(key string) string { return values[key] }); err == nil {
			t.Fatalf("accepted values=%v", values)
		}
	}
}

func TestRuntimeValidate(t *testing.T) {
	valid := Runtime{DatabaseURL: "postgres://localhost/db", HTTPAddr: ":8080", Shutdown: time.Second, WorkerCount: 1, LogLevel: "info"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, runtime := range []Runtime{{}, {DatabaseURL: "postgres://localhost/db", HTTPAddr: ":8080"}, {DatabaseURL: "postgres://localhost/db", HTTPAddr: ":8080", Shutdown: time.Second, WorkerCount: 0, LogLevel: "info"}} {
		if err := runtime.Validate(); err == nil {
			t.Fatalf("invalid runtime accepted: %+v", runtime)
		}
	}
}
