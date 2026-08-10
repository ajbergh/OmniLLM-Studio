package gitrepo

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestRemoteFetchRequiresLocalWriteGate(t *testing.T) {
	configured := map[string]RemoteConfig{
		"origin": {Repository: "repo", URL: "https://example.com/repo.git"},
	}

	readOnlyLocal := NewService(map[string]string{"repo": t.TempDir()})
	readOnlyRemote := newRemoteService(configured, true, false, nil, nil)
	readOnlyRemote.local = readOnlyLocal
	if readOnlyRemote.FetchEnabled() {
		t.Fatal("fetch enabled without OMNILLM_GIT_WRITE_ENABLED-equivalent local gate")
	}

	writableLocal := NewServiceWithWriteAccess(map[string]string{"repo": t.TempDir()}, true)
	writableRemote := newRemoteService(configured, true, false, nil, nil)
	writableRemote.local = writableLocal
	if !writableRemote.FetchEnabled() {
		t.Fatal("fetch not enabled when remote access and local write gate are both enabled")
	}

	disabledRemote := newRemoteService(configured, false, false, nil, nil)
	disabledRemote.local = writableLocal
	if disabledRemote.FetchEnabled() {
		t.Fatal("fetch enabled while remote network access is disabled")
	}
}

func TestRemoteFetchLimitReaderRejectsOversizedPack(t *testing.T) {
	reader := &remoteFetchLimitReader{reader: bytes.NewReader([]byte("abcdef")), remaining: 5}
	_, err := io.ReadAll(reader)
	if !errors.Is(err, errRemoteFetchTooLarge) {
		t.Fatalf("ReadAll error = %v, want %v", err, errRemoteFetchTooLarge)
	}
	if reader.read != 5 {
		t.Fatalf("read = %d, want 5", reader.read)
	}
}

func TestRemoteFetchLimitReaderAllowsExactLimit(t *testing.T) {
	reader := &remoteFetchLimitReader{reader: bytes.NewReader([]byte("abcde")), remaining: 5}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(got) != "abcde" {
		t.Fatalf("ReadAll = %q, want abcde", got)
	}
}

func TestRemoteTrackingReferenceDoesNotUseRawRemoteID(t *testing.T) {
	ref := remoteTrackingReference("remote.with.operator.name", "feature/test")
	if err := ref.Validate(); err != nil {
		t.Fatalf("tracking ref is invalid: %v", err)
	}
	if got := ref.String(); got == "refs/remotes/remote.with.operator.name/feature/test" {
		t.Fatal("tracking ref unexpectedly embeds the raw remote ID")
	}
}
