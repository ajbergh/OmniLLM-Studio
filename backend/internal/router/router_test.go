package router

import (
	"errors"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestParseDecisionValidJSON(t *testing.T) {
	decision, err := ParseDecision(`{"route":"sports_lookup","confidence":0.96,"requires_generation_llm":false,"sports":{"intent":"standings","league":"MLB"}}`)
	if err != nil {
		t.Fatalf("ParseDecision returned error: %v", err)
	}
	if decision.Route != RouteSportsLookup {
		t.Fatalf("route = %q, want %q", decision.Route, RouteSportsLookup)
	}
	if decision.Sports == nil || decision.Sports.Intent != "standings" {
		t.Fatalf("sports params not parsed: %#v", decision.Sports)
	}
}

func TestParseDecisionInvalidJSON(t *testing.T) {
	_, err := ParseDecision(`not json`)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("err = %v, want ErrInvalidJSON", err)
	}
}

func TestValidateDecisionLowConfidence(t *testing.T) {
	settings := models.DefaultAppSettings()
	settings.RouterEnabled = true
	settings.RouterConfidenceThreshold = 0.75
	err := ValidateDecision(RouterDecision{
		Route:                 RouteSportsLookup,
		Confidence:            0.5,
		RequiresGenerationLLM: false,
		Sports:                &SportsRouteParams{Intent: "standings", League: "MLB"},
	}, settings, []RouteName{RouteSportsLookup, RouteNormalLLM})
	if !errors.Is(err, ErrLowConfidence) {
		t.Fatalf("err = %v, want ErrLowConfidence", err)
	}
}

func TestValidateDecisionSportsOnlyRejectsFutureRoute(t *testing.T) {
	settings := models.DefaultAppSettings()
	settings.RouterEnabled = true
	err := ValidateDecision(RouterDecision{
		Route:                 RouteImageGeneration,
		Confidence:            0.9,
		RequiresGenerationLLM: false,
	}, settings, []RouteName{RouteSportsLookup, RouteNormalLLM})
	if !errors.Is(err, ErrUnsupportedRoute) {
		t.Fatalf("err = %v, want ErrUnsupportedRoute", err)
	}
}
