# Distrace

A lightweight distributed tracing backend that consumes OpenTelemetry spans from Kafka and intelligently correlates them into complete, hierarchical traces. Built as a learning project to explore distributed systems, OpenTelemetry Protocol (OTLP), and event-driven architectures.

## Overview

Distrace solves the fundamental challenge of distributed tracing: **reconstructing request flows across multiple services**. When a request travels through a microservices architecture, each service generates individual spans. Distrace's correlation engine assembles these scattered spans into coherent traces by:

- Consuming spans from Kafka topics in real-time
- Grouping spans by TraceID and resolving parent-child relationships
- Building hierarchical trace representations
- Managing trace lifecycle with intelligent timeout strategies

## Architecture

```
┌──────────────────────┐
│  Kafka Topic         │  OTLP ExportTraceServiceRequest (protobuf)
│  (OpenTelemetry)     │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│  Kafka Consumer      │  Deserializes protobuf, extracts spans
│  (kafka-go)          │
└──────────┬───────────┘
           │ spanCh
           ▼
┌──────────────────────┐
│ Correlation Engine   │  Groups spans by TraceID, builds hierarchies
│                      │  • 30s inactivity timeout
│                      │  • 5min maximum age limit
└──────────┬───────────┘
           │ traceCh
           ▼
┌──────────────────────┐
│  Complete Traces     │  Hierarchical trace with metadata
│                      │
└──────────────────────┘
```

### Component Pipeline

1. **Kafka Consumer** (`internal/kafka/consumer.go`)
   - Reads OTLP `ExportTraceServiceRequest` messages
   - Deserializes protobuf format
   - Extracts `service.name` from Resource attributes
   - Parses each span and sends to correlation engine via channel

2. **Correlation Engine** (`internal/correlation/correlation.go`)
   - Maintains `activeTraces` map keyed by TraceID
   - Processes incoming spans in real-time
   - Detects root spans (ParentSpanID == "")
   - Builds parent-child relationships
   - Completes traces based on inactivity or age

3. **Trace Output**
   - Completed traces emitted on output channel
   - Currently logs to stdout (extensible for storage/API)

## Technical Deep Dive: The Correlation Engine

The correlation engine is the heart of Distrace. It solves the challenge of assembling distributed spans that arrive **out-of-order** and from **multiple services** into coherent traces.

### Core Algorithm

**Data Structure:**
```go
type CorrelationEngine struct {
    spanCh       <-chan models.ParsedSpan          // Incoming spans from Kafka
    traceCh      chan<- models.Trace               // Outgoing complete traces
    activeTraces map[string]*models.IncompleteTrace // Key: TraceID
}

type IncompleteTrace struct {
    TraceID      string
    Spans        map[string]*ParsedSpan // Key: SpanID for O(1) lookup
    RootSpan     *ParsedSpan            // Entry point (ParentSpanID == "")
    LastActivity time.Time              // Last span received
    StartTime    time.Time              // First span timestamp
}
```

**Processing Flow:**

1. **Span Reception** (`correlation.go:91-126`)
   ```go
   func (ce *CorrelationEngine) processSpan(span models.ParsedSpan) {
       // Check if trace exists
       if activeTrace, exists := ce.activeTraces[span.TraceID]; exists {
           activeTrace.Spans[span.SpanID] = &span
           activeTrace.LastActivity = time.Now()

           // Mark root span if ParentSpanID is empty
           if span.ParentSpanID == "" {
               activeTrace.RootSpan = &span
           }
       } else {
           // Create new IncompleteTrace for this TraceID
           ce.activeTraces[span.TraceID] = newTrace(span)
       }
   }
   ```

2. **Trace Completion Strategies**

   **Strategy 1: Inactivity-Based Completion** (`correlation.go:69-88`)
   - Traces with a root span that haven't received new spans for 30 seconds are completed
   - Rationale: Most traces complete within milliseconds; 30s indicates all spans have arrived

   **Strategy 2: Maximum Age Limit**
   - Traces older than 5 minutes are force-completed even without a root span
   - Prevents memory leaks from orphaned or malformed traces

   ```go
   func (ce *CorrelationEngine) cleanupStalledTraces() {
       for traceID, trace := range ce.activeTraces {
           timeSinceActivity := time.Since(trace.LastActivity)
           age := time.Since(trace.StartTime)

           // Complete if inactive AND has root span
           if trace.RootSpan != nil && timeSinceActivity > 30*time.Second {
               ce.completeTrace(traceID)
           }
           // Force complete very old traces
           if age > 5*time.Minute {
               ce.completeTrace(traceID)
           }
       }
   }
   ```

