// Package codesandbox preserves the historical composition-root constructor for
// code execution while delegating to the authenticated sandbox protocol v2.
// New sandbox implementation code belongs in internal/sandbox.
package codesandbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

// New creates the ownership-bound protocol-v2 Broker used by code execution.
// The existing OMNILLM_CODE_SANDBOX_URL composition-root setting is retained as
// a compatibility alias for the runtime URL; the endpoint must now implement
// protocol v2 and OMNILLM_SANDBOX_TOKEN is mandatory. The previous unauthenticated
// /v1/execute client is intentionally no longer available through this package.
func New(baseURL string) (*sandbox.Broker, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_URL"))
	}
	if baseURL == "" {
		return nil, fmt.Errorf("sandbox runtime URL is required")
	}
	token := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("OMNILLM_SANDBOX_TOKEN is required for protocol-v2 sandbox execution")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runtime, err := sandbox.NewHTTPRuntime(ctx, baseURL, token)
	if err != nil {
		return nil, err
	}
	return sandbox.NewBroker(runtime)
}
