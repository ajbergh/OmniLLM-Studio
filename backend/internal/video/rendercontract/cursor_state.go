package rendercontract

import (
	"fmt"
	"math"
)

const (
	CursorStateContractV1 = "cursor-state-v1"
	CursorStateContractV2 = "cursor-state-v2"
	CursorClickWindowMS   = int64(300)
	CursorMinScale        = 0.25
	CursorMaxScale        = 4.0
)

// EvaluatedCursorState is renderer-independent cursor state at one exact
// clip-relative presentation time. Coordinates remain canvas pixels from the
// top-left. Click is the sampled press-proximity decision; consumers only draw
// a ring when both Click and ClickRings are true.
type EvaluatedCursorState struct {
	ContractVersion string  `json:"contract_version"`
	Visible         bool    `json:"visible"`
	Scale           float64 `json:"scale"`
	Highlight       bool    `json:"highlight"`
	ClickRings      bool    `json:"click_rings"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Click           bool    `json:"click"`
}

// EvaluateCursorState samples Timeline v2 cursor metadata at exact rational
// clip-relative time. Linear interpolation remains cursor-state-v1. When
// smoothing=true, cursor-state-v2 applies deterministic cubic smoothstep timing
// (3t^2-2t^3) between the same authored event coordinates. Endpoint hold and
// the strict <300ms click-proximity rule are identical in both versions.
func EvaluateCursorState(cursor *TimelineV2Cursor, time RationalMilliseconds) (*EvaluatedCursorState, error) {
	if cursor == nil {
		return nil, nil
	}
	if time.Denominator <= 0 {
		return nil, fmt.Errorf("canonical cursor sample denominator must be positive")
	}
	visible := cursor.Visible == nil || *cursor.Visible
	if !visible || len(cursor.Events) == 0 {
		return nil, nil
	}

	scale := 1.0
	if cursor.Scale != nil {
		scale = *cursor.Scale
	}
	if math.IsNaN(scale) || math.IsInf(scale, 0) {
		return nil, fmt.Errorf("canonical cursor scale must be finite")
	}
	if scale < CursorMinScale || scale > CursorMaxScale {
		return nil, fmt.Errorf("canonical cursor scale must be between %.2g and %.2g", CursorMinScale, CursorMaxScale)
	}
	if err := validateCursorEvents(cursor.Events); err != nil {
		return nil, err
	}

	x, y := sampleCursorPosition(cursor.Events, time, cursor.Smoothing)
	click := cursorClickNearTime(cursor.Events, time)
	contractVersion := CursorStateContractV1
	if cursor.Smoothing {
		contractVersion = CursorStateContractV2
	}
	return &EvaluatedCursorState{
		ContractVersion: contractVersion,
		Visible:         true,
		Scale:           scale,
		Highlight:       cursor.Highlight,
		ClickRings:      cursor.ClickRings,
		X:               x,
		Y:               y,
		Click:           click,
	}, nil
}

func validateCursorEvents(events []TimelineV2CursorEvent) error {
	previous := int64(-1)
	for index, event := range events {
		if event.TimeMS < 0 {
			return fmt.Errorf("canonical cursor event %d time_ms cannot be negative", index)
		}
		if index > 0 && event.TimeMS < previous {
			return fmt.Errorf("canonical cursor events must be ordered by time_ms")
		}
		if math.IsNaN(event.X) || math.IsInf(event.X, 0) || math.IsNaN(event.Y) || math.IsInf(event.Y, 0) {
			return fmt.Errorf("canonical cursor event %d coordinates must be finite", index)
		}
		previous = event.TimeMS
	}
	return nil
}

func sampleCursorPosition(events []TimelineV2CursorEvent, time RationalMilliseconds, smoothing bool) (float64, float64) {
	at := func(ms int64) int64 { return ms * time.Denominator }
	first := events[0]
	if time.Numerator <= at(first.TimeMS) {
		return first.X, first.Y
	}
	previous := first
	for index := 1; index < len(events); index++ {
		next := events[index]
		if time.Numerator <= at(next.TimeMS) {
			span := next.TimeMS - previous.TimeMS
			if span < 1 {
				span = 1
			}
			progress := float64(time.Numerator-at(previous.TimeMS)) / float64(span*time.Denominator)
			if smoothing {
				progress = smoothCursorProgress(progress)
			}
			return previous.X + (next.X-previous.X)*progress, previous.Y + (next.Y-previous.Y)*progress
		}
		previous = next
	}
	return previous.X, previous.Y
}

func smoothCursorProgress(progress float64) float64 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}
	return progress * progress * (3 - 2*progress)
}

func cursorClickNearTime(events []TimelineV2CursorEvent, time RationalMilliseconds) bool {
	window := CursorClickWindowMS * time.Denominator
	for _, event := range events {
		if !event.Click {
			continue
		}
		delta := event.TimeMS*time.Denominator - time.Numerator
		if delta < 0 {
			delta = -delta
		}
		if delta < window {
			return true
		}
	}
	return false
}
