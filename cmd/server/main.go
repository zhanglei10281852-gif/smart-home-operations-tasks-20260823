package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/config"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/httpapi"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/transport"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type webhookPublisher struct{ webhook transport.Webhook }

func (p webhookPublisher) Publish(ctx context.Context, message model.OutboxMessage) error {
	return p.webhook.SendWithKey(ctx, message, fmt.Sprintf("outbox-%d", message.ID))
}

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
	server.Scope = r
	server.Readiness = func(ctx context.Context) error {
		_, err := database.Health(ctx, time.Now)
		return err
	}
	runner := worker.New(r, automation, logger, cfg.WorkerCount)
	go func() {
		if err := runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("automation worker stopped", "error", err)
			cancel()
		}
	}()
	maintenance := worker.Scheduler{Maintenance: service.NewMaintenance(r, time.Now), Logger: logger, Interval: time.Minute}
	go func() {
		if err := maintenance.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("maintenance worker stopped", "error", err)
			cancel()
		}
	}()
	if cfg.OutboxWebhookURL != "" {
		client := transport.NewClient(cfg.OutboxWebhookURL)
		outbox := &worker.OutboxRunner{Repo: r, Publisher: webhookPublisher{webhook: transport.Webhook{Client: client, URL: cfg.OutboxWebhookURL}}, Logger: logger, RetryLimit: cfg.RetryLimit, PollInterval: 100 * time.Millisecond}
		go func() {
			if err := outbox.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("outbox worker stopped", "error", err)
				cancel()
			}
		}()
	} else {
		logger.Info("outbox delivery disabled; messages will remain durable", "config", "OUTBOX_WEBHOOK_URL")
	}
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
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
