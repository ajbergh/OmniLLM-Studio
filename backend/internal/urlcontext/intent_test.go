package urlcontext

import "testing"

func TestRequiresURLContext(t *testing.T) {
	urls := []string{"https://github.com/ajbergh/OmniLLM-Studio"}

	tests := []struct {
		name         string
		message      string
		forceOnURL   bool
		wantRequired bool
		wantGoal     AnalysisGoal
	}{
		{
			name:         "feature gap review",
			message:      "Review this project and let me know what features are missing? https://github.com/ajbergh/OmniLLM-Studio",
			forceOnURL:   true,
			wantRequired: true,
			wantGoal:     GoalFeatureGapReview,
		},
		{
			name:         "architecture review",
			message:      "Review the architecture of this repo https://github.com/ajbergh/OmniLLM-Studio",
			forceOnURL:   true,
			wantRequired: true,
			wantGoal:     GoalArchitectureReview,
		},
		{
			name:         "security review",
			message:      "Security review this project https://github.com/ajbergh/OmniLLM-Studio",
			forceOnURL:   true,
			wantRequired: true,
			wantGoal:     GoalSecurityReview,
		},
		{
			name:         "summarize",
			message:      "Summarize this article https://example.com/article",
			forceOnURL:   false,
			wantRequired: true,
			wantGoal:     GoalSummarize,
		},
		{
			name:         "explain",
			message:      "Explain this code https://github.com/owner/repo/blob/main/main.go",
			forceOnURL:   false,
			wantRequired: true,
			wantGoal:     GoalExplain,
		},
		{
			name:         "sharing without analysis",
			message:      "This is my website https://example.com",
			forceOnURL:   false,
			wantRequired: false,
		},
		{
			name:         "force on URL with question",
			message:      "What does this do? https://github.com/owner/repo",
			forceOnURL:   true,
			wantRequired: true,
		},
		{
			name:         "no URL",
			message:      "What are the MLB standings?",
			forceOnURL:   true,
			wantRequired: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := urls
			if tt.name == "no URL" {
				u = nil
			}
			got, goal := RequiresURLContext(tt.message, u, tt.forceOnURL)
			if got != tt.wantRequired {
				t.Errorf("RequiresURLContext(%q) required=%v, want %v", tt.message, got, tt.wantRequired)
			}
			if tt.wantGoal != "" && goal != tt.wantGoal {
				t.Errorf("RequiresURLContext(%q) goal=%v, want %v", tt.message, goal, tt.wantGoal)
			}
		})
	}
}
