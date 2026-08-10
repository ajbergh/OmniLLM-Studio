package api

import (
	"strings"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func stringPointer(value string) *string { return &value }

func TestValidateCreateMCPServerRejectsUnsafeHTTPURLShapes(t *testing.T) {
	for _, rawURL := range []string{
		"ftp://example.com/mcp",
		"https://user:password@example.com/mcp",
		"https://example.com/mcp#fragment",
		"https:///missing-host",
	} {
		t.Run(rawURL, func(t *testing.T) {
			err := validateCreateMCPServer(repository.CreateMCPServerInput{
				Name:      "remote",
				Transport: "http",
				URL:       stringPointer(rawURL),
			})
			if err == nil {
				t.Fatalf("expected URL %q to be rejected", rawURL)
			}
		})
	}
}

func TestValidateCreateMCPServerAcceptsHTTPS(t *testing.T) {
	err := validateCreateMCPServer(repository.CreateMCPServerInput{
		Name:      "remote",
		Transport: "http",
		URL:       stringPointer("https://mcp.example.com/v1"),
	})
	if err != nil {
		t.Fatalf("expected HTTPS MCP URL to validate: %v", err)
	}
}

func TestValidateUpdateMCPServerRejectsEmbeddedCredentials(t *testing.T) {
	err := validateUpdateMCPServer(repository.UpdateMCPServerInput{
		URL: stringPointer("https://token:secret@example.com/mcp"),
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "credentials") {
		t.Fatalf("expected embedded-credential rejection, got %v", err)
	}
}
