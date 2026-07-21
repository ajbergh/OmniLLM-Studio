package tools

import (
	"sort"
	"sync"
)

// ToolMetricSummary is a privacy-safe aggregate of terminal tool lifecycle
// events. It intentionally excludes arguments and result content.
type ToolMetricSummary struct {
	ToolName        string `json:"tool_name"`
	Calls           int64  `json:"calls"`
	Successes       int64  `json:"successes"`
	Failures        int64  `json:"failures"`
	Timeouts        int64  `json:"timeouts"`
	Cancellations   int64  `json:"cancellations"`
	Retries         int64  `json:"retries"`
	TotalDurationMS int64  `json:"total_duration_ms"`
	LastDurationMS  int64  `json:"last_duration_ms"`
	LastEvent       string `json:"last_event"`
}

var runtimeToolMetrics struct {
	sync.RWMutex
	byTool map[string]*ToolMetricSummary
}

func init() {
	runtimeToolMetrics.byTool = make(map[string]*ToolMetricSummary)
}

func recordGlobalToolMetric(event ToolEvent) {
	if event.ToolName == "" {
		return
	}
	if event.Type != ToolEventCompleted && event.Type != ToolEventFailed && event.Type != ToolEventTimedOut && event.Type != ToolEventCancelled {
		return
	}

	runtimeToolMetrics.Lock()
	defer runtimeToolMetrics.Unlock()
	metric := runtimeToolMetrics.byTool[event.ToolName]
	if metric == nil {
		metric = &ToolMetricSummary{ToolName: event.ToolName}
		runtimeToolMetrics.byTool[event.ToolName] = metric
	}
	metric.Calls++
	metric.LastEvent = string(event.Type)
	duration := int64MetricValue(event.Data["duration_ms"])
	metric.LastDurationMS = duration
	metric.TotalDurationMS += duration
	attempts := int64MetricValue(event.Data["attempt_count"])
	if attempts > 1 {
		metric.Retries += attempts - 1
	}
	switch event.Type {
	case ToolEventCompleted:
		if isError, _ := event.Data["is_error"].(bool); isError {
			metric.Failures++
		} else {
			metric.Successes++
		}
	case ToolEventFailed:
		metric.Failures++
	case ToolEventTimedOut:
		metric.Timeouts++
	case ToolEventCancelled:
		metric.Cancellations++
	}
}

func int64MetricValue(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

// ToolMetricsSnapshot returns a stable name-sorted copy of the current runtime
// aggregates. Metrics reset when the process restarts; the existing
// tool_invocations audit table remains the durable event history.
func ToolMetricsSnapshot() []ToolMetricSummary {
	runtimeToolMetrics.RLock()
	out := make([]ToolMetricSummary, 0, len(runtimeToolMetrics.byTool))
	for _, metric := range runtimeToolMetrics.byTool {
		out = append(out, *metric)
	}
	runtimeToolMetrics.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ToolName < out[j].ToolName })
	return out
}

func resetToolMetricsForTest() {
	runtimeToolMetrics.Lock()
	runtimeToolMetrics.byTool = make(map[string]*ToolMetricSummary)
	runtimeToolMetrics.Unlock()
}
