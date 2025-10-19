package correlation

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/liamcoop/distrace/internal/models"
)

// Helper function to create a test logger that discards output
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

// Helper function to create a mock ParsedSpan
func createMockSpan(traceID, spanID, parentSpanID, serviceName string) models.ParsedSpan {
	return models.ParsedSpan{
		TraceID:       traceID,
		SpanID:        spanID,
		ParentSpanID:  parentSpanID,
		ServiceName:   serviceName,
		OperationName: "test-operation",
		StartTime:     time.Now(),
		EndTime:       time.Now().Add(100 * time.Millisecond),
		Duration:      100 * time.Millisecond,
		Status: models.SpanStatus{
			Code:    "STATUS_CODE_OK",
			Message: "OK",
		},
		Tags: map[string]string{},
		Kind: models.SpanKindServer,
	}
}

// Helper to create a complete trace with root + children
func createMockTrace(traceID string, numChildren int) []models.ParsedSpan {
	spans := make([]models.ParsedSpan, 0, numChildren+1)

	// Root span
	rootSpan := createMockSpan(traceID, "root", "", "service-root")
	spans = append(spans, rootSpan)

	// Child spans
	for i := 0; i < numChildren; i++ {
		childSpan := createMockSpan(traceID, "child-"+string(rune('0'+i)), "root", "service-child")
		spans = append(spans, childSpan)
	}

	return spans
}

// Test extractServices function
func TestExtractServices(t *testing.T) {
	tests := []struct {
		name     string
		spans    map[string]*models.ParsedSpan
		expected []string
	}{
		{
			name: "single service",
			spans: map[string]*models.ParsedSpan{
				"span1": {ServiceName: "service-a"},
			},
			expected: []string{"service-a"},
		},
		{
			name: "multiple unique services",
			spans: map[string]*models.ParsedSpan{
				"span1": {ServiceName: "service-a"},
				"span2": {ServiceName: "service-b"},
				"span3": {ServiceName: "service-c"},
			},
			expected: []string{"service-a", "service-b", "service-c"},
		},
		{
			name: "duplicate services",
			spans: map[string]*models.ParsedSpan{
				"span1": {ServiceName: "service-a"},
				"span2": {ServiceName: "service-a"},
				"span3": {ServiceName: "service-b"},
			},
			expected: []string{"service-a", "service-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractServices(tt.spans)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d services, got %d", len(tt.expected), len(result))
			}

			// Check all expected services are present
			serviceMap := make(map[string]bool)
			for _, s := range result {
				serviceMap[s] = true
			}

			for _, expectedService := range tt.expected {
				if !serviceMap[expectedService] {
					t.Errorf("expected service %s not found in result", expectedService)
				}
			}
		})
	}
}

// Test flattenToSlice function
func TestFlattenToSlice(t *testing.T) {
	span1 := createMockSpan("trace1", "span1", "", "service-a")
	span2 := createMockSpan("trace1", "span2", "span1", "service-b")

	spans := map[string]*models.ParsedSpan{
		"span1": &span1,
		"span2": &span2,
	}

	result := flattenToSlice(spans)

	if len(result) != 2 {
		t.Errorf("expected 2 spans, got %d", len(result))
	}

	// Verify both spans are present
	foundSpan1 := false
	foundSpan2 := false
	for _, s := range result {
		if s.SpanID == "span1" {
			foundSpan1 = true
		}
		if s.SpanID == "span2" {
			foundSpan2 = true
		}
	}

	if !foundSpan1 || !foundSpan2 {
		t.Error("not all spans were found in flattened slice")
	}
}

// Test determineOverallStatus function
func TestDetermineOverallStatus(t *testing.T) {
	okStatus := models.SpanStatus{Code: "STATUS_CODE_OK", Message: "OK"}
	errorStatus := models.SpanStatus{Code: "STATUS_CODE_ERROR", Message: "Error occurred"}

	tests := []struct {
		name     string
		spans    map[string]*models.ParsedSpan
		expected models.SpanStatus
	}{
		{
			name: "all OK",
			spans: map[string]*models.ParsedSpan{
				"span1": {Status: okStatus},
				"span2": {Status: okStatus},
			},
			expected: okStatus,
		},
		{
			name: "one error",
			spans: map[string]*models.ParsedSpan{
				"span1": {Status: okStatus},
				"span2": {Status: errorStatus},
			},
			expected: errorStatus,
		},
		{
			name: "multiple errors",
			spans: map[string]*models.ParsedSpan{
				"span1": {Status: errorStatus},
				"span2": {Status: errorStatus},
			},
			expected: errorStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineOverallStatus(tt.spans)

			if result.Code != tt.expected.Code {
				t.Errorf("expected status code %s, got %s", tt.expected.Code, result.Code)
			}
		})
	}
}

