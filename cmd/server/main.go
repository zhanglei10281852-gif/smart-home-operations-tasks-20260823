package main

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/config"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/httpapi"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database open failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()
	if err = db.Migrate(ctx, database); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	r := repo.New(database)
	clock := model.RealClock{}
	authn := service.NewAuth(r, clock)
	households := service.NewHouseholds(r)
	devices := service.NewDevices(r, clock)
	telemetry := service.NewTelemetry(r, clock)
	energy := service.NewEnergy(r, clock)
	automation := service.NewAutomation(r, clock)
	reports := service.NewReport(r)
	server := httpapi.NewServer(households, authn, devices, telemetry, energy, automation, reports, logger)
	runner := worker.New(r, automation, logger, cfg.WorkerCount)
	go runner.Run(ctx)
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	logger.Info("smart home listening", "addr", cfg.HTTPAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
