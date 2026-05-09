package mcpclient

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/db"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

const filesystemServerPackage = "@modelcontextprotocol/server-filesystem@2025.8.21"

func TestRealWorldFilesystemServerClient(t *testing.T) {
	command := realWorldNPXCommand(t)
	root := realWorldFilesystemRoot(t)

	server := realWorldFilesystemServer(command, root)
	client := NewClient(server)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("start real filesystem MCP server: %v", err)
	}
	defer client.Stop(context.Background())

	discovered, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	assertHasMCPTool(t, discovered, "read_file")
	assertHasMCPTool(t, discovered, "list_directory")
	assertHasMCPTool(t, discovered, "write_file")

	readArgs := mustJSON(t, map[string]string{
		"path": filepath.Join(root, "fixture.txt"),
	})
	readResult, err := client.CallTool(ctx, "read_file", readArgs)
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	readContent := NormalizeToolResult(readResult).Content
	if !strings.Contains(readContent, "real world MCP fixture") {
		t.Fatalf("read_file content mismatch: %q", readContent)
	}

	writePath := filepath.Join(root, "written-by-mcp.txt")
	writeArgs := mustJSON(t, map[string]string{
		"path":    writePath,
		"content": "written through a real filesystem MCP server",
	})
	writeResult, err := client.CallTool(ctx, "write_file", writeArgs)
	if err != nil {
		t.Fatalf("write_file: %v", err)
	}
	if normalized := NormalizeToolResult(writeResult); normalized.IsError {
		t.Fatalf("write_file returned MCP error: %s", normalized.Content)
	}

	data, err := os.ReadFile(writePath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != "written through a real filesystem MCP server" {
		t.Fatalf("written file mismatch: %q", string(data))
	}
}

func TestRealWorldFilesystemServerManagerAndExecutor(t *testing.T) {
	command := realWorldNPXCommand(t)
	root := realWorldFilesystemRoot(t)

	database := realWorldTestDB(t)
	permRepo := repository.NewToolPermissionRepo(database)
	mcpRepo := repository.NewMCPServerRepo(database)
	registry := tools.NewRegistry()
	manager := NewManager(mcpRepo, permRepo, registry)

	server, err := mcpRepo.Create(repository.CreateMCPServerInput{
		Name:      "filesystem",
		Transport: "stdio",
		Command:   &command,
		Args:      []string{"-y", filesystemServerPackage, root},
	})
	if err != nil {
		t.Fatalf("create MCP server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := manager.Start(ctx, server.ID); err != nil {
		t.Fatalf("manager start: %v", err)
	}
	defer manager.Stop(context.Background(), server.ID)

	discovered, err := manager.ListTools(server.ID)
	if err != nil {
		t.Fatalf("manager list tools: %v", err)
	}
	readTool := findMCPTool(t, discovered, "read_file")
	if readTool.Policy != "ask" {
		t.Fatalf("default read_file policy = %q, want ask", readTool.Policy)
	}

	executor := tools.NewExecutor(registry, permRepo.PolicyResolver(), 0)
	call := tools.ToolCall{
		ID:   "real-world-read-denied",
		Name: readTool.InternalName,
		Arguments: mustJSON(t, map[string]string{
			"path": filepath.Join(root, "fixture.txt"),
		}),
	}

	blocked := executor.Execute(context.Background(), call)
	if !blocked.IsError || !strings.Contains(blocked.Content, "requires user approval") {
		t.Fatalf("ask policy should block execution, got error=%v content=%q", blocked.IsError, blocked.Content)
	}

	if err := manager.SetToolPolicy(server.ID, readTool.InternalName, "allow"); err != nil {
		t.Fatalf("set tool policy: %v", err)
	}

	allowed := executor.Execute(context.Background(), tools.ToolCall{
		ID:        "real-world-read-allowed",
		Name:      readTool.InternalName,
		Arguments: call.Arguments,
	})
	if allowed.IsError {
		t.Fatalf("allowed tool call failed: %s", allowed.Content)
	}
	if !strings.Contains(allowed.Content, "real world MCP fixture") {
		t.Fatalf("allowed tool content mismatch: %q", allowed.Content)
	}
}

func realWorldNPXCommand(t *testing.T) string {
	t.Helper()
	if os.Getenv("OMNILLM_RUN_REAL_MCP_TESTS") != "1" {
		t.Skip("set OMNILLM_RUN_REAL_MCP_TESTS=1 to run real MCP server integration tests")
	}

	if command := strings.TrimSpace(os.Getenv("OMNILLM_REAL_MCP_NPX")); command != "" {
		return command
	}

	candidates := []string{"npx"}
	if runtime.GOOS == "windows" {
		candidates = []string{"npx.cmd", "npx.exe", "npx"}
	}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	t.Skip("npx was not found; install Node.js/npm or set OMNILLM_REAL_MCP_NPX")
	return ""
}

func realWorldFilesystemRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fixture.txt"), []byte("real world MCP fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

func realWorldFilesystemServer(command, root string) models.MCPServer {
	return models.MCPServer{
		ID:        "real-filesystem",
		Name:      "filesystem",
		Transport: "stdio",
		Command:   &command,
		Args:      []string{"-y", filesystemServerPackage, root},
		Env:       map[string]string{},
	}
}

func realWorldTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close(database)
	})
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return database
}

func assertHasMCPTool(t *testing.T, tools []Tool, name string) {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return
		}
	}
	t.Fatalf("expected MCP tool %q in %v", name, toolNames(tools))
}

func findMCPTool(t *testing.T, tools []models.MCPTool, name string) models.MCPTool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("expected MCP tool %q in %v", name, mcpToolNames(tools))
	return models.MCPTool{}
}

func toolNames(tools []Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func mcpToolNames(tools []models.MCPTool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func mustJSON(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return data
}
