package sandbox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCredentialHandleContainsNoSecretAndIsOwnerBound(t *testing.T) {
	broker := NewCredentialBroker()
	resolved := 0
	if err := broker.RegisterSource("github", func(context.Context, OwnerScope) (string, error) {
		resolved++
		return "super-secret-token", nil
	}); err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1", ConversationID: "conversation-1"}
	handle, err := broker.Issue(owner, "github", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(handle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret-token") {
		t.Fatalf("credential handle leaked secret: %s", encoded)
	}
	if resolved != 0 {
		t.Fatalf("issuing a handle resolved the secret %d times", resolved)
	}
	if _, err := broker.Redeem(context.Background(), OwnerScope{UserID: "user-2"}, handle.ID); err == nil || !strings.Contains(err.Error(), "owned") {
		t.Fatalf("cross-owner Redeem() error = %v", err)
	}
	secret, err := broker.Redeem(context.Background(), owner, handle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "super-secret-token" || resolved != 1 {
		t.Fatalf("secret=%q resolved=%d", secret, resolved)
	}
}

func TestCredentialHandleExpiresAndRevokes(t *testing.T) {
	broker := NewCredentialBroker()
	now := time.Date(2026, 8, 12, 15, 0, 0, 0, time.UTC)
	broker.now = func() time.Time { return now }
	if err := broker.RegisterSource("service", func(context.Context, OwnerScope) (string, error) { return "secret", nil }); err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1"}
	handle, err := broker.Issue(owner, "service", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := broker.Redeem(context.Background(), owner, handle.ID); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired Redeem() error = %v", err)
	}

	now = now.Add(-2 * time.Minute)
	handle, err = broker.Issue(owner, "service", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.Revoke(owner, handle.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Redeem(context.Background(), owner, handle.ID); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("revoked Redeem() error = %v", err)
	}
}

func TestSandboxEnvironmentRejectsCredentialsAndProxyEscapeHatches(t *testing.T) {
	for _, key := range []string{
		"GITHUB_TOKEN",
		"OPENAI_API_KEY",
		"PASSWORD",
		"OMNILLM_MASTER_KEY",
		"SSH_AUTH_SOCK",
		"GIT_ASKPASS",
		"HTTPS_PROXY",
		"AWS_SHARED_CREDENTIALS_FILE",
	} {
		if err := validateSandboxEnvironmentEntry(key, "value"); err == nil {
			t.Fatalf("expected sensitive environment key %q to be rejected", key)
		}
	}
	for _, key := range []string{"LANG", "LC_ALL", "TERM", "BUILD_MODE"} {
		if err := validateSandboxEnvironmentEntry(key, "value"); err != nil {
			t.Fatalf("expected non-sensitive environment key %q to be accepted: %v", key, err)
		}
	}
}

func TestBrokerRejectsSensitiveEnvironmentBeforeRuntimeCreateOrExec(t *testing.T) {
	runtime := &fakeRuntime{capabilities: RuntimeCapabilities{Name: "fake"}}
	broker, err := NewBroker(runtime)
	if err != nil {
		t.Fatal(err)
	}
	owner := OwnerScope{UserID: "user-1"}
	if _, err := broker.Create(context.Background(), owner, CreateRequest{Environment: map[string]string{"GITHUB_TOKEN": "secret"}}); err == nil {
		t.Fatal("expected sensitive Create environment to be rejected")
	}
	if runtime.created.SessionID != "" {
		t.Fatalf("sensitive environment reached runtime create: %#v", runtime.created)
	}
	session, err := broker.Create(context.Background(), owner, CreateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Exec(context.Background(), owner, session.ID, ExecRequest{Command: "env", Env: map[string]string{"HTTPS_PROXY": "http://proxy"}}); err == nil {
		t.Fatal("expected proxy environment to be rejected")
	}
	if runtime.execCount != 0 {
		t.Fatalf("sensitive execution environment reached runtime, count=%d", runtime.execCount)
	}
}
