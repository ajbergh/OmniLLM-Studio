package mcpclient

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

const maxInternalToolNameLen = 64

// BuildToolName returns a provider-safe internal tool name for an MCP tool.
func BuildToolName(serverName, toolName string) string {
	serverSlug := slugIdentifier(serverName)
	toolSlug := slugIdentifier(toolName)
	if serverSlug == "" {
		serverSlug = "server"
	}
	if toolSlug == "" {
		toolSlug = "tool"
	}

	name := "mcp_" + serverSlug + "_" + toolSlug
	if len(name) <= maxInternalToolNameLen {
		return name
	}

	sum := sha1.Sum([]byte(name))
	suffix := "_" + hex.EncodeToString(sum[:])[:8]
	maxBase := maxInternalToolNameLen - len(suffix)
	if maxBase < len("mcp_")+1 {
		return "mcp_" + hex.EncodeToString(sum[:])[:maxInternalToolNameLen-len("mcp_")]
	}

	base := strings.Trim(name[:maxBase], "_")
	if base == "" {
		base = "mcp"
	}
	return base + suffix
}

func slugIdentifier(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false

	for _, r := range value {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		switch {
		case isAlphaNum:
			b.WriteRune(r)
			lastUnderscore = false
		case r == '_' || r == '-' || r == '.' || r == ' ':
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}

	return strings.Trim(b.String(), "_")
}
