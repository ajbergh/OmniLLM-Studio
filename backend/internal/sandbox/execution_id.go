package sandbox

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const executionIDPrefix = "exec_"

// NewExecutionID returns a canonical caller-known execution reference suitable
// for passing in ExecRequest.ExecutionID before synchronous Exec begins.
func NewExecutionID() string {
	return executionIDPrefix + uuid.NewString()
}

func validateExecutionID(value string) error {
	if value == "" {
		return fmt.Errorf("sandbox execution id is required")
	}
	if strings.TrimSpace(value) != value || len(value) != len(executionIDPrefix)+36 || !strings.HasPrefix(value, executionIDPrefix) {
		return fmt.Errorf("invalid sandbox execution id")
	}
	suffix := strings.TrimPrefix(value, executionIDPrefix)
	parsed, err := uuid.Parse(suffix)
	if err != nil || parsed.String() != suffix {
		return fmt.Errorf("invalid sandbox execution id")
	}
	return nil
}

func executionIDOrNew(value string) (string, error) {
	if value == "" {
		return NewExecutionID(), nil
	}
	if err := validateExecutionID(value); err != nil {
		return "", err
	}
	return value, nil
}
