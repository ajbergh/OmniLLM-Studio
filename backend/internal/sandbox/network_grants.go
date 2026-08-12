package sandbox

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var dnsNamePattern = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

// NetworkGrant is a short-lived, owner-bound approval reference. It contains no
// credentials and does not itself provide connectivity; a runtime must also
// advertise enforceable network allowlisting.
type NetworkGrant struct {
	ID        string     `json:"id"`
	Owner     OwnerScope `json:"-"`
	Domains   []string   `json:"domains"`
	Ports     []int      `json:"ports"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
}

type NetworkGrantStore struct {
	mu     sync.RWMutex
	grants map[string]NetworkGrant
	now    func() time.Time
}

func NewNetworkGrantStore() *NetworkGrantStore {
	return &NetworkGrantStore{grants: make(map[string]NetworkGrant), now: time.Now}
}

func (s *NetworkGrantStore) Create(owner OwnerScope, domains []string, ports []int, ttl time.Duration) (*NetworkGrant, error) {
	if owner.Empty() || owner.UserID == "" {
		return nil, fmt.Errorf("network grant owner is required")
	}
	if ttl <= 0 || ttl > 30*time.Minute {
		ttl = 15 * time.Minute
	}
	domains, err := validateOperatorNetworkDestinations(domains)
	if err != nil {
		return nil, err
	}
	ports, err = validateOperatorNetworkPorts(ports)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	grant := NetworkGrant{ID: "sng_" + uuid.NewString(), Owner: owner, Domains: domains, Ports: ports, CreatedAt: now, ExpiresAt: now.Add(ttl)}
	s.mu.Lock()
	s.grants[grant.ID] = grant
	s.mu.Unlock()
	copy := grant
	return &copy, nil
}

func (s *NetworkGrantStore) Resolve(owner OwnerScope, id string) (*NetworkGrant, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("network grant id is required")
	}
	s.mu.RLock()
	grant, ok := s.grants[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("network grant not found")
	}
	if !grant.Owner.Equal(owner) {
		return nil, fmt.Errorf("network grant is not owned by the current scope")
	}
	if !s.now().UTC().Before(grant.ExpiresAt) {
		return nil, fmt.Errorf("network grant has expired")
	}
	copy := grant
	copy.Domains = append([]string(nil), grant.Domains...)
	copy.Ports = append([]int(nil), grant.Ports...)
	return &copy, nil
}

var defaultNetworkGrants = NewNetworkGrantStore()

func DefaultNetworkGrantStore() *NetworkGrantStore { return defaultNetworkGrants }

func validateOperatorNetworkDestinations(domains []string) ([]string, error) {
	allowed := parseNetworkDomainPolicy(os.Getenv("OMNILLM_SANDBOX_NETWORK_DOMAINS"))
	if len(allowed) == 0 {
		return nil, fmt.Errorf("sandbox network destinations are disabled by operator policy")
	}
	if len(domains) == 0 || len(domains) > 16 {
		return nil, fmt.Errorf("network grant requires 1-16 domains")
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(domains))
	for _, raw := range domains {
		domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if domain == "localhost" || net.ParseIP(domain) != nil || !dnsNamePattern.MatchString(domain) {
			return nil, fmt.Errorf("network destination %q is not an allowed DNS hostname", raw)
		}
		if !domainAllowedByOperator(domain, allowed) {
			return nil, fmt.Errorf("network destination %q is not permitted by operator policy", domain)
		}
		if _, exists := seen[domain]; !exists {
			seen[domain] = struct{}{}
			out = append(out, domain)
		}
	}
	sort.Strings(out)
	return out, nil
}

func parseNetworkDomainPolicy(raw string) []string {
	var out []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(entry), "."))
		if entry != "" {
			out = append(out, entry)
		}
	}
	return out
}

func domainAllowedByOperator(domain string, allowed []string) bool {
	for _, rule := range allowed {
		if strings.HasPrefix(rule, "*.") {
			suffix := strings.TrimPrefix(rule, "*")
			if strings.HasSuffix(domain, suffix) && domain != strings.TrimPrefix(suffix, ".") {
				return true
			}
		} else if domain == rule {
			return true
		}
	}
	return false
}

func validateOperatorNetworkPorts(ports []int) ([]int, error) {
	allowed := map[int]struct{}{443: {}}
	if raw := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_NETWORK_PORTS")); raw != "" {
		allowed = map[int]struct{}{}
		for _, entry := range strings.Split(raw, ",") {
			port, err := strconv.Atoi(strings.TrimSpace(entry))
			if err != nil || port < 1 || port > 65535 {
				return nil, fmt.Errorf("invalid OMNILLM_SANDBOX_NETWORK_PORTS value")
			}
			allowed[port] = struct{}{}
		}
	}
	if len(ports) == 0 {
		ports = []int{443}
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(ports))
	for _, port := range ports {
		if _, ok := allowed[port]; !ok {
			return nil, fmt.Errorf("network port %d is not permitted by operator policy", port)
		}
		if _, exists := seen[port]; !exists {
			seen[port] = struct{}{}
			out = append(out, port)
		}
	}
	sort.Ints(out)
	return out, nil
}
