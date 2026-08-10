package mcpclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/tools"
)

// ValidateHTTPServerURL validates the structural security requirements for a
// remote MCP Streamable HTTP endpoint. Network-address policy is enforced again
// at dial time so DNS changes and rebinding cannot bypass the configured policy.
func ValidateHTTPServerURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid MCP server URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("MCP HTTP URL scheme must be http or https")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("MCP HTTP URL must include a hostname")
	}
	if parsed.User != nil {
		return fmt.Errorf("MCP HTTP URL must not contain embedded credentials")
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("MCP HTTP URL must not contain a fragment")
	}
	return nil
}

func newHTTPTransportClient(server models.MCPServer) *http.Client {
	var client *http.Client
	if server.AllowPrivateNetwork {
		client = &http.Client{Timeout: time.Duration(defaultRequestTimeout) * time.Second}
	} else {
		client = tools.NewSSRFSafeClient(time.Duration(defaultRequestTimeout) * time.Second)
	}
	// MCP endpoints are configured explicitly and may carry encrypted custom
	// authentication headers. Never forward those headers through redirects.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}
