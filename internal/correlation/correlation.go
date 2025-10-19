package correlation

import (
	"context"
	"log/slog"
	"time"

	"github.com/liamcoop/distrace/internal/models"
)

// this package is responsible for picking up spans from the spans channel
// and turning them into traces, with a hierarchical representation of the spans in the order that they were called

type CorrelationEngine struct {
	logger  *slog.Logger
	spanCh  <-chan models.ParsedSpan // incoming spans
	traceCh chan<- models.Trace      // outgoing traces

	activeTraces map[string]*models.IncompleteTrace

	Services map[string]bool
}

func NewCorrelationEngine(logger *slog.Logger, spanCh <-chan models.ParsedSpan, traceCh chan<- models.Trace) CorrelationEngine {
	c := &CorrelationEngine{
		spanCh:       spanCh,
		traceCh:      traceCh,
		activeTraces: make(map[string]*models.IncompleteTrace),
		Services:     make(map[string]bool),
		logger:       logger,
	}

	c.logger.Info("correlation engine initialized")
	return *c
}

// run cleanup every 30 seconds
// TODO: use envvar for this?
func (ce *CorrelationEngine) Start(ctx context.Context) error {
	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case span, ok := <-ce.spanCh:
			if !ok {
				ce.flushAll()
				return nil
			}
			ce.processSpan(span)

		case <-cleanupTicker.C:
			ce.cleanupStalledTraces()

		case <-ctx.Done():
			ce.flushAll()
			return ctx.Err()
		}
	}
}

func (ce *CorrelationEngine) flushAll() {
	for traceID := range ce.activeTraces {
		ce.logger.Debug("flushing trace", slog.String("traceID", traceID))
		ce.completeTrace(traceID)
	}
}

func (ce *CorrelationEngine) cleanupStalledTraces() {
	now := time.Now()
	maxAge := 5 * time.Minute
	inactivityThreshold := 30 * time.Second

	for traceID, trace := range ce.activeTraces {
		age := now.Sub(trace.StartTime)
		timeSinceActivity := now.Sub(trace.LastActivity)

		// Complete if inactive for 30s AND has root span
		if trace.RootSpan != nil && timeSinceActivity > inactivityThreshold {
			ce.logger.Debug("completing inactive trace, inactive w/ root", slog.String("traceID", traceID))
			ce.completeTrace(traceID)
		} else if age > maxAge {
			// Force complete very old traces even without root
			ce.logger.Debug("force completing old trace, maxAge", slog.String("traceID", traceID))
			ce.completeTrace(traceID)
		}
	}
}

// allocates a span to an activeTrace, if none exists with that traceID, a new one is created
func (ce *CorrelationEngine) processSpan(span models.ParsedSpan) {
	ce.logger.Debug("processing span", slog.Any("span", span))

	activeTrace, exists := ce.activeTraces[span.TraceID]

	if exists {
		activeTrace.Spans[span.SpanID] = &span
		activeTrace.LastActivity = time.Now()

		if span.ParentSpanID == "" {
			activeTrace.RootSpan = &span
		}
	} else {
		spans := make(map[string]*models.ParsedSpan)
		spans[span.SpanID] = &span

		services := make(map[string]bool)
		services[span.ServiceName] = true

		var rootSpan *models.ParsedSpan
		rootSpan = nil
		if span.ParentSpanID == "" {
			rootSpan = &span
		}

		it := &models.IncompleteTrace{
			TraceID:      span.TraceID,
			Spans:        spans,
			RootSpan:     rootSpan,
			LastActivity: time.Now(),
			StartTime:    span.StartTime,
		}

		ce.activeTraces[span.TraceID] = it
	}
}

// accepts TID, should buildHierarchy, ship out on channel, delete from activeTraces
func (ce *CorrelationEngine) completeTrace(traceID string) {
	itrace := ce.activeTraces[traceID]
	if itrace == nil {
		return
	}

	t := ce.buildHierarchy(itrace.Spans)

	// If buildHierarchy returns nil (no root span), skip sending
	if t == nil {
		ce.logger.Debug("skipping trace without root span", slog.String("traceID", traceID))
		delete(ce.activeTraces, traceID)
		return
	}

	select {
	case ce.traceCh <- *t:
		// sent success
	default:
		// Channel full
	}

	delete(ce.activeTraces, traceID)
}

func (ce *CorrelationEngine) buildHierarchy(spans map[string]*models.ParsedSpan) *models.Trace {

	var rootSpan *models.ParsedSpan
	childrenMap := make(map[string][]*models.ParsedSpan)

	for _, span := range spans {
		if span.ParentSpanID == "" {
			rootSpan = span
		} else {
			childrenMap[span.ParentSpanID] = append(childrenMap[span.ParentSpanID], span)
		}
	}

	// If no root span found, we can't build a valid trace
	if rootSpan == nil {
		ce.logger.Warn("attempted to build hierarchy without root span",
			slog.Int("spanCount", len(spans)))
		return nil
	}

	// Build the hierarchical SpanTree starting from root
	// spanTree := buildSpanTree(rootSpan, childrenMap)

	return &models.Trace{
		TraceID:  rootSpan.TraceID,
		Spans:    flattenToSlice(spans),
		RootSpan: rootSpan,
		// Children:  spanTree,
		StartTime: rootSpan.StartTime,
		EndTime:   rootSpan.EndTime,
		Duration:  rootSpan.EndTime.Sub(rootSpan.StartTime),
		Services:  extractServices(spans),
		Status:    determineOverallStatus(spans),
	}
}

// buildSpanTree recursively constructs a SpanTree from a span and its children
func buildSpanTree(span *models.ParsedSpan, childrenMap map[string][]*models.ParsedSpan) models.SpanTree {
	tree := models.SpanTree{
		ParsedSpan: *span,
		Children:   []*models.SpanTree{},
	}

	// Recursively build children
	if children, exists := childrenMap[span.SpanID]; exists {
		for _, child := range children {
			childTree := buildSpanTree(child, childrenMap)
			tree.Children = append(tree.Children, &childTree)
		}
	}

	return tree
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

	// TODO: this span specifically could still be "unset"
	return span.Status
}
