package main

import (
	"context"
	"os/signal"
	"syscall"

	"fluxa-api/internal/bootstrap"
	"fluxa-api/internal/config"

	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	container, err := bootstrap.New(config.Load(), log)
	if err != nil {
		log.Fatal("bootstrap failed", zap.Error(err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Info("fluxa worker started")
	if err := container.Worker.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal("worker failed", zap.Error(err))
	}
}
