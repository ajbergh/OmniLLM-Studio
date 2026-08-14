package sandbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHTTPRuntimeCarriesKnownExecutionIDThroughInFlightCancel(t *testing.T) {
	execSeen := make(chan ExecRequest, 1)
	cancelSeen := make(chan string, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/capabilities":
			_ = json.NewEncoder(w).Encode(RuntimeCapabilities{Name: "test-runtime", OSIsolation: true})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sandboxes/runtime-1/exec":
			var request ExecRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			execSeen <- request
			<-release
			_ = json.NewEncoder(w).Encode(ExecResult{ExecutionID: request.ExecutionID, ExitCode: 0})
		case r.Method == http.MethodPost && r.URL.Path == "/v2/sandboxes/runtime-1/cancel":
			var request struct {
				ExecutionID string `json:"execution_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			cancelSeen <- request.ExecutionID
			releaseOnce.Do(func() { close(release) })
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	runtime, err := NewHTTPRuntime(context.Background(), server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	executionID := NewExecutionID()
	type outcome struct {
		result *ExecResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := runtime.Exec(context.Background(), "runtime-1", ExecRequest{ExecutionID: executionID, Command: "echo"})
		done <- outcome{result: result, err: err}
	}()

	select {
	case request := <-execSeen:
		if request.ExecutionID != executionID {
			t.Fatalf("worker exec id = %q, want %q", request.ExecutionID, executionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runtime exec request did not arrive")
	}

	if err := runtime.Cancel(context.Background(), "runtime-1", executionID); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-cancelSeen:
		if got != executionID {
			t.Fatalf("cancel execution id = %q, want %q", got, executionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runtime cancel request did not arrive")
	}
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.result == nil || got.result.ExecutionID != executionID {
			t.Fatalf("exec result = %#v", got.result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runtime exec did not finish after cancel request")
	}
}
