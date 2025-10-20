# Distrace

Lightweight backend tool for pulling in spans from Kafka, correlating them into one single trace for use for debugging, or getting an understanding of your system dependencies.


## Getting Started

Run it with
`go run cmd/server/main.go`

The following envvars inform the kafka config, this uses [kafka-go](https://github.com/segmentio/kafka-go) as a dependency

```
KAFKA_BROKERS
KAFKA_TOPIC
KAFKA_GROUP_ID
```

I used the opentelemetry demo application to generate traces, they export their spans using the opentelemetry collector pattern.
It made generating useful data to get going quite easy.

https://opentelemetry.io/docs/demo/
