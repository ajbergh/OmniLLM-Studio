package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchOllamaModelsRejectsRequestControlledBaseURL(t *testing.T) {
	handler := NewProviderHandler(nil)
	request := httptest.NewRequest(http.MethodGet, "/providers/ollama/models?base_url=http://169.254.169.254", nil)
	response := httptest.NewRecorder()

	handler.FetchOllamaModels(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "provider_id is required") {
		t.Fatalf("response body = %q, want provider_id validation error", response.Body.String())
	}
}
