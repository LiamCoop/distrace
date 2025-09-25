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
	"github.com/liamcoop/distrace/internal/models"
)

func main() {
	cfg := config.Load()


	spanC := make(chan models.ParsedSpan)

	c := kafka.NewConsumer(cfg.Kafka, spanC)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := c.Start(ctx); err != nil {
			log.Printf("Consumer error: %v", err)
		}
	}()

	go func() {
		for span := range spanC {
			log.Printf("Received span: %+v\n", span)
			time.Sleep(2 * time.Second)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Trace monitor started")
	<-sigChan

	log.Println("Shutting down...")
	cancel()
}
