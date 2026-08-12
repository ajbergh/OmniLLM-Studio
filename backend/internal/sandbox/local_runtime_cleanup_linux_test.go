//go:build linux

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRuntimeDestroyIsIdempotent(t *testing.T) {
	scratchRoot := t.TempDir()
	scratch := filepath.Join(scratchRoot, "session")
	if err := os.Mkdir(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := &LocalRuntime{
		sessions: map[string]localRuntimeSession{
			"runtime-1": {id: "runtime-1", scratch: scratch},
		},
		active: make(map[string]context.CancelFunc),
	}
	if err := runtime.Destroy(context.Background(), "runtime-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("scratch still exists after destroy: %v", err)
	}
	if err := runtime.Destroy(context.Background(), "runtime-1"); err != nil {
		t.Fatalf("second Destroy() error = %v", err)
	}
}
