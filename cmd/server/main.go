package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/liamcoop/distrace/internal/config"
	"github.com/liamcoop/distrace/internal/correlation"
	"github.com/liamcoop/distrace/internal/kafka"
	"github.com/liamcoop/distrace/internal/logging"
	"github.com/liamcoop/distrace/internal/models"
)

func main() {
	cfg := config.Load()

	logger := logging.Init()
	ctx, cancel := context.WithCancel(context.Background())

	spanCh := make(chan models.ParsedSpan)
	traceCh := make(chan models.Trace)

	consumer := kafka.NewConsumer(cfg.Kafka, logger, spanCh)
	correlator := correlation.NewCorrelationEngine(logger, spanCh, traceCh)

	go func() {
		if err := consumer.Start(ctx); err != nil {
			logger.Error("Consumer error", slog.String("error", err.Error()))
		}
	}()

	go func() {
		if err := correlator.Start(ctx); err != nil {
			logger.Error("Correlator error", slog.String("error", err.Error()))
		}
	}()

	go func() {
		for {
			trace, ok := <-traceCh
			if !ok {
				logger.Info("Trace channel closed, stopping trace monitor")
				return
			}
			logger.Info("trace output",
				slog.Any("trace", trace),
			)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Trace monitor started")
	<-sigChan

	logger.Info("Shutting down...")
	cancel()
}
