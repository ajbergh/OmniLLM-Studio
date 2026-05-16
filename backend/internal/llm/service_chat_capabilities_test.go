package llm

import "testing"

func TestIsChatCapableProvider(t *testing.T) {
	if !IsChatCapableProvider("openai") {
		t.Fatal("expected openai to be chat-capable")
	}
	if !IsChatCapableProvider(" Gemini ") {
		t.Fatal("expected normalized gemini provider to be chat-capable")
	}
	if IsChatCapableProvider("stable-diffusion") {
		t.Fatal("expected stable-diffusion to be image-only for prompt enhancement")
	}
}
