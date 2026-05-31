package video

type TimelineDocument struct {
	Version    int              `json:"version"`
	Canvas     TimelineCanvas   `json:"canvas"`
	DurationMS int64            `json:"duration_ms"`
	Tracks     []TimelineTrack  `json:"tracks"`
	Markers    []TimelineMarker `json:"markers"`
	Metadata   map[string]any   `json:"metadata"`
}

type TimelineCanvas struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FPS        int    `json:"fps"`
	Background string `json:"background"`
}

type TimelineTrack struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Name    string         `json:"name"`
	Locked  bool           `json:"locked"`
	Muted   bool           `json:"muted"`
	Visible bool           `json:"visible"`
	Clips   []TimelineClip `json:"clips"`
}

type TimelineClip struct {
	ID         string         `json:"id"`
	AssetID    string         `json:"asset_id,omitempty"`
	StartMS    int64          `json:"start_ms"`
	DurationMS int64          `json:"duration_ms"`
	TrimInMS   int64          `json:"trim_in_ms"`
	TrimOutMS  int64          `json:"trim_out_ms"`
	Transform  map[string]any `json:"transform,omitempty"`
	Effects    []any          `json:"effects"`
	Keyframes  []any          `json:"keyframes"`
}

type TimelineMarker struct {
	ID     string `json:"id"`
	TimeMS int64  `json:"time_ms"`
	Label  string `json:"label"`
}

func NewEmptyTimeline(width, height, fps int) TimelineDocument {
	if width <= 0 {
		width = DefaultProjectWidth
	}
	if height <= 0 {
		height = DefaultProjectHeight
	}
	if fps <= 0 {
		fps = DefaultProjectFPS
	}
	return TimelineDocument{
		Version: 1,
		Canvas: TimelineCanvas{
			Width:      width,
			Height:     height,
			FPS:        fps,
			Background: "#000000",
		},
		DurationMS: 30000,
		Tracks:     []TimelineTrack{},
		Markers:    []TimelineMarker{},
		Metadata:   map[string]any{},
	}
}
