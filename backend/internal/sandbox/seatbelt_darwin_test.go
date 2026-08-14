//go:build darwin

package sandbox

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	darwinSeatbeltTestAction = "OMNILLM_TEST_DARWIN_SEATBELT_ACTION"
	darwinSeatbeltTestTarget = "OMNILLM_TEST_DARWIN_SEATBELT_TARGET"
)

func TestDarwinSeatbeltNativePrimitive(t *testing.T) {
	allowedRoot := t.TempDir()
	outsideRoot := t.TempDir()
	profile, err := darwinSeatbeltProfile([]string{allowedRoot})
	if err != nil {
		t.Fatalf("darwinSeatbeltProfile() error = %v", err)
	}

	allowedPath := allowedRoot + "/allowed.txt"
	if output, err := runDarwinSeatbeltHelper(t, profile, "write", allowedPath); err != nil {
		t.Fatalf("Seatbelt allowed-root write failed: %v\n%s", err, output)
	}
	if data, err := os.ReadFile(allowedPath); err != nil || string(data) != "seatbelt-ok\n" {
		t.Fatalf("allowed-root output data=%q err=%v", data, err)
	}

	outsidePath := outsideRoot + "/denied.txt"
	if output, err := runDarwinSeatbeltHelper(t, profile, "write", outsidePath); err == nil {
		t.Fatalf("Seatbelt unexpectedly allowed host write outside granted root: %s", output)
	}
	if _, err := os.Stat(outsidePath); !os.IsNotExist(err) {
		t.Fatalf("denied host path exists or returned unexpected error: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	if output, err := runDarwinSeatbeltHelper(t, profile, "dial", listener.Addr().String()); err == nil {
		t.Fatalf("Seatbelt unexpectedly allowed loopback network access: %s", output)
	}
}

func TestDarwinSeatbeltProfileRejectsNonDirectoryWriteRoot(t *testing.T) {
	path := t.TempDir() + "/file.txt"
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := darwinSeatbeltProfile([]string{path}); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("darwinSeatbeltProfile() error = %v, want directory rejection", err)
	}
}

func TestDarwinSeatbeltHelperProcess(t *testing.T) {
	action := os.Getenv(darwinSeatbeltTestAction)
	if action == "" {
		t.Skip("helper process only")
	}
	target := os.Getenv(darwinSeatbeltTestTarget)
	switch action {
	case "write":
		if err := os.WriteFile(target, []byte("seatbelt-ok\n"), 0o600); err != nil {
			t.Fatalf("write %q: %v", target, err)
		}
	case "dial":
		connection, err := net.DialTimeout("tcp", target, 750*time.Millisecond)
		if err != nil {
			t.Fatalf("dial %q: %v", target, err)
		}
		_ = connection.Close()
	default:
		t.Fatalf("unknown helper action %q", action)
	}
}

func runDarwinSeatbeltHelper(t *testing.T, profile, action, target string) (string, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve test executable: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd, err := darwinSeatbeltCommand(ctx, profile, executable, "-test.run=^TestDarwinSeatbeltHelperProcess$")
	if err != nil {
		return "", err
	}
	cmd.Env = append(cmd.Env,
		darwinSeatbeltTestAction+"="+action,
		darwinSeatbeltTestTarget+"="+target,
	)
	output, runErr := cmd.CombinedOutput()
	return string(output), runErr
}
