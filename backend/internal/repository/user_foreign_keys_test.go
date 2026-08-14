package repository_test

import (
	"database/sql"
	"testing"

	"github.com/ajbergh/omnillm-studio/internal/repository"
)

func TestUserDeleteAnonymizesContentAndCascadesDependentRows(t *testing.T) {
	database := newTestDB(t)
	if _, err := database.Exec(`
		INSERT INTO users (id, username, display_name, password_hash, role)
		VALUES ('user', 'user', 'User', 'hash', 'member');
		INSERT INTO conversations (id, title, user_id) VALUES ('conversation', 'Kept', 'user');
		INSERT INTO messages (id, conversation_id, role, content, user_id)
		VALUES ('message', 'conversation', 'user', 'Kept', 'user');
		INSERT INTO sessions (id, user_id, token, expires_at)
		VALUES ('session', 'user', 'token', datetime('now', '+1 day'));
	`); err != nil {
		t.Fatalf("seed user data: %v", err)
	}

	if err := repository.NewUserRepo(database).Delete("user"); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var conversationUser, messageUser sql.NullString
	if err := database.QueryRow("SELECT user_id FROM conversations WHERE id = 'conversation'").Scan(&conversationUser); err != nil {
		t.Fatalf("query conversation: %v", err)
	}
	if err := database.QueryRow("SELECT user_id FROM messages WHERE id = 'message'").Scan(&messageUser); err != nil {
		t.Fatalf("query message: %v", err)
	}
	if conversationUser.Valid || messageUser.Valid {
		t.Fatalf("content was not anonymized: conversation=%v message=%v", conversationUser, messageUser)
	}
	var sessions int
	if err := database.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = 'user'").Scan(&sessions); err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	if sessions != 0 {
		t.Fatalf("session count=%d, want 0", sessions)
	}
}
