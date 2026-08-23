package config

import (
	"testing"
	"time"
)

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
