package rendercontract

import (
	"math"
	"strings"
	"testing"
)

func TestEvaluateCursorStateMatchesPreviewSamplingSemantics(t *testing.T) {
	scale := 1.25
	cursor := &TimelineV2Cursor{
		Scale: &scale, Highlight: true, ClickRings: true,
		Events: []TimelineV2CursorEvent{
			{TimeMS: 0, X: 10, Y: 20},
			{TimeMS: 1000, X: 110, Y: 220, Click: true},
		},
	}
	state, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 500, Denominator: 1})
	if err != nil {
		t.Fatalf("EvaluateCursorState: %v", err)
	}
	if state == nil {
		t.Fatal("state is nil")
	}
	if state.ContractVersion != CursorStateContractV1 || !state.Visible || state.Scale != scale || !state.Highlight || !state.ClickRings {
		t.Fatalf("static state = %+v", state)
	}
	if state.X != 60 || state.Y != 120 || state.Click {
		t.Fatalf("sampled state = %+v", state)
	}

	// The click window is strict: exactly 300ms away is inactive, 299ms is active.
	boundary, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 700, Denominator: 1})
	if err != nil {
		t.Fatalf("boundary: %v", err)
	}
	if boundary == nil || boundary.Click {
		t.Fatalf("click at exact 300ms boundary = %+v", boundary)
	}
	inside, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 701, Denominator: 1})
	if err != nil {
		t.Fatalf("inside: %v", err)
	}
	if inside == nil || !inside.Click {
		t.Fatalf("click inside 300ms window = %+v", inside)
	}
}

func TestEvaluateCursorStateUsesExactRationalTimeAndEndpointHold(t *testing.T) {
	cursor := &TimelineV2Cursor{Events: []TimelineV2CursorEvent{
		{TimeMS: 100, X: 4, Y: 8},
		{TimeMS: 200, X: 14, Y: 28},
	}}
	before, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 50, Denominator: 1})
	if err != nil || before == nil || before.X != 4 || before.Y != 8 {
		t.Fatalf("before first = %+v err=%v", before, err)
	}
	after, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 250, Denominator: 1})
	if err != nil || after == nil || after.X != 14 || after.Y != 28 {
		t.Fatalf("after last = %+v err=%v", after, err)
	}
	fractional, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 451, Denominator: 3})
	if err != nil {
		t.Fatalf("fractional: %v", err)
	}
	wantProgress := (float64(451)/3 - 100) / 100
	wantX := 4 + 10*wantProgress
	wantY := 8 + 20*wantProgress
	if fractional == nil || math.Abs(fractional.X-wantX) > 1e-12 || math.Abs(fractional.Y-wantY) > 1e-12 {
		t.Fatalf("fractional = %+v want x=%v y=%v", fractional, wantX, wantY)
	}
}

func TestEvaluateCursorStateDefaultsVisibilityAndScale(t *testing.T) {
	cursor := &TimelineV2Cursor{Events: []TimelineV2CursorEvent{{TimeMS: 0, X: 1, Y: 2}}}
	state, err := EvaluateCursorState(cursor, RationalMilliseconds{Numerator: 0, Denominator: 1})
	if err != nil {
		t.Fatalf("EvaluateCursorState: %v", err)
	}
	if state == nil || !state.Visible || state.Scale != 1 {
		t.Fatalf("defaults = %+v", state)
	}

	hidden := false
	state, err = EvaluateCursorState(&TimelineV2Cursor{Visible: &hidden, Events: cursor.Events}, RationalMilliseconds{Numerator: 0, Denominator: 1})
	if err != nil || state != nil {
		t.Fatalf("hidden state = %+v err=%v", state, err)
	}
	state, err = EvaluateCursorState(&TimelineV2Cursor{}, RationalMilliseconds{Numerator: 0, Denominator: 1})
	if err != nil || state != nil {
		t.Fatalf("empty events state = %+v err=%v", state, err)
	}
}

func TestEvaluateCursorStateFailsClosedOnUndefinedOrInvalidState(t *testing.T) {
	validEvent := []TimelineV2CursorEvent{{TimeMS: 0, X: 1, Y: 2}}
	minBelow := CursorMinScale - 0.01
	maxAbove := CursorMaxScale + 0.01
	tests := []struct {
		name   string
		cursor *TimelineV2Cursor
		time   RationalMilliseconds
		want   string
	}{
		{name: "smoothing", cursor: &TimelineV2Cursor{Smoothing: true, Events: validEvent}, time: RationalMilliseconds{Denominator: 1}, want: "smoothing"},
		{name: "invalid denominator", cursor: &TimelineV2Cursor{Events: validEvent}, time: RationalMilliseconds{Denominator: 0}, want: "denominator"},
		{name: "scale below range", cursor: &TimelineV2Cursor{Scale: &minBelow, Events: validEvent}, time: RationalMilliseconds{Denominator: 1}, want: "scale"},
		{name: "scale above range", cursor: &TimelineV2Cursor{Scale: &maxAbove, Events: validEvent}, time: RationalMilliseconds{Denominator: 1}, want: "scale"},
		{name: "unordered events", cursor: &TimelineV2Cursor{Events: []TimelineV2CursorEvent{{TimeMS: 10, X: 0, Y: 0}, {TimeMS: 9, X: 1, Y: 1}}}, time: RationalMilliseconds{Denominator: 1}, want: "ordered"},
		{name: "negative event", cursor: &TimelineV2Cursor{Events: []TimelineV2CursorEvent{{TimeMS: -1, X: 0, Y: 0}}}, time: RationalMilliseconds{Denominator: 1}, want: "negative"},
		{name: "non-finite coordinate", cursor: &TimelineV2Cursor{Events: []TimelineV2CursorEvent{{TimeMS: 0, X: math.Inf(1), Y: 0}}}, time: RationalMilliseconds{Denominator: 1}, want: "finite"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, err := EvaluateCursorState(test.cursor, test.time)
			if err == nil || state != nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("state=%+v err=%v, want error containing %q", state, err, test.want)
			}
		})
	}
}
