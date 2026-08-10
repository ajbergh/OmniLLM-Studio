package gitrepo

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCloneLimitsRequireExplicitValidBudgets(t *testing.T) {
	t.Setenv(RemoteCloneMaxBytesEnv, "")
	t.Setenv(RemoteCloneMaxEntriesEnv, "")
	if _, _, ok := cloneLimitsFromEnvironment(); ok {
		t.Fatal("clone limits unexpectedly valid without explicit budgets")
	}

	t.Setenv(RemoteCloneMaxBytesEnv, "268435456")
	t.Setenv(RemoteCloneMaxEntriesEnv, "25000")
	bytes, entries, ok := cloneLimitsFromEnvironment()
	if !ok || bytes != 268435456 || entries != 25000 {
		t.Fatalf("clone limits = (%d, %d, %v), want (268435456, 25000, true)", bytes, entries, ok)
	}

	t.Setenv(RemoteCloneMaxBytesEnv, "1024")
	if _, _, ok := cloneLimitsFromEnvironment(); ok {
		t.Fatal("clone byte budget below hard minimum was accepted")
	}
	t.Setenv(RemoteCloneMaxBytesEnv, "268435456")
	t.Setenv(RemoteCloneMaxEntriesEnv, "100001")
	if _, _, ok := cloneLimitsFromEnvironment(); ok {
		t.Fatal("clone entry budget above hard maximum was accepted")
	}
}

func TestCloneMutationEnabledRequiresAllGlobalGatesAndBudgets(t *testing.T) {
	configured := map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git", AllowClone: true},
	}
	writable := NewServiceWithWriteAccess(map[string]string{"repo": filepath.Join(t.TempDir(), "repo")}, true)
	readOnly := NewService(map[string]string{"repo": filepath.Join(t.TempDir(), "repo")})

	cases := []struct {
		name       string
		enabled    bool
		cloneGate  bool
		maxBytes   int64
		maxEntries int64
		local      *Service
		want       bool
	}{
		{name: "all enabled", enabled: true, cloneGate: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: writable, want: true},
		{name: "remote disabled", cloneGate: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: writable},
		{name: "clone disabled", enabled: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: writable},
		{name: "missing bytes", enabled: true, cloneGate: true, maxEntries: minRemoteCloneEntries, local: writable},
		{name: "missing entries", enabled: true, cloneGate: true, maxBytes: minRemoteCloneBytes, local: writable},
		{name: "local write disabled", enabled: true, cloneGate: true, maxBytes: minRemoteCloneBytes, maxEntries: minRemoteCloneEntries, local: readOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newRemoteService(configured, tc.enabled, false, nil, nil)
			svc.local = tc.local
			svc.cloneEnabled = tc.cloneGate
			svc.cloneMaxBytes = tc.maxBytes
			svc.cloneMaxEntries = tc.maxEntries
			if got := svc.CloneMutationEnabled(); got != tc.want {
				t.Fatalf("CloneMutationEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCloneDestinationMustBeAbsentWithExistingRealParent(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "clone")
	local := NewServiceWithWriteAccess(map[string]string{"repo": destination}, true)
	svc := newRemoteService(nil, true, false, nil, nil)
	svc.local = local

	got, err := svc.cloneDestination("repo")
	if err != nil || got != destination {
		t.Fatalf("cloneDestination() = (%q, %v), want (%q, nil)", got, err, destination)
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.cloneDestination("repo"); err == nil {
		t.Fatal("existing destination unexpectedly accepted")
	}
}

func TestCloneDestinationRejectsMissingParent(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "missing", "clone")
	local := NewServiceWithWriteAccess(map[string]string{"repo": destination}, true)
	svc := newRemoteService(nil, true, false, nil, nil)
	svc.local = local
	if _, err := svc.cloneDestination("repo"); err == nil {
		t.Fatal("destination with missing parent unexpectedly accepted")
	}
}

func TestValidateClonePackHeader(t *testing.T) {
	valid := clonePackHeader(2, 42)
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(valid))); err != nil {
		t.Fatalf("valid pack header rejected: %v", err)
	}
	invalidMagic := append([]byte(nil), valid...)
	copy(invalidMagic[:4], []byte("NOPE"))
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(invalidMagic))); err == nil {
		t.Fatal("invalid pack magic accepted")
	}
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(clonePackHeader(4, 1)))); err == nil {
		t.Fatal("unsupported pack version accepted")
	}
	if err := validateClonePackHeader(bufio.NewReader(bytes.NewReader(clonePackHeader(2, maxRemoteCloneObjects+1)))); err == nil {
		t.Fatal("oversized pack object count accepted")
	}
}

func TestCloneStorageErrorsDoNotExposePaths(t *testing.T) {
	svc := &RemoteService{}
	if err := svc.safeCloneStorageError("origin", errCloneStorageQuotaExceeded, maxRemoteClonePackBytes); err == nil || !errors.Is(errCloneStorageQuotaExceeded, errCloneStorageQuotaExceeded) {
		t.Fatalf("unexpected storage quota mapping: %v", err)
	}
	message := svc.safeCloneCheckoutError("repo", errCloneEntryQuotaExceeded).Error()
	if bytes.Contains([]byte(message), []byte(t.TempDir())) {
		t.Fatalf("checkout error unexpectedly contains a filesystem path: %q", message)
	}
}

func clonePackHeader(version, objects uint32) []byte {
	header := make([]byte, 12)
	copy(header[:4], []byte("PACK"))
	binary.BigEndian.PutUint32(header[4:8], version)
	binary.BigEndian.PutUint32(header[8:12], objects)
	return header
}
