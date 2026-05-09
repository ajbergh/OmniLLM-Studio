package mcpclient

import (
	"strings"
	"testing"
)

func TestBuildToolNameUsesProviderSafeUnderscores(t *testing.T) {
	name := BuildToolName("File System", "read.file")
	if name != "mcp_file_system_read_file" {
		t.Fatalf("unexpected tool name: %s", name)
	}
}

func TestBuildToolNameCapsLengthWithStableSuffix(t *testing.T) {
	name := BuildToolName(strings.Repeat("server-", 20), strings.Repeat("tool-", 20))
	if len(name) > maxInternalToolNameLen {
		t.Fatalf("tool name length = %d, want <= %d", len(name), maxInternalToolNameLen)
	}
	if !strings.HasPrefix(name, "mcp_") {
		t.Fatalf("tool name should keep mcp prefix: %s", name)
	}
}

func TestUniqueToolNameAvoidsUsedNames(t *testing.T) {
	used := map[string]bool{"mcp_filesystem_read": true}
	name := uniqueToolName("mcp_filesystem_read", used, nil)
	if name != "mcp_filesystem_read_1" {
		t.Fatalf("unexpected unique name: %s", name)
	}
}
