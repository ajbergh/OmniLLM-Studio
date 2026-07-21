package video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeTranscriptionFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "speech.wav")
	if err := os.WriteFile(path, []byte("fixture-audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenAICompatibleTranscriberParsesTimedSegments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/audio/transcriptions" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing authorization header")
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if got := request.FormValue("model"); got != "test-transcriber" {
			t.Fatalf("unexpected model %q", got)
		}
		if got := request.FormValue("language"); got != "en" {
			t.Fatalf("unexpected language %q", got)
		}
		if values := request.MultipartForm.Value["timestamp_granularities[]"]; len(values) != 2 {
			t.Fatalf("expected word and segment timestamp granularities, got %v", values)
		}
		file, _, err := request.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{
			"text":"Hello world",
			"language":"en",
			"duration":1.25,
			"segments":[{
				"start":0.125,
				"end":1.25,
				"text":" Hello world ",
				"speaker":"speaker-1",
				"avg_logprob":-0.12,
				"words":[{"word":"Hello","start":0.125,"end":0.55}]
			}],
			"words":[{"word":"Hello"}],
			"usage":{"seconds":1.25}
		}`)
	}))
	defer server.Close()

	provider := &OpenAICompatibleTranscriber{
		baseURL: server.URL,
		apiKey:  "secret",
		client:  server.Client(),
	}
	result, err := provider.Transcribe(context.Background(), writeTranscriptionFixture(t), TranscriptionRequest{
		Model: "test-transcriber", Language: "en", WordTimestamps: true, Diarization: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Hello world" || result.Language != "en" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected one segment, got %d", len(result.Segments))
	}
	segment := result.Segments[0]
	if segment.StartMS != 125 || segment.EndMS != 1250 || segment.Speaker != "speaker-1" {
		t.Fatalf("unexpected segment: %#v", segment)
	}
	var words []map[string]any
	if err := json.Unmarshal([]byte(segment.WordsJSON), &words); err != nil || len(words) != 1 {
		t.Fatalf("unexpected words JSON %q: %v", segment.WordsJSON, err)
	}
}

func TestOpenAICompatibleTranscriberUsesTranslationRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/audio/translations" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"text":"Hello","language":"es","duration":1}`)
	}))
	defer server.Close()

	provider := &OpenAICompatibleTranscriber{baseURL: server.URL, apiKey: "secret", client: server.Client()}
	result, err := provider.Transcribe(context.Background(), writeTranscriptionFixture(t), TranscriptionRequest{
		Model: "test-transcriber", TranslateTo: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TranslatedLanguage != "en" || len(result.Segments) != 1 || result.Segments[0].EndMS != 1000 {
		t.Fatalf("unexpected translation result: %#v", result)
	}
}

func TestOpenAICompatibleTranscriberReturnsBoundedProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "provider rejected request", http.StatusBadRequest)
	}))
	defer server.Close()
	provider := &OpenAICompatibleTranscriber{baseURL: server.URL, apiKey: "secret", client: server.Client()}
	if _, err := provider.Transcribe(context.Background(), writeTranscriptionFixture(t), TranscriptionRequest{Model: "test"}); err == nil {
		t.Fatal("expected provider error")
	}
}
