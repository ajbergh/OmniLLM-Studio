package api

import (
	"strings"
	"testing"
)

func TestCleanEnhancedImagePrompt(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strips label and quotes",
			raw:  `Enhanced prompt: "A cinematic studio portrait with warm rim light."`,
			want: "A cinematic studio portrait with warm rim light.",
		},
		{
			name: "strips code fence",
			raw:  "```text\nA watercolor city street after rain.\n```",
			want: "A watercolor city street after rain.",
		},
		{
			name: "trims instruction label",
			raw:  "Instruction: Replace the sky with soft sunset clouds while keeping the building unchanged.",
			want: "Replace the sky with soft sunset clouds while keeping the building unchanged.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanEnhancedImagePrompt(tt.raw); got != tt.want {
				t.Fatalf("cleanEnhancedImagePrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildImagePromptEnhancementMessagesIncludesModeAndContext(t *testing.T) {
	messages := buildImagePromptEnhancementMessages(imagePromptEnhanceRequest{
		Prompt:                   "make it brighter",
		Mode:                     "edit",
		Size:                     "1024x1024",
		ImageModel:               "gpt-image-2",
		ReferenceImageCount:      2,
		StyleReferenceImageCount: 1,
		HasBaseImage:             true,
	})

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" || !strings.Contains(messages[0].Content, "expert image prompt engineer") {
		t.Fatalf("unexpected system message: %#v", messages[0])
	}
	user := messages[1].Content
	for _, want := range []string{
		"Mode: edit",
		"make it brighter",
		"canvas size 1024x1024",
		"target image model gpt-image-2",
		"2 content reference image(s)",
		"1 style reference image(s)",
		"base image is present",
	} {
		if !strings.Contains(user, want) {
			t.Fatalf("user message missing %q: %s", want, user)
		}
	}
}

func TestBuildImagePromptEnhancementMessagesTruncatesInput(t *testing.T) {
	longPrompt := strings.Repeat("x", maxPromptEnhanceInputChars+50)
	messages := buildImagePromptEnhancementMessages(imagePromptEnhanceRequest{Prompt: longPrompt})
	if strings.Contains(messages[1].Content, strings.Repeat("x", maxPromptEnhanceInputChars+1)) {
		t.Fatal("expected prompt input to be truncated")
	}
}
