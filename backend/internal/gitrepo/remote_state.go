package gitrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
)

// remoteBranchStateDigest fingerprints the complete advertised branch namespace.
// It intentionally excludes tags and other refs because guarded branch
// publication only needs to bind approval to branch existence and branch heads.
func remoteBranchStateDigest(advertised *packp.AdvRefs) string {
	entries := make([]string, 0)
	if advertised != nil {
		entries = make([]string, 0, len(advertised.References))
		for name, hash := range advertised.References {
			if !strings.HasPrefix(name, "refs/heads/") || hash == plumbing.ZeroHash {
				continue
			}
			entries = append(entries, name+"="+hash.String())
		}
	}
	sort.Strings(entries)
	hasher := sha256.New()
	for _, entry := range entries {
		_, _ = hasher.Write([]byte(entry))
		_, _ = hasher.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func validRemoteStateDigest(raw string) bool {
	raw = strings.TrimSpace(raw)
	if len(raw) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}