// Test buildSpanTree function
func TestBuildSpanTree(t *testing.T) {
	rootSpan := createMockSpan("trace1", "root", "", "service-root")
	child1 := createMockSpan("trace1", "child1", "root", "service-a")
	child2 := createMockSpan("trace1", "child2", "root", "service-b")
	grandchild := createMockSpan("trace1", "grandchild", "child1", "service-c")

	childrenMap := map[string][]*models.ParsedSpan{
		"root":   {&child1, &child2},
		"child1": {&grandchild},
	}

	tree := buildSpanTree(&rootSpan, childrenMap)

	// Verify root
	if tree.SpanID != "root" {
		t.Errorf("expected root span ID 'root', got %s", tree.SpanID)
	}

	// Verify root has 2 children
	if len(tree.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(tree.Children))
	}

	// Verify child1 has 1 grandchild
	for _, child := range tree.Children {
		if child.SpanID == "child1" {
			if len(child.Children) != 1 {
				t.Errorf("expected child1 to have 1 child, got %d", len(child.Children))
			}
			if child.Children[0].SpanID != "grandchild" {
				t.Errorf("expected grandchild span ID 'grandchild', got %s", child.Children[0].SpanID)
			}
		}
	}
}

// Test buildHierarchy function
func TestBuildHierarchy(t *testing.T) {
	logger := newTestLogger()
	spanCh := make(chan models.ParsedSpan, 10)
	traceCh := make(chan models.Trace, 10)
	ce := NewCorrelationEngine(logger, spanCh, traceCh)

	t.Run("with root span", func(t *testing.T) {
		rootSpan := createMockSpan("trace1", "root", "", "service-root")
		child1 := createMockSpan("trace1", "child1", "root", "service-a")

		spans := map[string]*models.ParsedSpan{
			"root":   &rootSpan,
			"child1": &child1,
		}

		trace := ce.buildHierarchy(spans)

		if trace == nil {
			t.Fatal("expected trace, got nil")
		}

		if trace.TraceID != "trace1" {
			t.Errorf("expected traceID 'trace1', got %s", trace.TraceID)
		}

		if trace.RootSpan.SpanID != "root" {
			t.Errorf("expected root span ID 'root', got %s", trace.RootSpan.SpanID)
		}

		if len(trace.Spans) != 2 {
			t.Errorf("expected 2 spans, got %d", len(trace.Spans))
		}
	})

	t.Run("without root span", func(t *testing.T) {
		child1 := createMockSpan("trace2", "child1", "root", "service-a")
		child2 := createMockSpan("trace2", "child2", "root", "service-b")

		spans := map[string]*models.ParsedSpan{
			"child1": &child1,
			"child2": &child2,
		}

		trace := ce.buildHierarchy(spans)

		if trace != nil {
			t.Error("expected nil trace when no root span exists")
		}
	})
}

