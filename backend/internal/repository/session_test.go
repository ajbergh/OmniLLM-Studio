package repository_test

import (
	"testing"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestSessionTokensAreHashedAtRest(t *testing.T) {
	database := newTestDB(t)
	repo := repository.NewSessionRepo(database)
	const token = "plaintext-session-token-for-test"

	session, err := repo.Create("user-1", token, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if session.Token != token {
		t.Fatalf("returned token = %q, want original token", session.Token)
	}

	var stored string
	if err := database.QueryRow("SELECT token FROM sessions WHERE id = ?", session.ID).Scan(&stored); err != nil {
		t.Fatalf("read stored token: %v", err)
	}
	if stored == token {
		t.Fatal("session token was stored in plaintext")
	}
	if len(stored) != 64 {
		t.Fatalf("stored digest length = %d, want 64", len(stored))
	}

	loaded, err := repo.GetByToken(token)
	if err != nil {
		t.Fatalf("GetByToken() error = %v", err)
	}
	if loaded == nil || loaded.ID != session.ID {
		t.Fatalf("GetByToken() = %#v, want session %s", loaded, session.ID)
	}
	if loaded.Token != "" {
		t.Fatal("loaded session must not expose a stored token digest")
	}

	if err := repo.DeleteByToken(token); err != nil {
		t.Fatalf("DeleteByToken() error = %v", err)
	}
	loaded, err = repo.GetByToken(token)
	if err != nil {
		t.Fatalf("GetByToken() after delete error = %v", err)
	}
	if loaded != nil {
		t.Fatal("session still exists after DeleteByToken")
	}
}
