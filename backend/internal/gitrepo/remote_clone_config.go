package gitrepo

import (
	"os"
	"strconv"
	"strings"
)

const (
	// RemoteCloneEnabledEnv is the independent process-wide clone gate. Clone
	// also requires remote access, local Git writes, a per-remote allow_clone
	// opt-in, and valid explicit byte/entry budgets.
	RemoteCloneEnabledEnv = "OMNILLM_GIT_REMOTE_CLONE_ENABLED"
	// RemoteCloneMaxBytesEnv is the total logical byte budget shared by .git and
	// worktree writes during one clone.
	RemoteCloneMaxBytesEnv = "OMNILLM_GIT_CLONE_MAX_BYTES"
	// RemoteCloneMaxEntriesEnv bounds cumulative files/directories/symlinks
	// created during one clone so empty-tree metadata cannot exhaust inodes.
	RemoteCloneMaxEntriesEnv = "OMNILLM_GIT_CLONE_MAX_ENTRIES"

	minRemoteCloneBytes   int64 = 1 << 20
	maxRemoteCloneBytes   int64 = 1 << 30
	minRemoteCloneEntries int64 = 128
	maxRemoteCloneEntries int64 = 100_000
	maxRemoteClonePackBytes int64 = 128 << 20
)

func cloneLimitsFromEnvironment() (maxBytes, maxEntries int64, ok bool) {
	bytes, bytesOK := boundedPositiveInt64Environment(RemoteCloneMaxBytesEnv, minRemoteCloneBytes, maxRemoteCloneBytes)
	entries, entriesOK := boundedPositiveInt64Environment(RemoteCloneMaxEntriesEnv, minRemoteCloneEntries, maxRemoteCloneEntries)
	if !bytesOK || !entriesOK {
		return 0, 0, false
	}
	return bytes, entries, true
}

func boundedPositiveInt64Environment(name string, minimum, maximum int64) (int64, bool) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, false
	}
	return value, true
}