// Test processSpan function
func TestProcessSpan(t *testing.T) {
	logger := newTestLogger()
	spanCh := make(chan models.ParsedSpan, 10)
	traceCh := make(chan models.Trace, 10)

	t.Run("creates new trace for new traceID", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		span := createMockSpan("trace1", "span1", "root", "service-a")

		ce.processSpan(span)

		if len(ce.activeTraces) != 1 {
			t.Errorf("expected 1 active trace, got %d", len(ce.activeTraces))
		}

		trace, exists := ce.activeTraces["trace1"]
		if !exists {
			t.Fatal("expected trace1 to exist in activeTraces")
		}

		if len(trace.Spans) != 1 {
			t.Errorf("expected 1 span in trace, got %d", len(trace.Spans))
		}
	})

	t.Run("adds span to existing trace", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		span1 := createMockSpan("trace1", "span1", "root", "service-a")
		span2 := createMockSpan("trace1", "span2", "root", "service-b")

		ce.processSpan(span1)
		ce.processSpan(span2)

		trace := ce.activeTraces["trace1"]
		if len(trace.Spans) != 2 {
			t.Errorf("expected 2 spans in trace, got %d", len(trace.Spans))
		}

		if _, exists := trace.Spans["span1"]; !exists {
			t.Error("expected span1 to exist in trace")
		}
		if _, exists := trace.Spans["span2"]; !exists {
			t.Error("expected span2 to exist in trace")
		}
	})

	t.Run("identifies root span correctly", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		rootSpan := createMockSpan("trace1", "root", "", "service-root")

		ce.processSpan(rootSpan)

		trace := ce.activeTraces["trace1"]
		if trace.RootSpan == nil {
			t.Fatal("expected root span to be set")
		}

		if trace.RootSpan.SpanID != "root" {
			t.Errorf("expected root span ID 'root', got %s", trace.RootSpan.SpanID)
		}
	})

	t.Run("root span arrives after children", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		child := createMockSpan("trace1", "child1", "root", "service-a")
		rootSpan := createMockSpan("trace1", "root", "", "service-root")

		// Child arrives first
		ce.processSpan(child)
		if ce.activeTraces["trace1"].RootSpan != nil {
			t.Error("root span should not be set yet")
		}

		// Root arrives later
		ce.processSpan(rootSpan)
		if ce.activeTraces["trace1"].RootSpan == nil {
			t.Fatal("expected root span to be set after processing")
		}
		if ce.activeTraces["trace1"].RootSpan.SpanID != "root" {
			t.Errorf("expected root span ID 'root', got %s", ce.activeTraces["trace1"].RootSpan.SpanID)
		}
	})

	t.Run("updates LastActivity timestamp", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		span1 := createMockSpan("trace1", "span1", "root", "service-a")

		ce.processSpan(span1)
		firstActivity := ce.activeTraces["trace1"].LastActivity

		time.Sleep(10 * time.Millisecond)

		span2 := createMockSpan("trace1", "span2", "root", "service-b")
		ce.processSpan(span2)
		secondActivity := ce.activeTraces["trace1"].LastActivity

		if !secondActivity.After(firstActivity) {
			t.Error("expected LastActivity to be updated")
		}
	})
}

// Test completeTrace function
func TestCompleteTrace(t *testing.T) {
	logger := newTestLogger()
	spanCh := make(chan models.ParsedSpan, 10)
	traceCh := make(chan models.Trace, 10)

	t.Run("sends trace to channel when root exists", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		rootSpan := createMockSpan("trace1", "root", "", "service-root")
		child := createMockSpan("trace1", "child1", "root", "service-a")

		ce.processSpan(rootSpan)
		ce.processSpan(child)

		ce.completeTrace("trace1")

		// Check trace was sent to channel
		select {
		case trace := <-traceCh:
			if trace.TraceID != "trace1" {
				t.Errorf("expected traceID 'trace1', got %s", trace.TraceID)
			}
			if len(trace.Spans) != 2 {
				t.Errorf("expected 2 spans, got %d", len(trace.Spans))
			}
		default:
			t.Error("expected trace to be sent to channel")
		}

		// Check trace was removed from activeTraces
		if _, exists := ce.activeTraces["trace1"]; exists {
			t.Error("expected trace to be removed from activeTraces")
		}
	})

	t.Run("skips trace without root span", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		child1 := createMockSpan("trace1", "child1", "root", "service-a")
		child2 := createMockSpan("trace1", "child2", "root", "service-b")

		ce.processSpan(child1)
		ce.processSpan(child2)

		ce.completeTrace("trace1")

		// Check nothing was sent to channel
		select {
		case <-traceCh:
			t.Error("did not expect trace to be sent without root span")
		default:
			// Expected
		}

		// Check trace was still removed from activeTraces
		if _, exists := ce.activeTraces["trace1"]; exists {
			t.Error("expected trace to be removed from activeTraces even without root")
		}
	})

	t.Run("handles non-existent traceID gracefully", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)

		// Should not panic
		ce.completeTrace("non-existent-trace")

		// Nothing should be sent
		select {
		case <-traceCh:
			t.Error("did not expect anything to be sent")
		default:
			// Expected
		}
	})

	t.Run("handles full channel gracefully", func(t *testing.T) {
		// Create channel with buffer of 1
		smallTraceCh := make(chan models.Trace, 1)
		ce := NewCorrelationEngine(logger, spanCh, smallTraceCh)

		// Fill the channel
		rootSpan1 := createMockSpan("trace1", "root1", "", "service-root")
		ce.processSpan(rootSpan1)
		ce.completeTrace("trace1")

		// Try to send another (channel is full)
		rootSpan2 := createMockSpan("trace2", "root2", "", "service-root")
		ce.processSpan(rootSpan2)
		ce.completeTrace("trace2")

		// Should not panic, trace should still be removed
		if _, exists := ce.activeTraces["trace2"]; exists {
			t.Error("expected trace2 to be removed even if channel is full")
		}
	})
}

