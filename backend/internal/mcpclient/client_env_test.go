package mcpclient

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

func TestStdioMCPDoesNotInheritAmbientBackendSecrets(t *testing.T) {
	t.Setenv("OMNILLM_MASTER_KEY", "parent-secret-must-not-leak")
	helper := buildStdioEnvironmentHelper(t)
	server := models.MCPServer{
		ID:        "env-test",
		Name:      "env-test",
		Transport: "stdio",
		Command:   &helper,
		Env: map[string]string{
			"EXPLICIT_MCP_VALUE": "configured-value",
		},
	}
	client := NewClient(server)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer client.Stop(context.Background())

	if got := callEnvironmentTool(t, ctx, client, "OMNILLM_MASTER_KEY"); got != "" {
		t.Fatalf("stdio MCP inherited OMNILLM_MASTER_KEY = %q", got)
	}
	if got := callEnvironmentTool(t, ctx, client, "EXPLICIT_MCP_VALUE"); got != "configured-value" {
		t.Fatalf("explicit MCP environment value = %q, want configured-value", got)
	}
}

func callEnvironmentTool(t *testing.T, ctx context.Context, client *Client, name string) string {
	t.Helper()
	arguments, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CallTool(ctx, "read_env", arguments)
	if err != nil {
		t.Fatalf("CallTool(read_env) error = %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("read_env content = %#v", result.Content)
	}
	text, _ := result.Content[0]["text"].(string)
	return text
}

func buildStdioEnvironmentHelper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	binary := filepath.Join(dir, "mcp-env-helper")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	const program = `package main

import (
    "bufio"
    "encoding/json"
    "os"
)

type request struct {
    JSONRPC string          ` + "`json:\"jsonrpc\"`" + `
    ID      json.RawMessage ` + "`json:\"id,omitempty\"`" + `
    Method  string          ` + "`json:\"method\"`" + `
    Params  json.RawMessage ` + "`json:\"params,omitempty\"`" + `
}

type response struct {
    JSONRPC string          ` + "`json:\"jsonrpc\"`" + `
    ID      json.RawMessage ` + "`json:\"id\"`" + `
    Result  interface{}     ` + "`json:\"result\"`" + `
}

func main() {
    scanner := bufio.NewScanner(os.Stdin)
    encoder := json.NewEncoder(os.Stdout)
    for scanner.Scan() {
        var req request
        if json.Unmarshal(scanner.Bytes(), &req) != nil {
            continue
        }
        if len(req.ID) == 0 {
            continue
        }
        switch req.Method {
        case "initialize":
            _ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{
                "protocolVersion": "2025-06-18",
                "capabilities": map[string]interface{}{},
                "serverInfo": map[string]string{"name": "env-helper", "version": "1"},
            }})
        case "tools/call":
            var params struct {
                Arguments map[string]interface{} ` + "`json:\"arguments\"`" + `
            }
            _ = json.Unmarshal(req.Params, &params)
            name, _ := params.Arguments["name"].(string)
            _ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{
                "content": []map[string]interface{}{{"type": "text", "text": os.Getenv(name)}},
            }})
        default:
            _ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}})
        }
    }
}
`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binary, source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build MCP helper: %v\n%s", err, output)
	}
	return binary
}
