package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// contextKey is a private type for context keys in this package.
type contextKey string

const (
	// ContextKeyUser is the context key for the authenticated user.
	ContextKeyUser contextKey = "auth_user"

	// SessionTokenHeader is the header used for session-based auth.
	SessionTokenHeader = "Authorization"

	// SessionDuration is the default validity period for a session token.
	SessionDuration = 7 * 24 * time.Hour // 7 days

	// BcryptCost is the bcrypt hashing cost.
	BcryptCost = 12
)

// UserRepo is the interface for looking up users (avoids import cycle).
type UserRepo interface {
	CountUsers() (int, error)
}

// SessionRepo is the interface for looking up sessions (avoids import cycle).
type SessionRepo interface {
	GetByToken(token string) (*models.Session, error)
}

// UserByIDFunc resolves a user by ID.
type UserByIDFunc func(id string) (*models.User, error)

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken creates a secure random session token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if no user is set (solo mode or unauthenticated).
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(ContextKeyUser).(*models.User)
	return u
}

// UserIDFromContext returns the authenticated user's ID, or "" in solo mode.
func UserIDFromContext(ctx context.Context) string {
	u := UserFromContext(ctx)
	if u != nil {
		return u.ID
	}
	return ""
}

// isLocalhost checks if a bind address is a localhost address (safe for solo mode).
func isLocalhost(bindAddr string) bool {
	// Extract host portion if addr includes port
	host := bindAddr
	if idx := strings.LastIndex(bindAddr, ":"); idx >= 0 {
		host = bindAddr[:idx]
	}
	host = strings.TrimSpace(host)
	return host == "" || host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// Middleware returns an HTTP middleware that:
// - If no users exist AND bind is localhost (solo mode): passes through without auth.
// - If no users exist AND bind is NOT localhost: requires registration first (setup mode).
// - If users exist: requires a valid session token in the Authorization header.
func Middleware(userRepo UserRepo, sessionRepo SessionRepo, getUserByID UserByIDFunc, bindAddress ...string) func(http.Handler) http.Handler {
	bindAddr := ""
	if len(bindAddress) > 0 {
		bindAddr = bindAddress[0]
	}
	localBind := isLocalhost(bindAddr)
	var soloWarnOnce sync.Once
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if any users exist — solo mode bypasses auth entirely
			count, err := userRepo.CountUsers()
			if err != nil {
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}
			if count == 0 {
				if localBind {
					soloWarnOnce.Do(func() {
						log.Println("[WARN] Running in solo mode (no users). Register an account to enable authentication.")
					})
					next.ServeHTTP(w, r)
					return
				}
				// Non-localhost bind with no users: block until first user is registered
				http.Error(w, `{"error":"setup required: register the first user via /v1/auth/register"}`, http.StatusServiceUnavailable)
				return
			}

			// Multi-user mode: require valid session token
			token := extractBearerToken(r)
			if token == "" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessionRepo.GetByToken(token)
			if err != nil || session == nil {
				http.Error(w, `{"error":"invalid or expired session"}`, http.StatusUnauthorized)
				return
			}

			// Check expiry
			if time.Now().After(session.ExpiresAt) {
				http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
				return
			}

			// Load user
			user, err := getUserByID(session.UserID)
			if err != nil || user == nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}

			// Inject user into context
			ctx := context.WithValue(r.Context(), ContextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that checks if the authenticated user has one
// of the specified roles. In solo mode (no user in context) all access is allowed.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				// Solo mode — allow all
				next.ServeHTTP(w, r)
				return
			}
			if !allowed[user.Role] {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractBearerToken extracts the token from "Bearer <token>" header.
func extractBearerToken(r *http.Request) string {
	h := r.Header.Get(SessionTokenHeader)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
