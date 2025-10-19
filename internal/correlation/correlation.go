package correlation

import (
	"time"

	"github.com/liamcoop/distrace/internal/models"
)

// pickup spans by span channel, defined in main
// engine should process spans,
// we should accept them, put them onto our list of Traces.
// when traces are ready we should send them into a traces channel
// something else can pickup the traces and deal with them from there

type Correlator struct {
	spanChan        <-chan models.ParsedSpan // incoming spans
	completedTraces chan<- models.Trace      // outgoing traces

	activeTraces map[string]*models.IncompleteTrace

	Services map[string]bool
}

func New(spanChan <-chan models.ParsedSpan, traceChan chan<- models.Trace) Correlator {
	c := &Correlator{
		spanChan:        spanChan,
		completedTraces: traceChan,
	}

	return *c
}

func (c *Correlator) processSpan(span models.ParsedSpan) {
	activeTrace, exists := c.activeTraces[span.TraceID]

	if exists {
		activeTrace.Spans[span.SpanID] = &span
		activeTrace.Services[span.ServiceName] = true

		if span.ParentSpanID == "" {
			activeTrace.RootSpan = &span
		}
	} else {
		spans := make(map[string]*models.ParsedSpan)
		spans[span.SpanID] = &span

		services := make(map[string]bool)
		services[span.ServiceName] = true

		var rootSpan *models.ParsedSpan
		if span.ParentSpanID == "" {
			rootSpan = &span
		}

		it := &models.IncompleteTrace{
			TraceID:      span.TraceID,
			Spans:        spans,
			Services:     services,
			RootSpan:     rootSpan,
			LastActivity: time.Now(),
			StartTime:    span.StartTime,
		}

		c.activeTraces[span.TraceID] = it
	}
}

func (c *Correlator) buildHierarchy(spans map[string]*models.ParsedSpan) *models.Trace {

	var rootSpan *models.ParsedSpan
	children := make(map[string][]*models.ParsedSpan)

	for _, span := range spans {
		if span.ParentSpanID == "" {
			rootSpan = span
		} else {
			children[span.ParentSpanID] = append(children[span.ParentSpanID], span)
		}
	}

	return &models.Trace{
		TraceID:   rootSpan.TraceID,
		Spans:     flattenToSlice(spans),
		RootSpan:  rootSpan,
		StartTime: rootSpan.StartTime,
		EndTime:   rootSpan.EndTime,
		Duration:  rootSpan.EndTime.Sub(rootSpan.StartTime),
		Services:  extractServices(spans),
		Status:    determineOverallStatus(spans),
	}
}

func extractServices(spans map[string]*models.ParsedSpan) (services []string) {
	seen := make(map[string]bool)

	for _, s := range spans {
		if !seen[s.ServiceName] {
			seen[s.ServiceName] = true
			services = append(services, s.ServiceName)
		}
	}

	return services
}

func flattenToSlice(mp map[string]*models.ParsedSpan) (s []models.ParsedSpan) {
	for _, span := range mp {
		s = append(s, *span)
	}

	return s
}

func determineOverallStatus(mp map[string]*models.ParsedSpan) models.SpanStatus {
	var span *models.ParsedSpan
	for _, span = range mp {
		switch span.Status.Code {
		case "STATUS_CODE_ERROR", "ERROR":
			return span.Status
		}
	}

	return span.Status
}