// Test cleanupStalledTraces function
func TestCleanupStalledTraces(t *testing.T) {
	logger := newTestLogger()
	spanCh := make(chan models.ParsedSpan, 10)
	traceCh := make(chan models.Trace, 10)

	t.Run("completes traces inactive for 30s with root span", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		rootSpan := createMockSpan("trace1", "root", "", "service-root")
		child := createMockSpan("trace1", "child1", "root", "service-a")

		ce.processSpan(rootSpan)
		ce.processSpan(child)

		// Manually set LastActivity to 31 seconds ago
		trace := ce.activeTraces["trace1"]
		trace.LastActivity = time.Now().Add(-31 * time.Second)

		ce.cleanupStalledTraces()

		// Check trace was completed
		if _, exists := ce.activeTraces["trace1"]; exists {
			t.Error("expected inactive trace to be completed")
		}

		// Check trace was sent
		select {
		case <-traceCh:
			// Expected
		default:
			t.Error("expected trace to be sent to channel")
		}
	})

	t.Run("does not complete recent traces", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		rootSpan := createMockSpan("trace1", "root", "", "service-root")

		ce.processSpan(rootSpan)

		ce.cleanupStalledTraces()

		// Trace should still be active
		if _, exists := ce.activeTraces["trace1"]; !exists {
			t.Error("expected recent trace to remain active")
		}

		// Nothing should be sent
		select {
		case <-traceCh:
			t.Error("did not expect recent trace to be completed")
		default:
			// Expected
		}
	})

	t.Run("force completes very old traces without root", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		child := createMockSpan("trace1", "child1", "root", "service-a")

		ce.processSpan(child)

		// Manually set StartTime to 6 minutes ago
		trace := ce.activeTraces["trace1"]
		trace.StartTime = time.Now().Add(-6 * time.Minute)

		ce.cleanupStalledTraces()

		// Check trace was removed
		if _, exists := ce.activeTraces["trace1"]; exists {
			t.Error("expected old trace without root to be removed")
		}

		// Nothing should be sent (no root span)
		select {
		case <-traceCh:
			t.Error("did not expect trace without root to be sent")
		default:
			// Expected
		}
	})

	t.Run("does not complete inactive traces without root span if not old", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		child := createMockSpan("trace1", "child1", "root", "service-a")

		ce.processSpan(child)

		// Set LastActivity to 31 seconds ago but trace is not old
		trace := ce.activeTraces["trace1"]
		trace.LastActivity = time.Now().Add(-31 * time.Second)

		ce.cleanupStalledTraces()

		// Trace should still exist (no root span, not old enough)
		if _, exists := ce.activeTraces["trace1"]; !exists {
			t.Error("expected trace without root to remain if not old")
		}
	})
}

