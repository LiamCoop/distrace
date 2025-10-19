package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liamcoop/distrace/internal/config"
	"github.com/liamcoop/distrace/internal/kafka"
	"github.com/liamcoop/distrace/internal/correlation"
	"github.com/liamcoop/distrace/internal/models"
)

func main() {
	cfg := config.Load()


	ctx, cancel := context.WithCancel(context.Background())

	spanC := make(chan models.ParsedSpan)
	c := kafka.NewConsumer(cfg.Kafka, spanC)


	go func() {
		if err := c.Start(ctx); err != nil {
			log.Printf("Consumer error: %v", err)
		}
	}()


	correlator := correlation.New(spanC)

	go func() {
		if err := correlator.Start(); err != nil {
			log.Printf("Correlator error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Trace monitor started")
	<-sigChan

	log.Println("Shutting down...")
	cancel()
}
