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
	want := "{\"contents\":[{\"parts\":[{\"text\":\"room\"}]}],\"generationConfig\":{\"imageConfig\":{\"aspectRatio\":\"16:9\"},\"responseModalities\":[\"IMAGE\",\"TEXT\"]}}"
	if string(got) != want {
		t.Fatalf("serialized body = %s\nwant            = %s", got, want)
	}
}

func TestGeminiStudioRequestSerializationUsesDocumentedAspectRatioLabel(t *testing.T) {
	body := buildGeminiStudioImageBody(
		ImageStudioRequest{ImageRequest: ImageRequest{Prompt: "ultra wide room"}},
		imageStudioGeometryResolution{Mode: ImageGeometryExplicit, Size: "1344x576", AspectRatio: "7:3"},
	)
	generationConfig := body["generationConfig"].(map[string]interface{})
	imageConfig := generationConfig["imageConfig"].(map[string]interface{})
	if got := imageConfig["aspectRatio"]; got != "21:9" {
		t.Fatalf("aspectRatio = %#v, want documented 21:9 label", got)
	}
}
