package config

import (
	"os"
)

type Config struct {
	Kafka KafkaConfig
}

type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

func Load() *Config {
	Brokers := getEnv("KAFKA_BROKERS", "localhost:9093")
	Topic := getEnv("KAFKA_TOPIC", "traces")
	GroupID := getEnv("KAFKA_GROUP_ID", "trace-monitor")

	return &Config{
		Kafka: KafkaConfig{
			Brokers: []string{Brokers},
			Topic:   Topic,
			GroupID: GroupID,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
