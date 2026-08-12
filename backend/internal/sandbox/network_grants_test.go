package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestNetworkGrantEnforcesOperatorDestinationPolicy(t *testing.T) {
	t.Setenv("OMNILLM_SANDBOX_NETWORK_DOMAINS", "registry.npmjs.org,*.golang.org")
	t.Setenv("OMNILLM_SANDBOX_NETWORK_PORTS", "443,8443")
	store := NewNetworkGrantStore()
	owner := OwnerScope{UserID: "user-1", ConversationID: "conversation-1"}

	grant, err := store.Create(owner, []string{"proxy.golang.org", "registry.npmjs.org"}, []int{443}, 5*time.Minute)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.HasPrefix(grant.ID, "sng_") || len(grant.Domains) != 2 || grant.Ports[0] != 443 {
		t.Fatalf("grant = %#v", grant)
	}
	resolved, err := store.Resolve(owner, grant.ID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.ID != grant.ID {
		t.Fatalf("resolved grant = %#v", resolved)
	}

	for _, domains := range [][]string{
		{"localhost"},
		{"127.0.0.1"},
		{"169.254.169.254"},
		{"example.com"},
		{"golang.org"},
	} {
		if _, err := store.Create(owner, domains, []int{443}, time.Minute); err == nil {
			t.Fatalf("expected domains %#v to be rejected", domains)
		}
	}
	if _, err := store.Create(owner, []string{"registry.npmjs.org"}, []int{22}, time.Minute); err == nil {
		t.Fatal("expected unapproved port to be rejected")
	}
}

func TestNetworkGrantIsOwnerBoundAndExpires(t *testing.T) {
	t.Setenv("OMNILLM_SANDBOX_NETWORK_DOMAINS", "example.com")
	store := NewNetworkGrantStore()
	now := time.Date(2026, 8, 12, 14, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	owner := OwnerScope{UserID: "user-1", ConversationID: "conversation-1"}
	grant, err := store.Create(owner, []string{"example.com"}, nil, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(OwnerScope{UserID: "user-2"}, grant.ID); err == nil || !strings.Contains(err.Error(), "owned") {
		t.Fatalf("cross-owner Resolve() error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := store.Resolve(owner, grant.ID); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired Resolve() error = %v", err)
	}
}

func TestNetworkGrantsFailClosedWithoutOperatorPolicy(t *testing.T) {
	t.Setenv("OMNILLM_SANDBOX_NETWORK_DOMAINS", "")
	store := NewNetworkGrantStore()
	_, err := store.Create(OwnerScope{UserID: "user-1"}, []string{"example.com"}, nil, time.Minute)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestRequireCapabilitiesDistinguishesIsolationFromAllowlisting(t *testing.T) {
	capabilities := RuntimeCapabilities{Name: "isolated-only", NetworkIsolation: true}
	if err := requireCapabilities(capabilities, RuntimeRequirements{NetworkIsolation: true}); err != nil {
		t.Fatalf("network isolation requirement failed: %v", err)
	}
	if err := requireCapabilities(capabilities, RuntimeRequirements{NetworkAllowlist: true}); err == nil || !strings.Contains(err.Error(), "network_allowlist") {
		t.Fatalf("network allowlist requirement error = %v", err)
	}
}
