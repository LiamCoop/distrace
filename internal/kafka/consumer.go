package kafka

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	otlpcollectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	otlpcommon "go.opentelemetry.io/proto/otlp/common/v1"
	otlpresource "go.opentelemetry.io/proto/otlp/resource/v1"
	otlptrace "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/liamcoop/distrace/internal/config"
	"github.com/liamcoop/distrace/internal/models"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
	config config.KafkaConfig
	spanCh chan<- models.ParsedSpan	
}

func NewConsumer(cfg config.KafkaConfig, spanChan chan models.ParsedSpan) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Brokers,
		Topic: cfg.Topic,
		GroupID: cfg.GroupID,
		StartOffset: kafka.LastOffset,
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait: 1 * time.Second,
		Logger: kafka.LoggerFunc(log.Printf),
		ErrorLogger: kafka.LoggerFunc(log.Printf),
	})

	return &Consumer{
		reader: r,
		config: cfg,
		spanCh: spanChan,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	defer c.reader.Close()

	log.Printf("Starting kafka consumer for topic: %s", c.config.Topic)

	for {
		select {
		case <-ctx.Done():
			log.Println("Consumer context cancelled, shutting down")
			return ctx.Err()

		default:
			// read
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Error reading message: %v", err)
				continue
			}

			if err := c.processMessage(msg); err != nil {
				log.Printf("Error processing message: %v", err)
			}
		}
	}
}

func (c *Consumer) processMessage(message kafka.Message) error {
	var traceData otlpcollectortrace.ExportTraceServiceRequest	

	if err := proto.Unmarshal(message.Value, &traceData); err != nil {
		return fmt.Errorf("failed to parse kafka message")
	}

	// spans are batched from a single service to incur less overhead
	// turn our batched spans from kafka into multiple span objects, need to be correlated to a trace individually
	for _, resourceSpan := range traceData.ResourceSpans {
		serviceName := extractServicename(resourceSpan.Resource)

		// spans may be batched from a single service
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				parsedSpan := c.parseSpan(span, serviceName)
				c.spanCh <- *parsedSpan
			}
		}
	}

	return nil
}

func extractServicename(rs *otlpresource.Resource) string {
	if rs == nil {
		return "unknown"
	}

	for _, attr := range rs.Attributes {
		if (attr.Key == "service.name" && attr.Value.GetStringValue() != "") {
			return attr.Value.GetStringValue()
		}
	}
	return "unknown"
}

func (c *Consumer) parseSpan(span *otlptrace.Span, serviceName string) *models.ParsedSpan {
    // Convert binary IDs to hex strings
    traceID := hex.EncodeToString(span.TraceId)
    spanID := hex.EncodeToString(span.SpanId)
    parentSpanID := ""
    if len(span.ParentSpanId) > 0 {
        parentSpanID = hex.EncodeToString(span.ParentSpanId)
    }
    
    // Convert nanosecond timestamps
    startTime := time.Unix(0, int64(span.StartTimeUnixNano))
    endTime := time.Unix(0, int64(span.EndTimeUnixNano))
    
    // Extract status
    status := "ok"
    statusMessage := ""
    if span.Status != nil {
        switch span.Status.Code {
        case otlptrace.Status_STATUS_CODE_ERROR:
            status = "error"
        case otlptrace.Status_STATUS_CODE_OK:
            status = "ok"
        default:
            status = "unset"
        }
        statusMessage = span.Status.Message
    }
    
    // Extract attributes as tags
    tags := make(map[string]string)
    for _, attr := range span.Attributes {
        key := attr.Key
        value := getAttributeValue(attr.Value)
        tags[key] = value
    }
    
    return &models.ParsedSpan{
        TraceID:       traceID,
        SpanID:        spanID,
        ParentSpanID:  parentSpanID,
        ServiceName:   serviceName,
        OperationName: span.Name,
        StartTime:     startTime,
        EndTime:       endTime,
        Duration:      endTime.Sub(startTime),
        Status: models.SpanStatus{
            Code:    status,
            Message: statusMessage,
        },
        Tags: tags,
        Kind: span.Kind.String(),
    }
}

func getAttributeValue(value *otlpcommon.AnyValue) string {
    if value == nil {
        return ""
    }
    
    switch v := value.Value.(type) {
    case *otlpcommon.AnyValue_StringValue:
        return v.StringValue
    case *otlpcommon.AnyValue_IntValue:
        return fmt.Sprintf("%d", v.IntValue)
    case *otlpcommon.AnyValue_DoubleValue:
        return fmt.Sprintf("%.2f", v.DoubleValue)
    case *otlpcommon.AnyValue_BoolValue:
        return fmt.Sprintf("%t", v.BoolValue)
    case *otlpcommon.AnyValue_BytesValue:
        return hex.EncodeToString(v.BytesValue)
    default:
        return ""
    }
}
