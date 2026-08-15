package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestBrokerRejectsUnenforceableRequestedResourceLimits(t *testing.T) {
	tests := []struct {
		name       string
		limits     ResourceLimits
		wantField  string
		enable     func(*RuntimeCapabilities)
	}{
		{
			name:      "memory",
			limits:    ResourceLimits{MemoryBytes: 64 << 20},
			wantField: "memory_bytes",
			enable: func(cap *RuntimeCapabilities) {
				cap.MemoryLimit = true
			},
		},
		{
			name:      "cpu",
			limits:    ResourceLimits{CPUTimeMS: 500},
			wantField: "cpu_time_ms",
			enable: func(cap *RuntimeCapabilities) {
				cap.CPULimit = true
			},
		},
		{
			name:      "process count",
			limits:    ResourceLimits{MaxProcesses: 4},
			wantField: "max_processes",
			enable: func(cap *RuntimeCapabilities) {
				cap.PIDLimit = true
			},
		},
		{
			name:      "disk",
			limits:    ResourceLimits{DiskBytes: 128 << 20},
			wantField: "disk_bytes",
			enable: func(cap *RuntimeCapabilities) {
				cap.DiskLimit = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "quota-test"}}
			broker, err := NewBroker(runtime)
			if err != nil {
				t.Fatal(err)
			}
			owner := OwnerScope{UserID: "user-1"}
			request := CreateRequest{Resources: tt.limits}

			if _, err := broker.Create(context.Background(), owner, request); err == nil || !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("Create() error = %v, want unsupported %s rejection", err, tt.wantField)
			}
			if runtime.created.SessionID != "" {
				t.Fatal("runtime Create must not run when a requested quota is unenforceable")
			}

			t.enable(&runtime.capabilities)
			if _, err := broker.Create(context.Background(), owner, request); err != nil {
				t.Fatalf("Create() with matching capability error = %v", err)
			}
			if runtime.created.Spec.Resources != tt.limits {
				t.Fatalf("runtime resource limits = %#v, want %#v", runtime.created.Spec.Resources, tt.limits)
			}
		})
	}
}

func TestBrokerAllowsExistingBoundedResourcesWithoutNewCapabilityBits(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "bounded-output-runtime"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	limits := ResourceLimits{
		WallTimeMS:       1000,
		MaxStdoutBytes:   4096,
		MaxStderrBytes:   4096,
		MaxArtifactBytes: 8192,
	}
	if _, err := broker.Create(context.Background(), OwnerScope{UserID: "user-1"}, CreateRequest{Resources: limits}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if runtime.created.Spec.Resources != limits {
		t.Fatalf("runtime resource limits = %#v, want %#v", runtime.created.Spec.Resources, limits)
	}
}
