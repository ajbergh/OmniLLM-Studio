package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

type addressableWorkerRuntime struct {
	execSeen   chan sandbox.ExecRequest
	cancelSeen chan string
	release    chan struct{}
	once       sync.Once
}

func newAddressableWorkerRuntime() *addressableWorkerRuntime {
	return &addressableWorkerRuntime{execSeen: make(chan sandbox.ExecRequest, 1), cancelSeen: make(chan string, 1), release: make(chan struct{})}
}
func (r *addressableWorkerRuntime) Capabilities() sandbox.RuntimeCapabilities {
	return sandbox.RuntimeCapabilities{Name: "test-runtime"}
}
func (r *addressableWorkerRuntime) Create(context.Context, sandbox.RuntimeCreateRequest) (string, error) {
	return "runtime-1", nil
}
func (r *addressableWorkerRuntime) Exec(_ context.Context, _ string, request sandbox.ExecRequest) (*sandbox.ExecResult, error) {
	r.execSeen <- request
	<-r.release
	return &sandbox.ExecResult{ExecutionID: request.ExecutionID, ExitCode: 0}, nil
}
func (r *addressableWorkerRuntime) Cancel(_ context.Context, _ string, executionID string) error {
	r.cancelSeen <- executionID
	r.once.Do(func() { close(r.release) })
	return nil
}
func (r *addressableWorkerRuntime) Status(context.Context, string) (*sandbox.Status, error) {
	return &sandbox.Status{State: "ready"}, nil
}
func (r *addressableWorkerRuntime) Destroy(context.Context, string) error { return nil }

func TestWorkerPreservesKnownExecutionIDAcrossExecAndCancel(t *testing.T) {
	runtime := newAddressableWorkerRuntime()
	server := httptest.NewServer(authenticated("secret-token", newHandler(runtime)))
	defer server.Close()

	executionID := sandbox.NewExecutionID()
	execBody, err := json.Marshal(sandbox.ExecRequest{ExecutionID: executionID, Command: "echo"})
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result sandbox.ExecResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		req, err := http.NewRequest(http.MethodPost, server.URL+"/v2/sandboxes/runtime-1/exec", bytes.NewReader(execBody))
		if err != nil {
			done <- outcome{err: err}
			return
		}
		req.Header.Set("Authorization", "Bearer secret-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- outcome{err: err}
			return
		}
		defer resp.Body.Close()
		var result sandbox.ExecResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		done <- outcome{result: result, err: err}
	}()

	select {
	case request := <-runtime.execSeen:
		if request.ExecutionID != executionID {
			t.Fatalf("worker Exec id = %q, want %q", request.ExecutionID, executionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker Exec did not start")
	}

	cancelBody, err := json.Marshal(map[string]string{"execution_id": executionID})
	if err != nil {
		t.Fatal(err)
	}
	cancelReq, err := http.NewRequest(http.MethodPost, server.URL+"/v2/sandboxes/runtime-1/cancel", bytes.NewReader(cancelBody))
	if err != nil {
		t.Fatal(err)
	}
	cancelReq.Header.Set("Authorization", "Bearer secret-token")
	cancelReq.Header.Set("Content-Type", "application/json")
	cancelResp, err := http.DefaultClient.Do(cancelReq)
	if err != nil {
		t.Fatal(err)
	}
	cancelResp.Body.Close()
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d", cancelResp.StatusCode)
	}

	select {
	case got := <-runtime.cancelSeen:
		if got != executionID {
			t.Fatalf("worker Cancel id = %q, want %q", got, executionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker Cancel did not arrive")
	}
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.result.ExecutionID != executionID {
			t.Fatalf("worker result id = %q, want %q", got.result.ExecutionID, executionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker Exec did not finish")
	}
}
