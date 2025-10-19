package models

import "time"

type Trace struct {
	TraceID    string        `json:"traceId"`
	Spans      []ParsedSpan  `json:"spans"`      // All spans, ordered by start time
	RootSpan   *ParsedSpan   `json:"rootSpan"`   // Entry point to the trace
	Children   SpanTree      `json:"children"`   // Hierarchical representation
	StartTime  time.Time     `json:"startTime"`  // Earliest span start
	EndTime    time.Time     `json:"endTime"`    // Latest span end
	Duration   time.Duration `json:"duration"`   // Total trace time
	Services   []string      `json:"services"`   // All services involved
	Status     SpanStatus    `json:"status"`     // "ok", "error", "timeout"
	SpanCount  int           `json:"spanCount"`  // Number of spans
	ErrorCount int           `json:"errorCount"` // Number of error spans
}

// handle hierarchical representation of spans
type SpanTree struct {
	Span     *ParsedSpan
	Children []*SpanTree
}

type IncompleteTrace struct {
	TraceID      string                   `json:""`
	Spans        map[string](*ParsedSpan) `json:""`
	RootSpan     *ParsedSpan              `json:""`
	LastActivity time.Time                `json:""`
	StartTime    time.Time                `json:""`
	startTime    time.Time                `json:""`
	Services     map[string]bool          `json:""`
}

type Span struct {
	traceId      string     `json:""`
	spanId       string     `json:""`
	parentSpanId string     `json:""`
	startTime    string     `json:""`
	endTime      string     `json:""`
	status       string     `json:""`
	attributes   Attributes `json:""`
}

type Attributes struct {
	method string `json:""`
	userId string `json:""`
}

type ParsedSpan struct {
	TraceID       string            `json:""`
	SpanID        string            `json:""`
	ParentSpanID  string            `json:""`
	ServiceName   string            `json:""`
	OperationName string            `json:""`
	StartTime     time.Time         `json:""`
	EndTime       time.Time         `json:""`
	Duration      time.Duration     `json:""`
	Status        SpanStatus        `json:""`
	Tags          map[string]string `json:""`
	Kind          SpanKind          `json:""`
}

type SpanStatus struct {
	Code    string `json:""`
	Message string `json:""`
}

const (
	// SpanKindInternal indicates that the span represents an internal operation within an application.
	// This is the default span kind if not specified.
	SpanKindInternal SpanKind = iota

	// SpanKindServer indicates that the span covers server-side handling of a synchronous RPC or
	// other remote request. The parent is often a remote Client span.
	SpanKindServer

	// SpanKindClient indicates that the span describes a request to some remote service.
	// The child is usually a Server span.
	SpanKindClient

	// SpanKindProducer indicates that the span describes the initiator of an asynchronous request.
	// The child is always a Consumer span.
	SpanKindProducer

	// SpanKindConsumer indicates that the span describes a child of an asynchronous Producer request.
	// The parent is always a Producer span.
	SpanKindConsumer
)

// ParseSpanKind converts a string representation to a SpanKind.
func ParseSpanKind(s string) SpanKind {
	switch s {
	case "SPAN_KIND_SERVER", "SERVER":
		return SpanKindServer
	case "SPAN_KIND_CLIENT", "CLIENT":
		return SpanKindClient
	case "SPAN_KIND_PRODUCER", "PRODUCER":
		return SpanKindProducer
	case "SPAN_KIND_CONSUMER", "CONSUMER":
		return SpanKindConsumer
	case "SPAN_KIND_INTERNAL", "INTERNAl":
		return SpanKindInternal
	default:
		return SpanKindInternal
	}
}
