//go:build linux

package sandbox

import (
	"strings"
	"testing"
)

func TestSandboxCommandModes(t *testing.T) {
	command, args, err := sandboxCommand(ExecRequest{Language: "python", Code: "print(1)"})
	if err != nil {
		t.Fatal(err)
	}
	if command != "python3" || len(args) != 2 || args[0] != "-c" || args[1] != "print(1)" {
		t.Fatalf("python command = %q %#v", command, args)
	}

	command, args, err = sandboxCommand(ExecRequest{Command: "go", Args: []string{"test", "./..."}})
	if err != nil {
		t.Fatal(err)
	}
	if command != "go" || strings.Join(args, " ") != "test ./..." {
		t.Fatalf("terminal command = %q %#v", command, args)
	}

	for _, request := range []ExecRequest{
		{},
		{Language: "python", Code: "print(1)", Command: "echo"},
		{Language: "ruby", Code: "puts 1"},
		{Language: "python"},
	} {
		if _, _, err := sandboxCommand(request); err == nil {
			t.Fatalf("expected %#v to be rejected", request)
		}
	}
}

func TestSandboxDirectoryContainment(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"", "/workspace"},
		{".", "/workspace"},
		{"src", "/workspace/src"},
		{"src/pkg", "/workspace/src/pkg"},
		{"src\\pkg", "/workspace/src/pkg"},
	} {
		got, err := sandboxDirectory(tc.input)
		if err != nil {
			t.Fatalf("sandboxDirectory(%q) error = %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("sandboxDirectory(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	for _, input := range []string{"/etc", "../etc", "src/../../etc", "\\etc"} {
		if _, err := sandboxDirectory(input); err == nil {
			t.Fatalf("expected directory %q to be rejected", input)
		}
	}
}

func TestBoundedOutputTruncatesWithoutShortWrite(t *testing.T) {
	output := newBoundedOutput(5)
	input := []byte("abcdefgh")
	n, err := output.Write(input)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(input) {
		t.Fatalf("Write() n = %d, want %d", n, len(input))
	}
	if got := output.String(); got != "abcde" {
		t.Fatalf("output = %q", got)
	}
	if !output.Truncated() {
		t.Fatal("expected truncated output")
	}
}

func TestLocalRuntimeCapabilitiesDoNotOverclaimResourceLimits(t *testing.T) {
	runtime := &LocalRuntime{}
	capabilities := runtime.Capabilities()
	if !capabilities.OSIsolation || !capabilities.FilesystemIsolation || !capabilities.NetworkIsolation || !capabilities.ProcessTreeIsolation {
		t.Fatalf("isolation capabilities = %#v", capabilities)
	}
	if capabilities.MemoryLimit || capabilities.CPULimit || capabilities.PIDLimit || capabilities.DiskLimit {
		t.Fatalf("runtime overclaims resource limits: %#v", capabilities)
	}
}
