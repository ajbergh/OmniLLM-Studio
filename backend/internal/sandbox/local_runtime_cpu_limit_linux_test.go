//go:build linux

package sandbox

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const linuxRuntimeCPULimitMS = 150

// TestLinuxLocalRuntimeCPUQuotaNative exercises the actual Bubblewrap runtime,
// not only the cgroup primitive. The tiny test rootfs contains a statically
// linked BusyBox shell so no host-root bind is needed. Two background CPU
// burners prove aggregate descendant accounting while the execution result must
// distinguish CPU quota exhaustion from caller cancellation and wall timeout.
// Public CPULimit remains false until this native path is independently green.
func TestLinuxLocalRuntimeCPUQuotaNative(t *testing.T) {
	root := os.Getenv(linuxCgroupTestRootEnv)
	if root == "" {
		t.Skip("native delegated cgroup-v2 root not configured")
	}
	busybox, err := exec.LookPath("busybox")
	if err != nil {
		t.Skip("busybox-static is required for native Linux runtime CPU assurance")
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Fatal(err)
	}

	rootFS := t.TempDir()
	binDir := filepath.Join(rootFS, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	busyboxTarget := filepath.Join(binDir, "busybox")
	if err := copyLinuxRuntimeTestFile(busybox, busyboxTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("busybox", filepath.Join(binDir, "sh")); err != nil {
		t.Fatal(err)
	}

	runtimeValue, err := NewLocalRuntime(LocalRuntimeConfig{
		RootFS:     rootFS,
		BwrapPath:  bwrap,
		CgroupRoot: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The enforcement path is intentionally hidden until this native test and
	// its final PR gates are green.
	if runtimeValue.Capabilities().CPULimit {
		t.Fatal("Linux runtime advertised CPULimit before native promotion gate")
	}

	runtimeID, err := runtimeValue.Create(context.Background(), RuntimeCreateRequest{
		SessionID: "sbx_test_cpu_limit",
		Owner:     OwnerScope{UserID: "test-user"},
		Spec: CreateRequest{
			Network: NetworkPolicy{Mode: NetworkNone},
			Resources: ResourceLimits{
				CPUTimeMS:      linuxRuntimeCPULimitMS,
				WallTimeMS:     5_000,
				MaxStdoutBytes: 16 << 10,
				MaxStderrBytes: 16 << 10,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeValue.Destroy(context.Background(), runtimeID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := runtimeValue.Exec(ctx, runtimeID, ExecRequest{
		Language: "shell",
		Code:     `(while :; do :; done) & (while :; do :; done) & wait`,
	})
	if err != nil {
		t.Fatalf("execute CPU-limited runtime: %v", err)
	}
	if got, ok := result.Metadata["cpu_limit_enforced"].(bool); !ok || !got {
		t.Fatalf("cpu_limit_enforced = %#v", result.Metadata["cpu_limit_enforced"])
	}
	if got, ok := result.Metadata["cpu_limit_exceeded"].(bool); !ok || !got {
		t.Fatalf("cpu_limit_exceeded = %#v metadata=%#v", result.Metadata["cpu_limit_exceeded"], result.Metadata)
	}
	if got := result.Metadata["termination_reason"]; got != "cpu_quota_exceeded" {
		t.Fatalf("termination_reason = %#v, want cpu_quota_exceeded", got)
	}
	usage, ok := result.Metadata["cpu_usage_us"].(uint64)
	if !ok {
		t.Fatalf("cpu_usage_us type/value = %#v", result.Metadata["cpu_usage_us"])
	}
	if usage < uint64(linuxRuntimeCPULimitMS*1000) {
		t.Fatalf("cpu_usage_us = %d, want at least %d", usage, linuxRuntimeCPULimitMS*1000)
	}
	if ctx.Err() != nil {
		t.Fatalf("CPU quota was misreported as caller timeout: %v", ctx.Err())
	}
}

func copyLinuxRuntimeTestFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
