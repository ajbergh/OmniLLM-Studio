package api

import (
	"context"
	"fmt"
	"strings"
)

type turnToolMode string

const (
	turnToolModeAuto     turnToolMode = "auto"
	turnToolModeNone     turnToolMode = "none"
	turnToolModeRequired turnToolMode = "required"
	turnToolModeSpecific turnToolMode = "specific"
)

type turnToolSelection struct {
	Mode         turnToolMode
	AllowedTools map[string]struct{}
	RequiredTool string
}

type turnToolSelectionContextKey struct{}

func parseTurnToolSelection(mode string, allowedTools []string, requiredTool string) (turnToolSelection, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = string(turnToolModeAuto)
	}
	selection := turnToolSelection{
		Mode:         turnToolMode(mode),
		AllowedTools: make(map[string]struct{}, len(allowedTools)),
		RequiredTool: strings.TrimSpace(requiredTool),
	}
	switch selection.Mode {
	case turnToolModeAuto, turnToolModeNone, turnToolModeRequired, turnToolModeSpecific:
	default:
		return turnToolSelection{}, fmt.Errorf("tool_mode must be auto, none, required, or specific")
	}
	for _, name := range allowedTools {
		name = strings.TrimSpace(name)
		if name != "" {
			selection.AllowedTools[name] = struct{}{}
		}
	}
	if selection.Mode == turnToolModeSpecific && selection.RequiredTool == "" {
		return turnToolSelection{}, fmt.Errorf("required_tool is required when tool_mode is specific")
	}
	if selection.RequiredTool != "" && len(selection.AllowedTools) > 0 {
		if _, ok := selection.AllowedTools[selection.RequiredTool]; !ok {
			return turnToolSelection{}, fmt.Errorf("required_tool must be included in allowed_tools")
		}
	}
	return selection, nil
}

func (s turnToolSelection) allows(toolName string) bool {
	if s.Mode == turnToolModeNone {
		return false
	}
	if s.Mode == turnToolModeSpecific {
		return toolName == s.RequiredTool
	}
	if s.RequiredTool != "" && s.Mode == turnToolModeRequired {
		return toolName == s.RequiredTool
	}
	if len(s.AllowedTools) == 0 {
		return true
	}
	_, ok := s.AllowedTools[toolName]
	return ok
}

func (s turnToolSelection) directive() string {
	switch s.Mode {
	case turnToolModeRequired:
		if s.RequiredTool != "" {
			return fmt.Sprintf("TOOL MODE: You must call the %s tool before answering. Do not answer from memory when that tool can satisfy the request.", s.RequiredTool)
		}
		return "TOOL MODE: You must call at least one available tool before giving the final answer."
	case turnToolModeSpecific:
		return fmt.Sprintf("TOOL MODE: You must call the %s tool before answering. No other tool is available for this turn.", s.RequiredTool)
	case turnToolModeNone:
		return "TOOL MODE: Tools are disabled for this turn. Do not claim to have used tools or live external capabilities."
	default:
		return ""
	}
}

func contextWithTurnToolSelection(ctx context.Context, selection turnToolSelection) context.Context {
	return context.WithValue(ctx, turnToolSelectionContextKey{}, selection)
}

func turnToolSelectionFromContext(ctx context.Context) turnToolSelection {
	if ctx != nil {
		if selection, ok := ctx.Value(turnToolSelectionContextKey{}).(turnToolSelection); ok {
			return selection
		}
	}
	selection, _ := parseTurnToolSelection("", nil, "")
	return selection
}

func (h *MessageHandler) chatPreflightAllowedForTurn(ctx context.Context, toolName string) bool {
	return turnToolSelectionFromContext(ctx).allows(toolName) && h.chatPreflightAllowed(toolName)
}
