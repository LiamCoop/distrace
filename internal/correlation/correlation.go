package correlation

// SpanKind represents the role of a span in a trace.
// It provides hints to the tracing backend about how traces should be assembled.
type SpanKind int

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

// String returns the string representation of the SpanKind.
func (sk SpanKind) String() string {
	switch sk {
	case SpanKindServer:
		return "SERVER"
	case SpanKindClient:
		return "CLIENT"
	case SpanKindProducer:
		return "PRODUCER"
	case SpanKindConsumer:
		return "CONSUMER"
	case SpanKindInternal:
		return "INTERNAL"
	default:
		return "INTERNAL"
	}
}

// ParseSpanKind converts a string representation to a SpanKind.
func ParseSpanKind(s string) SpanKind {
	switch s {
	case "SPAN_KIND_SERVER", "Server":
		return SpanKindServer
	case "SPAN_KIND_CLIENT", "Client":
		return SpanKindClient
	case "SPAN_KIND_PRODUCER", "Producer":
		return SpanKindProducer
	case "SPAN_KIND_CONSUMER", "Consumer":
		return SpanKindConsumer
	case "SPAN_KIND_INTERNAL", "Internal":
		return SpanKindInternal
	default:
		return SpanKindInternal
	}
}



