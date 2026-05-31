package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

var (
	ErrRouterDisabled     = errors.New("router disabled")
	ErrInvalidJSON        = errors.New("invalid router json")
	ErrLowConfidence      = errors.New("router confidence below threshold")
	ErrUnsupportedRoute   = errors.New("unsupported router route")
	ErrMissingSportsParam = errors.New("missing sports params")
)

func ParseDecision(content string) (RouterDecision, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var decision RouterDecision
	if err := json.Unmarshal([]byte(content), &decision); err != nil {
		return RouterDecision{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	decision.Route = RouteName(strings.TrimSpace(string(decision.Route)))
	if decision.RewrittenQuery = strings.TrimSpace(decision.RewrittenQuery); decision.RewrittenQuery == "" {
		decision.RewrittenQuery = ""
	}
	return decision, nil
}

func ValidateDecision(decision RouterDecision, settings models.AppSettings, available []RouteName) error {
	if !settings.RouterEnabled || settings.RouterMode == string(RouterModeOff) {
		return ErrRouterDisabled
	}
	if decision.Confidence < settings.RouterConfidenceThreshold {
		return fmt.Errorf("%w: %.2f < %.2f", ErrLowConfidence, decision.Confidence, settings.RouterConfidenceThreshold)
	}
	if len(available) > 0 {
		ok := false
		for _, route := range available {
			if decision.Route == route {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnsupportedRoute, decision.Route)
		}
	}
	switch RouterMode(settings.RouterMode) {
	case RouterModeSportsOnly:
		if decision.Route != RouteSportsLookup && decision.Route != RouteNormalLLM && decision.Route != RouteClarify && decision.Route != RouteNone {
			return fmt.Errorf("%w: %s", ErrUnsupportedRoute, decision.Route)
		}
	}
	if decision.Route == RouteSportsLookup && decision.Sports == nil {
		return ErrMissingSportsParam
	}
	return nil
}

func fallbackReason(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrInvalidJSON):
		return "invalid_json"
	case errors.Is(err, ErrLowConfidence):
		return "low_confidence"
	case errors.Is(err, ErrUnsupportedRoute):
		return "unsupported_route"
	case errors.Is(err, ErrMissingSportsParam):
		return "missing_sports_params"
	case errors.Is(err, ErrRouterDisabled):
		return "router_disabled"
	default:
		return "router_error"
	}
}
