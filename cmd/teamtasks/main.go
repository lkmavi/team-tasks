package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/lkmavi/team-tasks/internal/app"
	"github.com/lkmavi/team-tasks/internal/config"
	"github.com/lkmavi/team-tasks/pkg/logger"
)

func main() {
	cfgPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg := config.MustLoad(*cfgPath)
	log := logger.New(cfg.Env)

	a, err := app.New(cfg, log)
	if err != nil {
		log.Error("failed to initialize app", "err", err)
		os.Exit(1)
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server starting", "addr", a.Addr())
		if err := a.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			select {
			case serverErr <- err:
			default:
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("signal received, shutting down", "signal", sig)
	case err := <-serverErr:
		log.Error("server error", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)

	if err := a.Shutdown(ctx); err != nil {
		cancel()
		log.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	cancel()

	log.Info("server stopped")
}
