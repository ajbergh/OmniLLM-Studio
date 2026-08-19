//go:build linux

package sandbox

import "testing"

func TestParseLinuxCPUStatUsageUsec(t *testing.T) {
	usage, err := parseLinuxCPUStat([]byte("usage_usec 123456\nuser_usec 100000\nsystem_usec 23456\nnr_periods 4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if usage != 123456 {
		t.Fatalf("usage_usec = %d, want 123456", usage)
	}
}

func TestParseLinuxCPUStatRequiresAggregateUsage(t *testing.T) {
	if _, err := parseLinuxCPUStat([]byte("user_usec 100\nsystem_usec 20\n")); err == nil {
		t.Fatal("cpu.stat without usage_usec unexpectedly succeeded")
	}
}
