package codesandbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientExecutesOnlyExternalHTTPContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/execute" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"s1","stdout":"ok\n","exit_code":0}`))
	}))
	defer server.Close()
	client, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	out, err := client.Execute(context.Background(), ExecuteRequest{Language: "python", Code: "print('ok')", TimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if out.SessionID != "s1" || !strings.Contains(out.Stdout, "ok") || out.ExitCode != 0 {
		t.Fatalf("unexpected response %#v", out)
	}
}

func TestClientRejectsInvalidSandboxURL(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/sandbox", "https://user:pass@example.com"} {
		if _, err := New(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}
