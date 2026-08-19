package sandbox

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type staticBrokeredHTTPResolver struct {
	addresses map[string][]net.IPAddr
	err       error
}

func (r staticBrokeredHTTPResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]net.IPAddr(nil), r.addresses[host]...), nil
}

func TestValidateBrokeredHTTPURLRequiresExactGrantDestination(t *testing.T) {
	grant := &NetworkGrant{Domains: []string{"api.example.com"}, Ports: []int{443, 8443}}
	for _, tc := range []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "default https", rawURL: "https://api.example.com/v1"},
		{name: "explicit allowed port", rawURL: "https://api.example.com:8443/v1"},
		{name: "wrong host", rawURL: "https://other.example.com/v1", wantErr: true},
		{name: "subdomain not implied", rawURL: "https://x.api.example.com/v1", wantErr: true},
		{name: "wrong port", rawURL: "https://api.example.com:9443/v1", wantErr: true},
		{name: "ip literal", rawURL: "https://203.0.113.10/v1", wantErr: true},
		{name: "userinfo", rawURL: "https://user@api.example.com/v1", wantErr: true},
		{name: "unsupported scheme", rawURL: "ftp://api.example.com/file", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatal(err)
			}
			err = validateBrokeredHTTPURL(target, grant)
			if tc.wantErr && err == nil {
				t.Fatalf("validateBrokeredHTTPURL(%q) unexpectedly succeeded", tc.rawURL)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateBrokeredHTTPURL(%q) = %v", tc.rawURL, err)
			}
		})
	}
}

func TestBrokeredHTTPResolvePublicIPsRejectsMixedPrivateDNSAnswer(t *testing.T) {
	client := NewBrokeredHTTPClient(NewNetworkGrantStore(), NewCredentialBroker())
	client.resolver = staticBrokeredHTTPResolver{addresses: map[string][]net.IPAddr{
		"api.example.com": {
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("127.0.0.1")},
		},
	}}
	if _, err := client.resolvePublicIPs(context.Background(), "api.example.com"); err == nil || !strings.Contains(err.Error(), "prohibited address") {
		t.Fatalf("mixed private DNS answer err = %v", err)
	}
}

func TestBrokeredHTTPResolvePublicIPsAcceptsOnlyPublicAnswers(t *testing.T) {
	client := NewBrokeredHTTPClient(NewNetworkGrantStore(), NewCredentialBroker())
	client.resolver = staticBrokeredHTTPResolver{addresses: map[string][]net.IPAddr{
		"api.example.com": {{IP: net.ParseIP("93.184.216.34")}},
	}}
	ips, err := client.resolvePublicIPs(context.Background(), "api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("unexpected public IPs: %#v", ips)
	}
}

func TestBrokeredHTTPCredentialConsumerIsServiceAndDestinationBound(t *testing.T) {
	credentials := NewCredentialBroker()
	owner := OwnerScope{UserID: "user-1", ConversationID: "conversation-1"}
	if err := credentials.RegisterSource("example_api", func(context.Context, OwnerScope) (string, error) {
		return "super-secret", nil
	}); err != nil {
		t.Fatal(err)
	}
	handle, err := credentials.Issue(owner, "example_api", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	client := NewBrokeredHTTPClient(NewNetworkGrantStore(), credentials)
	if err := client.RegisterCredentialConsumer(HTTPCredentialConsumer{
		Service: "example_api",
		Domains: []string{"api.example.com"},
		Header:  "Authorization",
		Prefix:  "Bearer ",
	}); err != nil {
		t.Fatal(err)
	}

	target, _ := url.Parse("https://api.example.com/v1")
	consumer, secret, err := client.resolveCredentialConsumer(context.Background(), owner, handle.ID, target)
	if err != nil {
		t.Fatal(err)
	}
	if consumer.Service != "example_api" || secret != "super-secret" {
		t.Fatalf("unexpected consumer/service result: %#v secret=%q", consumer, secret)
	}

	wrongHost, _ := url.Parse("https://other.example.com/v1")
	if _, _, err := client.resolveCredentialConsumer(context.Background(), owner, handle.ID, wrongHost); err == nil {
		t.Fatal("credential consumer unexpectedly allowed another hostname")
	}
	plainHTTP, _ := url.Parse("http://api.example.com/v1")
	if _, _, err := client.resolveCredentialConsumer(context.Background(), owner, handle.ID, plainHTTP); err == nil {
		t.Fatal("credential consumer unexpectedly allowed plaintext HTTP")
	}
	otherOwner := OwnerScope{UserID: "user-2", ConversationID: "conversation-1"}
	if _, _, err := client.resolveCredentialConsumer(context.Background(), otherOwner, handle.ID, target); err == nil {
		t.Fatal("credential consumer unexpectedly redeemed another owner's handle")
	}
}

func TestBrokeredHTTPRejectsCallerCredentialAndProxyHeaders(t *testing.T) {
	for _, header := range []string{"Authorization", "Cookie", "Proxy-Authorization", "Host", "Forwarded", "X-Forwarded-For"} {
		if !brokeredHTTPHeaderForbidden(header) {
			t.Fatalf("expected %s to be forbidden", header)
		}
	}
	if brokeredHTTPHeaderForbidden("Accept") {
		t.Fatal("Accept should remain caller-controlled")
	}
}

func TestSafeBrokeredHTTPResponseHeadersDropsCredentialState(t *testing.T) {
	headers := http.Header{
		"Content-Type":  []string{"application/json"},
		"Set-Cookie":    []string{"session=secret"},
		"Authorization": []string{"Bearer secret"},
		"Etag":          []string{"abc"},
	}
	safe := safeBrokeredHTTPResponseHeaders(headers)
	if got := firstHeaderValue(safe, "Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if firstHeaderValue(safe, "Set-Cookie") != "" || firstHeaderValue(safe, "Authorization") != "" {
		t.Fatalf("unsafe response headers leaked: %#v", safe)
	}
}

func firstHeaderValue(headers map[string][]string, key string) string {
	values := headers[http.CanonicalHeaderKey(key)]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
