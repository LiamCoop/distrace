package models

import "time"

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
	Kind          string            `json:""`
}

type SpanStatus struct {
	Code    string `json:""`
	Message string `json:""`
}
