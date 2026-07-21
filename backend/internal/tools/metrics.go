package tools

import (
	"sort"
	"strings"
	"sync"
)

// ToolMetricSummary is a privacy-safe aggregate of terminal tool lifecycle
// events. It intentionally excludes arguments, result content, and user IDs.
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
	byScopeTool map[string]*ToolMetricSummary
}

func init() {
	runtimeToolMetrics.byScopeTool = make(map[string]*ToolMetricSummary)
}

func toolMetricKey(userID, toolName string) string {
	return userID + "\x00" + toolName
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
	key := toolMetricKey(event.Scope.UserID, event.ToolName)
	metric := runtimeToolMetrics.byScopeTool[key]
	if metric == nil {
		metric = &ToolMetricSummary{ToolName: event.ToolName}
		runtimeToolMetrics.byScopeTool[key] = metric
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

func mergeToolMetric(target *ToolMetricSummary, source ToolMetricSummary) {
	target.Calls += source.Calls
	target.Successes += source.Successes
	target.Failures += source.Failures
	target.Timeouts += source.Timeouts
	target.Cancellations += source.Cancellations
	target.Retries += source.Retries
	target.TotalDurationMS += source.TotalDurationMS
	if source.LastDurationMS != 0 || source.LastEvent != "" {
		target.LastDurationMS = source.LastDurationMS
		target.LastEvent = source.LastEvent
	}
}

func snapshotToolMetrics(userID *string) []ToolMetricSummary {
	runtimeToolMetrics.RLock()
	aggregated := make(map[string]*ToolMetricSummary)
	for key, metric := range runtimeToolMetrics.byScopeTool {
		separator := strings.IndexByte(key, 0)
		metricUserID := ""
		if separator >= 0 {
			metricUserID = key[:separator]
		}
		if userID != nil && metricUserID != *userID {
			continue
		}
		current := aggregated[metric.ToolName]
		if current == nil {
			copyMetric := *metric
			aggregated[metric.ToolName] = &copyMetric
			continue
		}
		mergeToolMetric(current, *metric)
	}
	runtimeToolMetrics.RUnlock()

	out := make([]ToolMetricSummary, 0, len(aggregated))
	for _, metric := range aggregated {
		out = append(out, *metric)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ToolName < out[j].ToolName })
	return out
}

// ToolMetricsSnapshot returns process-wide runtime aggregates for internal
// diagnostics and tests. It does not include user identifiers.
func ToolMetricsSnapshot() []ToolMetricSummary {
	return snapshotToolMetrics(nil)
}

// ToolMetricsSnapshotForUser returns only metrics recorded under the supplied
// authenticated user scope. Solo mode naturally uses the empty user ID scope.
func ToolMetricsSnapshotForUser(userID string) []ToolMetricSummary {
	return snapshotToolMetrics(&userID)
}

func resetToolMetricsForTest() {
	runtimeToolMetrics.Lock()
	runtimeToolMetrics.byScopeTool = make(map[string]*ToolMetricSummary)
	runtimeToolMetrics.Unlock()
}
