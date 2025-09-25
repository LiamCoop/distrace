package models

import (
    otlpcollectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
    otlpcommon "go.opentelemetry.io/proto/otlp/common/v1"
    otlpresource "go.opentelemetry.io/proto/otlp/resource/v1"
    otlptrace "go.opentelemetry.io/proto/otlp/trace/v1"
)

// The main message type from Kafka
type ExportTraceServiceRequest = otlpcollectortrace.ExportTraceServiceRequest

// ResourceSpans contains spans from a single resource
type ResourceSpans = otlptrace.ResourceSpans

// Resource describes the entity producing telemetry
type Resource = otlpresource.Resource

// ScopeSpans contains spans from a single instrumentation scope
type ScopeSpans = otlptrace.ScopeSpans

// InstrumentationScope describes the instrumentation library
type InstrumentationScope = otlpcommon.InstrumentationScope

// The actual span data
type ProtoSpan = otlptrace.Span

// Span status information
type Status = otlptrace.Status

// Status codes enum
type StatusCode = otlptrace.Status_StatusCode

// Key-value attributes
type KeyValue = otlpcommon.KeyValue

// Attribute values (can be string, int, bool, etc.)
type AnyValue = otlpcommon.AnyValue

// Span events (logs within a span)
type Event = otlptrace.Span_Event

// Span kinds (CLIENT, SERVER, INTERNAL, etc.)
type SpanKind = otlptrace.Span_SpanKind

