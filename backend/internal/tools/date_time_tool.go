package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DateTimeTool provides deterministic current-time, timezone conversion, and
// date arithmetic without asking an LLM to infer temporal facts.
type DateTimeTool struct{}

func NewDateTimeTool() *DateTimeTool { return &DateTimeTool{} }

func (t *DateTimeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:             "date_time",
		Description:      "Get the current time in an IANA timezone, convert a timestamp between timezones, or add a duration to a timestamp.",
		Category:         "utility",
		Enabled:          true,
		Version:          "2",
		Risk:             RiskLow,
		ReadOnly:         true,
		SupportsParallel: true,
		DefaultTimeoutMS: 2000,
		MaxResultBytes:   16384,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"operation":{"type":"string","enum":["now","convert","add"],"default":"now"},
				"timezone":{"type":"string","description":"IANA timezone, for example America/Toronto"},
				"from_timezone":{"type":"string"},
				"to_timezone":{"type":"string"},
				"timestamp":{"type":"string","description":"RFC3339 timestamp or local YYYY-MM-DDTHH:MM:SS"},
				"duration":{"type":"string","description":"Go duration such as 45m, 24h, or -30m"}
			}
		}`),
		OutputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"timestamp":{"type":"string"},
				"timezone":{"type":"string"},
				"utc_offset":{"type":"string"},
				"unix":{"type":"integer"}
			}
		}`),
		Examples: []ToolExample{
			{Description: "Current Toronto time", Arguments: json.RawMessage(`{"operation":"now","timezone":"America/Toronto"}`)},
			{Description: "Convert a local time", Arguments: json.RawMessage(`{"operation":"convert","timestamp":"2026-07-18T19:00:00","from_timezone":"America/Toronto","to_timezone":"Europe/Prague"}`)},
		},
	}
}

type dateTimeArgs struct {
	Operation    string `json:"operation"`
	Timezone     string `json:"timezone"`
	FromTimezone string `json:"from_timezone"`
	ToTimezone   string `json:"to_timezone"`
	Timestamp    string `json:"timestamp"`
	Duration     string `json:"duration"`
}

func (t *DateTimeTool) Validate(raw json.RawMessage) error {
	var args dateTimeArgs
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	switch strings.ToLower(args.Operation) {
	case "", "now":
		return validateTimezone(defaultString(args.Timezone, "UTC"))
	case "convert":
		if args.Timestamp == "" || args.ToTimezone == "" {
			return fmt.Errorf("timestamp and to_timezone are required")
		}
		if err := validateTimezone(defaultString(args.FromTimezone, "UTC")); err != nil {
			return err
		}
		return validateTimezone(args.ToTimezone)
	case "add":
		if args.Timestamp == "" || args.Duration == "" {
			return fmt.Errorf("timestamp and duration are required")
		}
		if _, err := time.ParseDuration(args.Duration); err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		return validateTimezone(defaultString(args.Timezone, "UTC"))
	default:
		return fmt.Errorf("unsupported operation %q", args.Operation)
	}
}

func (t *DateTimeTool) Execute(_ context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args dateTimeArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
	}
	op := strings.ToLower(args.Operation)
	if op == "" {
		op = "now"
	}

	var result time.Time
	var zoneName string
	switch op {
	case "now":
		zoneName = defaultString(args.Timezone, "UTC")
		loc, _ := time.LoadLocation(zoneName)
		result = time.Now().In(loc)
	case "convert":
		from := defaultString(args.FromTimezone, "UTC")
		parsed, err := parseTimestamp(args.Timestamp, from)
		if err != nil {
			return nil, err
		}
		zoneName = args.ToTimezone
		loc, _ := time.LoadLocation(zoneName)
		result = parsed.In(loc)
	case "add":
		zoneName = defaultString(args.Timezone, "UTC")
		parsed, err := parseTimestamp(args.Timestamp, zoneName)
		if err != nil {
			return nil, err
		}
		duration, _ := time.ParseDuration(args.Duration)
		result = parsed.Add(duration)
	}

	_, offsetSeconds := result.Zone()
	offset := formatUTCOffset(offsetSeconds)
	structured, _ := json.Marshal(map[string]interface{}{
		"timestamp":  result.Format(time.RFC3339),
		"timezone":   zoneName,
		"utc_offset": offset,
		"unix":       result.Unix(),
		"weekday":    result.Weekday().String(),
	})
	return &ToolResult{
		Content:    fmt.Sprintf("%s (%s, UTC%s)", result.Format("Monday, January 2, 2006 3:04:05 PM"), zoneName, offset),
		Structured: structured,
		Metadata: map[string]interface{}{
			"timezone": zoneName,
		},
	}, nil
}

func parseTimestamp(value, timezone string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, err
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("timestamp must be RFC3339 or a supported local date/time")
}

func validateTimezone(name string) error {
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return nil
}

func formatUTCOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, (seconds%3600)/60)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
