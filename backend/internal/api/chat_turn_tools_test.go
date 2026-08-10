package api

import (
	"context"
	"testing"
)

func TestParseTurnToolSelection(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		allowed  []string
		required string
		wantErr  bool
	}{
		{name: "default auto"},
		{name: "none", mode: "none"},
		{name: "required any", mode: "required"},
		{name: "required named", mode: "required", allowed: []string{"web_search"}, required: "web_search"},
		{name: "specific", mode: "specific", allowed: []string{"sports_lookup"}, required: "sports_lookup"},
		{name: "specific missing", mode: "specific", wantErr: true},
		{name: "required outside allowlist", mode: "required", allowed: []string{"calculator"}, required: "web_search", wantErr: true},
		{name: "invalid", mode: "always", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection, err := parseTurnToolSelection(tt.mode, tt.allowed, tt.required)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTurnToolSelection: %v", err)
			}
			if selection.Mode == "" {
				t.Fatal("selection mode should be normalized")
			}
		})
	}
}

func TestTurnToolSelectionNarrowsButNeverWidensPolicy(t *testing.T) {
	allowOnly, err := parseTurnToolSelection("auto", []string{"calculator"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !allowOnly.allows("calculator") || allowOnly.allows("sports_lookup") {
		t.Fatal("allowed_tools should narrow the per-turn catalog")
	}

	none, _ := parseTurnToolSelection("none", nil, "")
	if none.allows("calculator") {
		t.Fatal("none mode must disable every tool")
	}

	specific, _ := parseTurnToolSelection("specific", []string{"sports_lookup"}, "sports_lookup")
	if !specific.allows("sports_lookup") || specific.allows("calculator") {
		t.Fatal("specific mode must expose only required_tool")
	}
}

func TestChatPreflightAllowedForTurnIntersectsGlobalPolicy(t *testing.T) {
	policy := "allow"
	h := newCapabilityPolicyHandler(&policy)

	selection, _ := parseTurnToolSelection("none", nil, "")
	ctx := contextWithTurnToolSelection(context.Background(), selection)
	if h.chatPreflightAllowedForTurn(ctx, "sports_lookup") {
		t.Fatal("turn-level none must narrow a global allow")
	}

	selection, _ = parseTurnToolSelection("specific", []string{"sports_lookup"}, "sports_lookup")
	ctx = contextWithTurnToolSelection(context.Background(), selection)
	policy = "deny"
	if h.chatPreflightAllowedForTurn(ctx, "sports_lookup") {
		t.Fatal("turn selection must never widen a global deny")
	}
}

func TestTurnToolSelectionContextDefaultsToAuto(t *testing.T) {
	selection := turnToolSelectionFromContext(context.Background())
	if selection.Mode != turnToolModeAuto || !selection.allows("calculator") {
		t.Fatalf("default selection = %#v, want unrestricted auto", selection)
	}
}
