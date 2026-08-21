package rendercontract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type textStateFixture struct {
	Version      int `json:"version"`
	CanvasHeight int `json:"canvas_height"`
	DefaultCase  struct {
		Input    TimelineV2Text     `json:"input"`
		Expected EvaluatedTextState `json:"expected"`
	} `json:"default_case"`
	AuthoredCase struct {
		Input    TimelineV2Text     `json:"input"`
		Expected EvaluatedTextState `json:"expected"`
	} `json:"authored_case"`
}

func loadTextStateFixture(t *testing.T) textStateFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve text state fixture path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "video-renderer", "test", "fixtures", "text-state-v1.json"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read text state fixture: %v", err)
	}
	var fixture textStateFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode text state fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("fixture version = %d, want 1", fixture.Version)
	}
	return fixture
}

func TestEvaluateTextStateMatchesSharedFixture(t *testing.T) {
	fixture := loadTextStateFixture(t)
	cases := []struct {
		name     string
		input    TimelineV2Text
		expected EvaluatedTextState
	}{
		{"defaults", fixture.DefaultCase.Input, fixture.DefaultCase.Expected},
		{"authored", fixture.AuthoredCase.Input, fixture.AuthoredCase.Expected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := EvaluateTextState(&tc.input, fixture.CanvasHeight)
			if err != nil {
				t.Fatalf("EvaluateTextState: %v", err)
			}
			if state == nil || !reflect.DeepEqual(*state, tc.expected) {
				got, _ := json.Marshal(state)
				want, _ := json.Marshal(tc.expected)
				t.Fatalf("text state\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestEvaluateTextStateFailsClosedOnInvalidAuthoring(t *testing.T) {
	negative := -1.0
	nan := math.NaN()
	zero := 0.0
	cases := []struct {
		name string
		text TimelineV2Text
		want string
	}{
		{"alignment", TimelineV2Text{Text: "x", TextAlign: "justify"}, "text_align"},
		{"vertical", TimelineV2Text{Text: "x", VerticalAlign: "baseline"}, "vertical_align"},
		{"negative-padding", TimelineV2Text{Text: "x", PaddingLeft: &negative}, "padding_left"},
		{"non-finite-spacing", TimelineV2Text{Text: "x", LetterSpacing: &nan}, "letter_spacing"},
		{"non-positive-box", TimelineV2Text{Text: "x", BoxWidth: &zero}, "box_width"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := EvaluateTextState(&tc.text, 360); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestEvaluateTextStateDoesNotInterpretExtensionParams(t *testing.T) {
	state, err := EvaluateTextState(&TimelineV2Text{Text: "x", Params: Metadata{"font_size": 999, "unknown": true}}, 360)
	if err != nil {
		t.Fatalf("EvaluateTextState: %v", err)
	}
	if state.FontSize != 20 {
		t.Fatalf("font size = %d, want preview default 20", state.FontSize)
	}
}

func TestEvaluateTextStateFontFaceProvenance(t *testing.T) {
	cases := []struct {
		name         string
		text         TimelineV2Text
		wantFace     string
		wantResource string
	}{
		{"default", TimelineV2Text{Text: "x"}, TextFontFaceSourceCompositionDefault, ""},
		{"family-only", TimelineV2Text{Text: "x", FontFamily: "Inter"}, TextFontFaceSourceFamilyNameOnly, ""},
		{"resource", TimelineV2Text{Text: "x", FontFamily: "Inter", FontResourceID: "inter-400-normal"}, TextFontFaceSourceFamilyNameOnly, "inter-400-normal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := EvaluateTextState(&tc.text, 360)
			if err != nil {
				t.Fatalf("EvaluateTextState: %v", err)
			}
			if state.FontFaceSource != tc.wantFace {
				t.Fatalf("font_face_source = %q, want %q", state.FontFaceSource, tc.wantFace)
			}
			if state.FontResourceID != tc.wantResource {
				t.Fatalf("font_resource_id = %q, want %q", state.FontResourceID, tc.wantResource)
			}
		})
	}
}

func TestEvaluateTextStateRejectsWhitespaceResourceID(t *testing.T) {
	if _, err := EvaluateTextState(&TimelineV2Text{Text: "x", FontResourceID: " inter-400-normal "}, 360); err == nil || !strings.Contains(err.Error(), "font_resource_id") {
		t.Fatalf("error = %v, want font_resource_id whitespace rejection", err)
	}
}
