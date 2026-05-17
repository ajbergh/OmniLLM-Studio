package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/llm"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

type Service struct {
	llmSvc       *llm.Service
	settingsRepo *repository.SettingsRepo
	providerRepo *repository.ProviderRepo
}

func NewService(llmSvc *llm.Service, settingsRepo *repository.SettingsRepo, providerRepo *repository.ProviderRepo) *Service {
	return &Service{llmSvc: llmSvc, settingsRepo: settingsRepo, providerRepo: providerRepo}
}

func (s *Service) Enabled(ctx context.Context) bool {
	settings, err := s.settingsRepo.GetTyped()
	if err != nil {
		return false
	}
	return settings.RouterEnabled && RouterMode(settings.RouterMode) != RouterModeOff
}

func (s *Service) Route(ctx context.Context, req RouteRequest) (*RouteResponse, error) {
	settings, err := s.settingsRepo.GetTyped()
	if err != nil {
		return nil, err
	}
	mode := RouterMode(firstNonEmpty(string(req.Mode), settings.RouterMode, string(RouterModeSportsOnly)))
	telemetry := RouterTelemetry{
		Enabled:              settings.RouterEnabled,
		Mode:                 mode,
		Provider:             settings.RouterProvider,
		Model:                settings.RouterModel,
		StructuredOutputMode: structuredOutputMode(settings, ""),
	}
	if !settings.RouterEnabled || mode == RouterModeOff {
		telemetry.FallbackUsed = true
		telemetry.FallbackReason = "router_disabled"
		return &RouteResponse{Telemetry: telemetry, FallbackReason: "router_disabled"}, nil
	}
	if strings.TrimSpace(settings.RouterProvider) == "" || strings.TrimSpace(settings.RouterModel) == "" {
		telemetry.FallbackUsed = true
		telemetry.FallbackReason = "missing_provider_or_model"
		return &RouteResponse{Telemetry: telemetry, FallbackReason: "missing_provider_or_model"}, nil
	}
	available := req.AvailableRoutes
	if len(available) == 0 {
		available = []RouteName{RouteSportsLookup, RouteNormalLLM, RouteClarify}
	}
	timeout := time.Duration(settings.RouterTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	routeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	maxTokens := settings.RouterMaxTokens
	if maxTokens <= 0 {
		maxTokens = 600
	}
	temp := settings.RouterTemperature
	llmReq := llm.ChatRequest{
		Provider: settings.RouterProvider,
		Model:    settings.RouterModel,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt(mode, available, time.Now())},
			{Role: "user", Content: userPrompt(req.UserMessage)},
		},
		ResponseFormat: responseFormatForSettings(settings),
		MaxTokens:      &maxTokens,
		Temperature:    &temp,
	}
	telemetry.StructuredOutputMode = structuredOutputMode(settings, llmReq.Provider)
	resp, callErr := s.llmSvc.ChatComplete(routeCtx, llmReq)
	telemetry.LatencyMS = int(time.Since(start).Milliseconds())
	if callErr != nil {
		telemetry.FallbackUsed = true
		telemetry.FallbackReason = "llm_error"
		telemetry.Error = callErr.Error()
		return &RouteResponse{Telemetry: telemetry, FallbackReason: "llm_error"}, nil
	}
	decision, parseErr := ParseDecision(resp.Content)
	if parseErr != nil {
		telemetry.FallbackUsed = true
		telemetry.FallbackReason = fallbackReason(parseErr)
		telemetry.Error = parseErr.Error()
		return &RouteResponse{Telemetry: telemetry, FallbackReason: telemetry.FallbackReason}, nil
	}
	telemetry.Confidence = decision.Confidence
	telemetry.Route = decision.Route
	validateErr := ValidateDecision(decision, settings, available)
	if validateErr != nil {
		telemetry.FallbackUsed = true
		telemetry.FallbackReason = fallbackReason(validateErr)
		telemetry.Error = validateErr.Error()
		return &RouteResponse{Decision: decision, Telemetry: telemetry, FallbackReason: telemetry.FallbackReason}, nil
	}
	telemetry.Validated = true
	return &RouteResponse{Decision: decision, Telemetry: telemetry, Valid: true}, nil
}

func (s *Service) Suggestions(ctx context.Context, providerNameOrID string) (*SuggestionsResponse, error) {
	providers, err := s.providerRepo.List()
	if err != nil {
		return nil, err
	}
	target := strings.TrimSpace(providerNameOrID)
	for _, provider := range providers {
		if target == "" || provider.ID == target || strings.EqualFold(provider.Name, target) || strings.EqualFold(provider.Type, target) {
			resp := SuggestionsForProvider(provider)
			return &resp, nil
		}
	}
	return nil, fmt.Errorf("provider not found")
}

func responseFormatForSettings(settings models.AppSettings) interface{} {
	switch strings.ToLower(strings.TrimSpace(settings.RouterStructuredOutputMode)) {
	case "", "auto", "json_schema":
		return JSONSchemaResponseFormat()
	case "json_object":
		return JSONObjectResponseFormat()
	case "prompted_json", "none":
		return nil
	default:
		return JSONSchemaResponseFormat()
	}
}

func structuredOutputMode(settings models.AppSettings, provider string) string {
	mode := strings.ToLower(strings.TrimSpace(settings.RouterStructuredOutputMode))
	if mode == "" || mode == "auto" {
		return "json_schema"
	}
	return mode
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func IsFallbackResponse(resp *RouteResponse) bool {
	return resp == nil || !resp.Valid || resp.Telemetry.FallbackUsed || resp.FallbackReason != ""
}

func IsNormalLLM(decision RouterDecision) bool {
	return decision.Route == RouteNormalLLM || decision.Route == RouteNone
}

func IsRouterFailure(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
