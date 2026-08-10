package gitrepo

import "testing"

func TestParseRemoteConfigAcceptsHTTPSAndKeepsSecretsIndirect(t *testing.T) {
	raw := `{
		"origin":{"repository":"omni","url":"https://github.com/example/repo.git","token_env":"OMNILLM_GIT_TOKEN","allow_push":true},
		"public":{"repository":"omni","url":"https://git.example.com/repo.git"}
	}`
	configured := ParseRemoteConfig(raw)
	if len(configured) != 2 {
		t.Fatalf("configured remotes = %d, want 2", len(configured))
	}
	origin := configured["origin"]
	if origin.Username != "git" || origin.TokenEnv != "OMNILLM_GIT_TOKEN" || !origin.AllowPush {
		t.Fatalf("unexpected origin config: %#v", origin)
	}
}

func TestParseRemoteConfigRejectsUnsafeEndpoints(t *testing.T) {
	raw := `{
		"http":{"repository":"omni","url":"http://github.com/example/repo.git"},
		"creds":{"repository":"omni","url":"https://user:secret@github.com/example/repo.git"},
		"query":{"repository":"omni","url":"https://github.com/example/repo.git?token=secret"},
		"port":{"repository":"omni","url":"https://github.com:8443/example/repo.git"},
		"bad-token-env":{"repository":"omni","url":"https://github.com/example/repo.git","token_env":"BAD-NAME"},
		"good":{"repository":"omni","url":"https://github.com/example/repo.git"}
	}`
	configured := ParseRemoteConfig(raw)
	if len(configured) != 1 {
		t.Fatalf("configured remotes = %#v, want only good", configured)
	}
	if _, ok := configured["good"]; !ok {
		t.Fatal("good remote was rejected")
	}
}

func TestNewRemoteServiceFiltersUnknownLocalRepositories(t *testing.T) {
	local := NewService(map[string]string{"omni": t.TempDir()})
	configured := map[string]RemoteConfig{
		"good": {Repository: "omni", URL: "https://github.com/example/repo.git"},
		"bad":  {Repository: "other", URL: "https://github.com/example/other.git"},
	}
	filtered := make(map[string]RemoteConfig)
	for id, remote := range configured {
		if _, ok := local.repositories[remote.Repository]; ok {
			filtered[id] = remote
		}
	}
	svc := newRemoteService(filtered, true, false, nil, nil)
	if !svc.Configured() || len(svc.ids) != 1 || svc.ids[0] != "good" {
		t.Fatalf("unexpected filtered remote service: %#v", svc.ids)
	}
}
