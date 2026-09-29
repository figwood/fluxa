package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"fluxa-api/internal/bootstrap"
	"fluxa-api/internal/config"
	httpapi "fluxa-api/internal/interfaces/http"

	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := config.Load()
	container, err := bootstrap.New(cfg, log)
	if err != nil {
		log.Fatal("bootstrap failed", zap.Error(err))
	}
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: httpapi.NewRouter(container),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Info("fluxa api started", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("api failed", zap.Error(err))
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("api shutdown failed", zap.Error(err))
	}
}