3. **Hierarchy Construction** (`correlation.go:154-188`)

   Builds parent-child relationships using ParentSpanID references:

   ```go
   func (ce *CorrelationEngine) buildHierarchy(spans map[string]*ParsedSpan) *models.Trace {
       var rootSpan *ParsedSpan
       childrenMap := make(map[string][]*models.ParsedSpan)

       // First pass: identify root and build children map
       for _, span := range spans {
           if span.ParentSpanID == "" {
               rootSpan = span
           } else {
               childrenMap[span.ParentSpanID] = append(
                   childrenMap[span.ParentSpanID], span)
           }
       }

       // Recursively build SpanTree from root
       return buildSpanTree(rootSpan, childrenMap)
   }
   ```

4. **Status Determination** (`correlation.go:229-240`)

   Uses error propagation: if ANY span in a trace has an error status, the entire trace is marked as ERROR:

   ```go
   func determineOverallStatus(spans map[string]*ParsedSpan) SpanStatus {
       for _, span := range spans {
           if span.Status.Code == "ERROR" {
               return span.Status  // Propagate error to trace level
           }
       }
       return span.Status  // All OK
   }
   ```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **TraceID-based grouping** | OpenTelemetry standard for correlating distributed spans |
| **Map-based span storage** | O(1) lookup when adding spans, supports out-of-order arrival |
| **Root span detection** | Establishes entry point and hierarchy anchor for trace visualization |
| **Dual completion strategy** | Balances trace completeness (inactivity) with memory management (max age) |
| **Channel-based architecture** | Decouples Kafka consumption from correlation logic, enables backpressure |
| **Periodic cleanup ticker** | Runs every 30s to avoid blocking main event loop |

### Handling Edge Cases

- **Out-of-order arrival**: Spans stored in map; root span set whenever it arrives
- **Missing root spans**: Traces without roots are logged and skipped after max age timeout
- **Slow spans**: 30s inactivity threshold accommodates slow database queries or external API calls
- **Channel backpressure**: Consumer blocks if correlation engine can't keep up

## Data Model

**ParsedSpan** (`internal/models/traces.go:34-46`)
```go
type ParsedSpan struct {
    TraceID       string            // Links to distributed trace
    SpanID        string            // Unique span identifier
    ParentSpanID  string            // Empty for root spans
    ServiceName   string            // Service that created this span
    OperationName string            // e.g., "GET /api/users"
    StartTime     time.Time
    EndTime       time.Time
    Duration      time.Duration
    Status        SpanStatus        // ok, error, unset
    Tags          map[string]string // OpenTelemetry attributes
    Kind          SpanKind          // SERVER, CLIENT, PRODUCER, CONSUMER, INTERNAL
}
```

**Trace** (`internal/models/traces.go:5-17`)
```go
type Trace struct {
    TraceID    string
    Spans      []ParsedSpan  // All spans in trace
    RootSpan   *ParsedSpan   // Entry point
    Children   SpanTree      // Hierarchical representation
    StartTime  time.Time     // Earliest span start
    EndTime    time.Time     // Latest span end
    Duration   time.Duration
    Services   []string      // All services involved
    Status     SpanStatus    // Propagated from spans
}
```

## Getting Started

### Prerequisites

- Go 1.21+
- Kafka cluster (tested with kafka-go)
- OpenTelemetry-instrumented applications exporting to Kafka

### Configuration

Set the following environment variables:

```bash
KAFKA_BROKERS=localhost:9093    # Comma-separated broker addresses
KAFKA_TOPIC=traces              # Topic containing OTLP spans
KAFKA_GROUP_ID=trace-monitor    # Consumer group ID
```

### Running

```bash
go run cmd/server/main.go
```

### Testing with OpenTelemetry Demo

The [OpenTelemetry Demo](https://opentelemetry.io/docs/demo/) application provides a complete microservices environment with built-in trace generation:

1. Deploy the demo with Kafka exporter enabled
2. Configure the OTEL Collector to export to your Kafka topic
3. Run Distrace to consume and correlate spans

This setup generates realistic traces across multiple services (frontend, checkout, payment, shipping, etc.), making it ideal for testing correlation logic.

## Project Learnings

This project was built to explore:

- **Distributed Tracing Concepts**: Understanding TraceID/SpanID propagation, parent-child relationships, and trace context
- **OpenTelemetry Protocol (OTLP)**: Working with protobuf serialization, ResourceSpans, ScopeSpans, and span attributes
- **Event-Driven Architecture**: Kafka consumer patterns, channel-based concurrency, backpressure handling
- **Correlation Algorithms**: Building hierarchies from unordered data, handling missing/late spans
- **Go Concurrency Patterns**: Goroutines, channels, select statements, context cancellation

## Future Enhancements

- [ ] Web UI for trace visualization
- [ ] Storage backend (PostgreSQL, ClickHouse)
- [ ] Trace search/filtering by service, operation, tags
- [ ] Metrics on trace duration, error rates
- [ ] Sampling strategies for high-volume environments
- [ ] gRPC API for querying traces

## Dependencies

- [kafka-go](https://github.com/segmentio/kafka-go) - Pure Go Kafka client
- [OpenTelemetry Proto](https://github.com/open-telemetry/opentelemetry-proto) - OTLP protobuf definitions

## License

MIT
