package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ExtensionSandboxMode controls the persistent stdio plugin/MCP process
// confinement policy. Auto preserves compatibility on platforms/deployments
// without a configured native sandbox, while automatically using confinement
// when the platform runtime is configured. Required fails closed. Off is an
// explicit compatibility override and still uses the sanitized host runner.
type ExtensionSandboxMode string

const (
	ExtensionSandboxAuto     ExtensionSandboxMode = "auto"
	ExtensionSandboxRequired ExtensionSandboxMode = "required"
	ExtensionSandboxOff      ExtensionSandboxMode = "off"
)

// ExtensionCommandRunner is the default runner for persistent local plugin and
// stdio MCP processes. Unlike the concrete HostCommandRunner, it can insert
// platform-native OS confinement while retaining the existing streaming
// stdin/stdout lifecycle.
type ExtensionCommandRunner struct{}

// NewExtensionCommandRunner returns the persistent extension process runner.
func NewExtensionCommandRunner() ExtensionCommandRunner { return ExtensionCommandRunner{} }

func (ExtensionCommandRunner) CommandContext(ctx context.Context, spec ProcessSpec) (*exec.Cmd, error) {
	mode, err := extensionSandboxModeFromEnvironment()
	if err != nil {
		return nil, err
	}
	if mode == ExtensionSandboxOff {
		return (HostCommandRunner{}).CommandContext(ctx, spec)
	}
	return platformExtensionCommandContext(ctx, spec, mode)
}

func extensionSandboxModeFromEnvironment() (ExtensionSandboxMode, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("OMNILLM_EXTENSION_SANDBOX_MODE")))
	if value == "" {
		return ExtensionSandboxAuto, nil
	}
	mode := ExtensionSandboxMode(value)
	switch mode {
	case ExtensionSandboxAuto, ExtensionSandboxRequired, ExtensionSandboxOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid OMNILLM_EXTENSION_SANDBOX_MODE %q", value)
	}
}

// validateExtensionEnvironment applies the arbitrary-sandbox secret policy only
// when native extension confinement is active/required. Auto compatibility mode
// without a native sandbox keeps the previous contract: explicit configured MCP
// or plugin environment values are allowed while ambient backend secrets remain
// stripped by HostCommandRunner.
func validateExtensionEnvironment(spec ProcessSpec) error {
	allowSensitive := strings.EqualFold(strings.TrimSpace(os.Getenv("OMNILLM_EXTENSION_ALLOW_SECRET_ENV")), "true")
	for key, value := range spec.Env {
		if allowSensitive {
			if err := validateEnvironmentEntry(key, value); err != nil {
				return err
			}
			continue
		}
		if err := validateSandboxEnvironmentEntry(key, value); err != nil {
			return fmt.Errorf("extension environment: %w", err)
		}
	}
	return nil
}