// Test flushAll function
func TestFlushAll(t *testing.T) {
	logger := newTestLogger()
	spanCh := make(chan models.ParsedSpan, 10)
	traceCh := make(chan models.Trace, 10)

	t.Run("completes all active traces", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)

		// Create multiple traces
		root1 := createMockSpan("trace1", "root1", "", "service-a")
		root2 := createMockSpan("trace2", "root2", "", "service-b")
		root3 := createMockSpan("trace3", "root3", "", "service-c")

		ce.processSpan(root1)
		ce.processSpan(root2)
		ce.processSpan(root3)

		if len(ce.activeTraces) != 3 {
			t.Errorf("expected 3 active traces, got %d", len(ce.activeTraces))
		}

		ce.flushAll()

		// All traces should be cleared
		if len(ce.activeTraces) != 0 {
			t.Errorf("expected 0 active traces after flush, got %d", len(ce.activeTraces))
		}

		// All traces should be sent
		completedCount := 0
		for i := 0; i < 3; i++ {
			select {
			case <-traceCh:
				completedCount++
			default:
				break
			}
		}

		if completedCount != 3 {
			t.Errorf("expected 3 traces to be sent, got %d", completedCount)
		}
	})

	t.Run("handles empty activeTraces", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)

		// Should not panic
		ce.flushAll()

		if len(ce.activeTraces) != 0 {
			t.Error("expected activeTraces to remain empty")
		}
	})
}

// Integration test - full flow
func TestCorrelationEngineIntegration(t *testing.T) {
	logger := newTestLogger()
	spanCh := make(chan models.ParsedSpan, 10)
	traceCh := make(chan models.Trace, 10)

	t.Run("full flow: spans in, traces out", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)

		// Simulate spans arriving in random order
		child1 := createMockSpan("trace1", "child1", "root", "service-a")
		child2 := createMockSpan("trace1", "child2", "root", "service-b")
		rootSpan := createMockSpan("trace1", "root", "", "service-root")

		// Send spans through channel
		spanCh <- child1
		spanCh <- child2
		spanCh <- rootSpan

		// Process spans
		for i := 0; i < 3; i++ {
			span := <-spanCh
			ce.processSpan(span)
		}

		// Complete the trace
		ce.completeTrace("trace1")

		// Verify trace was output
		select {
		case trace := <-traceCh:
			if trace.TraceID != "trace1" {
				t.Errorf("expected traceID 'trace1', got %s", trace.TraceID)
			}
			if len(trace.Spans) != 3 {
				t.Errorf("expected 3 spans in trace, got %d", len(trace.Spans))
			}
			if trace.RootSpan.SpanID != "root" {
				t.Errorf("expected root span ID 'root', got %s", trace.RootSpan.SpanID)
			}
		default:
			t.Error("expected trace to be output")
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		ce := NewCorrelationEngine(logger, spanCh, traceCh)
		ctx, cancel := context.WithCancel(context.Background())

		// Add a trace
		rootSpan := createMockSpan("trace1", "root", "", "service-root")
		ce.processSpan(rootSpan)

		// Start the engine in a goroutine
		errCh := make(chan error, 1)
		go func() {
			errCh <- ce.Start(ctx)
		}()

		// Cancel the context
		cancel()

		// Wait for the engine to stop
		select {
		case err := <-errCh:
			if err != context.Canceled {
				t.Errorf("expected context.Canceled error, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for engine to stop")
		}

		// Verify trace was flushed
		select {
		case trace := <-traceCh:
			if trace.TraceID != "trace1" {
				t.Errorf("expected flushed trace1, got %s", trace.TraceID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("expected trace to be flushed on shutdown")
		}
	})

	t.Run("handles channel closure", func(t *testing.T) {
		localSpanCh := make(chan models.ParsedSpan, 10)
		ce := NewCorrelationEngine(logger, localSpanCh, traceCh)

		// Add a trace
		rootSpan := createMockSpan("trace1", "root", "", "service-root")
		ce.processSpan(rootSpan)

		// Start the engine
		ctx := context.Background()
		errCh := make(chan error, 1)
		go func() {
			errCh <- ce.Start(ctx)
		}()

		// Close the span channel
		close(localSpanCh)

		// Wait for the engine to stop
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("expected nil error on channel close, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Error("timeout waiting for engine to stop")
		}

		// Verify trace was flushed
		select {
		case trace := <-traceCh:
			if trace.TraceID != "trace1" {
				t.Errorf("expected flushed trace1, got %s", trace.TraceID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("expected trace to be flushed on channel close")
		}
	})
}
