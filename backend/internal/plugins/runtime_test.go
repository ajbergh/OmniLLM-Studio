package plugins

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

func TestPluginProcessLifecycleAndCancellation(t *testing.T) {
	helper := buildPluginHelper(t)
	manifest := &models.PluginManifest{Name: "test-plugin", Entrypoint: "./" + filepath.Base(helper)}
	process := NewPluginProcess(manifest, filepath.Dir(helper))
	if process == nil {
		t.Fatal("expected plugin process")
	}

	if err := process.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !process.IsRunning() {
		t.Fatal("plugin should be running after Start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := process.Call(ctx, "echo", json.RawMessage(`{"value":"ok"}`))
	if err != nil {
		t.Fatalf("Call(echo) error = %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("decode echo result: %v", err)
	}
	if got["value"] != "ok" {
		t.Fatalf("echo value = %q, want ok", got["value"])
	}

	hangCtx, hangCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer hangCancel()
	if _, err := process.Call(hangCtx, "hang", json.RawMessage(`{}`)); err != context.DeadlineExceeded {
		t.Fatalf("Call(hang) error = %v, want context deadline exceeded", err)
	}

	if err := process.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if process.IsRunning() {
		t.Fatal("plugin should not be running after Stop")
	}
}

func TestNewPluginProcessRejectsEscapingEntrypoint(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "outside-plugin")
	if err := os.WriteFile(outside, []byte("not executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)

	manifest := &models.PluginManifest{Name: "escape", Entrypoint: outside}
	if process := NewPluginProcess(manifest, dir); process != nil {
		t.Fatal("expected escaping entrypoint to be rejected")
	}
}

func buildPluginHelper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	binary := filepath.Join(dir, "plugin-helper")
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
    ID      int             ` + "`json:\"id\"`" + `
    Method  string          ` + "`json:\"method\"`" + `
    Params  json.RawMessage ` + "`json:\"params\"`" + `
}

type response struct {
    JSONRPC string          ` + "`json:\"jsonrpc\"`" + `
    ID      int             ` + "`json:\"id\"`" + `
    Result  json.RawMessage ` + "`json:\"result,omitempty\"`" + `
}

func main() {
    scanner := bufio.NewScanner(os.Stdin)
    encoder := json.NewEncoder(os.Stdout)
    for scanner.Scan() {
        var req request
        if json.Unmarshal(scanner.Bytes(), &req) != nil {
            continue
        }
        switch req.Method {
        case "initialize":
            _ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(` + "`{\"ok\":true}`" + `)})
        case "echo":
            _ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: req.Params})
        case "hang":
            continue
        case "shutdown":
            _ = encoder.Encode(response{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(` + "`{}`" + `)})
            return
        }
    }
}
`
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", binary, source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, output)
	}
	return binary
}
