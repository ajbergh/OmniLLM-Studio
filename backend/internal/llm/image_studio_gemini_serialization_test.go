package llm

import (
	"encoding/json"
	"testing"
)

func TestGeminiStudioRequestSerializationExplicitGeometry(t *testing.T) {
	body := buildGeminiStudioImageBody(
		ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "room"}},
		imageStudioGeometryResolution{Mode: ImageGeometryExplicit, AspectRatio: "16:9"},
	)
	got, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal Gemini Studio body: %v", err)
	}
	want := "{\"contents\":[{\"parts\":[{\"text\":\"room\"}]}],\"generationConfig\":{\"responseFormat\":{\"image\":{\"aspectRatio\":\"16:9\"}},\"responseModalities\":[\"IMAGE\",\"TEXT\"]}}"
	if string(got) != want {
		t.Fatalf("serialized body = %s\nwant            = %s", got, want)
	}
}
