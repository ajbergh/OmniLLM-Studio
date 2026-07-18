package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const (
	ContextKeyUser contextKey = "auth_user"

	SessionTokenHeader = "Authorization"
	SessionCookieName  = "omnillm_session"
	SessionDuration    = 7 * 24 * time.Hour
	BcryptCost         = 12
)

type UserRepo interface {
	CountUsers() (int, error)
}

type SessionRepo interface {
	GetByToken(token string) (*models.Session, error)
}

type UserByIDFunc func(id string) (*models.User, error)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(ContextKeyUser).(*models.User)
	return user
}

func UserIDFromContext(ctx context.Context) string {
	if user := UserFromContext(ctx); user != nil {
		return user.ID
	}
	return ""
}

func isLocalhost(bindAddress string) bool {
	host := strings.TrimSpace(bindAddress)
	if host == "" {
		return true
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func Middleware(userRepo UserRepo, sessionRepo SessionRepo, getUserByID UserByIDFunc, bindAddress ...string) func(http.Handler) http.Handler {
	bindAddr := ""
	if len(bindAddress) > 0 {
		bindAddr = bindAddress[0]
	}
	localBind := isLocalhost(bindAddr)
	var soloWarnOnce sync.Once

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				http.Error(w, `{"error":"setup required: register the first user via /v1/auth/register"}`, http.StatusServiceUnavailable)
				return
			}

			token := TokenFromRequest(r)
			if token == "" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			session, err := sessionRepo.GetByToken(token)
			if err != nil || session == nil {
				http.Error(w, `{"error":"invalid or expired session"}`, http.StatusUnauthorized)
				return
			}
			if time.Now().After(session.ExpiresAt) {
				http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
				return
			}
			user, err := getUserByID(session.UserID)
			if err != nil || user == nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ContextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
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

// TokenFromRequest accepts a bearer token for API clients and falls back to the
// HttpOnly session cookie used by the web application.
func TokenFromRequest(r *http.Request) string {
	if token := extractBearerToken(r); token != "" {
		return token
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func extractBearerToken(r *http.Request) string {
	header := r.Header.Get(SessionTokenHeader)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
