package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

const testSpec = `{
  "openapi":"3.0.3",
  "paths":{
    "/widgets/{id}":{
      "get":{
        "operationId":"getWidget",
        "description":"Read one widget",
        "parameters":[
          {"name":"id","in":"path","required":true,"schema":{"type":"string"}},
          {"name":"verbose","in":"query","schema":{"type":"boolean"}}
        ]
      }
    },
    "/widgets":{
      "post":{
        "operationId":"createWidget",
        "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"name":{"type":"string"}}}}}}
      }
    }
  }
}`

func TestValidateBaseURLRejectsPrivateByDefault(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:8080", "http://localhost:8080", "http://10.0.0.2"} {
		if err := validateBaseURL(raw, false); err == nil {
			t.Fatalf("%s should be rejected without private-network opt-in", raw)
		}
	}
	if err := validateBaseURL("http://127.0.0.1:8080", true); err != nil {
		t.Fatalf("private opt-in should allow local endpoint: %v", err)
	}
	if err := validateBaseURL("file:///etc/passwd", true); err == nil {
		t.Fatal("non-http schemes must be rejected")
	}
}

func TestParseOperationsBuildsReadAndWriteDefinitions(t *testing.T) {
	server := models.OpenAPIServerRuntime{OpenAPIServer: models.OpenAPIServer{ID: "abcdef12", Name: "Widget API", BaseURL: "https://example.com", SpecJSON: testSpec}}
	ops, err := parseOperations(server)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("operations=%d, want 2", len(ops))
	}
	registry := tools.NewRegistry()
	for _, op := range ops {
		registry.MustRegister(&operationTool{server: server, operation: op})
	}
	var sawRead, sawWrite bool
	for _, def := range registry.List() {
		if !strings.HasPrefix(def.Name, "openapi_widget_api_") {
			continue
		}
		if def.ReadOnly {
			sawRead = true
		} else if def.SideEffecting && def.Risk == tools.RiskHigh {
			sawWrite = true
		}
	}
	if !sawRead || !sawWrite {
		t.Fatalf("generated definitions missing read/write safety metadata: read=%v write=%v", sawRead, sawWrite)
	}
}

func TestOperationToolExecutesBoundedConfiguredHost(t *testing.T) {
	serverHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/widgets/abc" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if got := r.URL.Query().Get("verbose"); got != "true" {
			t.Errorf("verbose=%s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer serverHTTP.Close()
	server := models.OpenAPIServerRuntime{OpenAPIServer: models.OpenAPIServer{ID: "abcdef12", OwnerUserID: "user-1", Name: "Widget API", BaseURL: serverHTTP.URL, SpecJSON: testSpec, AllowPrivateNetwork: true}}
	ops, err := parseOperations(server)
	if err != nil {
		t.Fatal(err)
	}
	var get operation
	for _, op := range ops {
		if op.OperationID == "getWidget" {
			get = op
		}
	}
	tool := &operationTool{server: server, operation: get}
	args := json.RawMessage(`{"id":"abc","verbose":true}`)
	if err := tool.Validate(args); err != nil {
		t.Fatal(err)
	}
	ctx := tools.ContextWithInvocationScope(context.Background(), tools.InvocationScope{UserID: "user-1"})
	result, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || !strings.Contains(result.Content, "\"ok\":true") {
		t.Fatalf("result=%#v", result)
	}
}

func TestOperationToolRejectsDifferentOwner(t *testing.T) {
	server := models.OpenAPIServerRuntime{OpenAPIServer: models.OpenAPIServer{ID: "abcdef12", OwnerUserID: "user-1", Name: "Widget API", BaseURL: "https://example.com", SpecJSON: testSpec}}
	ops, err := parseOperations(server)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tools.ContextWithInvocationScope(context.Background(), tools.InvocationScope{UserID: "user-2"})
	_, err = (&operationTool{server: server, operation: ops[0]}).Execute(ctx, json.RawMessage(`{"id":"abc"}`))
	if err == nil {
		t.Fatal("cross-owner OpenAPI execution must be rejected")
	}
}
