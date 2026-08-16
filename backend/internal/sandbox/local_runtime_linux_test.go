//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSandboxCommandModes(t *testing.T) {
	command, args, err := sandboxCommand(ExecRequest{Language: "python", Code: "print(1)"})
	if err != nil {
		t.Fatal(err)
	}
	wantPythonArgs := []string{"-I", "-S", "-c", "print(1)"}
	if command != "python3" || !reflect.DeepEqual(args, wantPythonArgs) {
		t.Fatalf("python command = %q %#v, want python3 %#v", command, args, wantPythonArgs)
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

func TestLocalRuntimeCapabilitiesDoNotOverclaimResourceOrNetworkLimits(t *testing.T) {
	runtime := &LocalRuntime{}
	capabilities := runtime.Capabilities()
	if !capabilities.OSIsolation || !capabilities.FilesystemIsolation || !capabilities.NetworkIsolation || !capabilities.ProcessTreeIsolation {
		t.Fatalf("isolation capabilities = %#v", capabilities)
	}
	if capabilities.NetworkAllowlist || capabilities.MemoryLimit || capabilities.CPULimit || capabilities.PIDLimit || capabilities.DiskLimit {
		t.Fatalf("runtime overclaims controls: %#v", capabilities)
	}
}

func TestLocalRuntimeAdvertisesPIDLimitOnlyWithDelegatedManager(t *testing.T) {
	runtime := &LocalRuntime{pidCgroup: &linuxPIDCgroupManager{root: "/delegated"}}
	capabilities := runtime.Capabilities()
	if !capabilities.PIDLimit {
		t.Fatalf("runtime did not advertise initialized PID control: %#v", capabilities)
	}
	if capabilities.NetworkAllowlist || capabilities.MemoryLimit || capabilities.CPULimit || capabilities.DiskLimit {
		t.Fatalf("runtime overclaims unrelated controls: %#v", capabilities)
	}
}

func TestValidateRuntimeMountsRequiresTrustedResolution(t *testing.T) {
	root := t.TempDir()
	owner := OwnerScope{UserID: "user-1"}
	request := RuntimeCreateRequest{
		SessionID: "sbx-test",
		Owner:     owner,
		Spec: CreateRequest{Mounts: []WorkspaceMount{{
			WorkspaceID: "workspace-1",
			Mode:        MountReadOnly,
		}}},
		ResolvedMounts: []RuntimeMount{{
			WorkspaceID: "workspace-1",
			SourcePath:  root,
			Mode:        MountReadOnly,
		}},
	}
	mounts, err := validateRuntimeMounts(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(mounts) != 1 || mounts[0].SourcePath != root {
		t.Fatalf("resolved mounts = %#v", mounts)
	}

	request.ResolvedMounts[0].WorkspaceID = "other"
	if _, err := validateRuntimeMounts(request); err == nil {
		t.Fatal("expected workspace identity mismatch to be rejected")
	}
}

func TestRuntimeWorkspaceMountArgsNarrowNoDeleteAndProtectGit(t *testing.T) {
	scratch := t.TempDir()
	root := t.TempDir()

	args, mode, err := runtimeWorkspaceMountArgs(localRuntimeSession{scratch: scratch})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"--bind", scratch, "/workspace"}) || mode != "ephemeral" {
		t.Fatalf("ephemeral mount args = %#v mode=%q", args, mode)
	}

	args, mode, err = runtimeWorkspaceMountArgs(localRuntimeSession{mounts: []RuntimeMount{{
		WorkspaceID: "workspace-1",
		SourcePath:  root,
		Mode:        MountReadWriteNoDelete,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"--ro-bind", root, "/workspace"}) || mode != string(MountReadOnly) {
		t.Fatalf("no-delete mount args = %#v mode=%q", args, mode)
	}

	gitPath := filepath.Join(root, ".git")
	if err := os.Mkdir(gitPath, 0o700); err != nil {
		t.Fatal(err)
	}
	args, mode, err = runtimeWorkspaceMountArgs(localRuntimeSession{mounts: []RuntimeMount{{
		WorkspaceID: "workspace-1",
		SourcePath:  root,
		Mode:        MountReadWrite,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--bind", root, "/workspace", "--ro-bind", gitPath, "/workspace/.git"}
	if !reflect.DeepEqual(args, want) || mode != string(MountReadWrite) {
		t.Fatalf("read-write mount args = %#v mode=%q, want %#v", args, mode, want)
	}
}
