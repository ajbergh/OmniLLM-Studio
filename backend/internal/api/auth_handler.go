package api

import (
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/auth"
	"github.com/ajbergh/omnillm-studio/internal/config"
	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userRepo    *repository.UserRepo
	sessionRepo *repository.SessionRepo
	cfg         *config.Config

	// Registration count and first-admin creation must be atomic. Without this
	// lock, two concurrent first registrations can both observe zero users and
	// both become administrators.
	registerMu sync.Mutex
}

func NewAuthHandler(userRepo *repository.UserRepo, sessionRepo *repository.SessionRepo, cfg *config.Config) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, sessionRepo: sessionRepo, cfg: cfg}
}

type registerRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	// Token remains available for non-browser API clients. The first-party web
	// application can authenticate through the HttpOnly cookie instead.
	Token     string                     `json:"token"`
	ExpiresAt time.Time                  `json:"expires_at"`
	User      models.UserWithoutPassword `json:"user"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	h.registerMu.Lock()
	defer h.registerMu.Unlock()

	var request registerRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	if request.Username == "" || request.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(request.Username) > 64 {
		respondError(w, http.StatusBadRequest, "username too long (max 64 characters)")
		return
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`).MatchString(request.Username) {
		respondError(w, http.StatusBadRequest, "username contains invalid characters (allowed: a-z, 0-9, _, ., -)")
		return
	}
	if len(request.Password) < 8 || len(request.Password) > 256 {
		respondError(w, http.StatusBadRequest, "password must be between 8 and 256 characters")
		return
	}
	if request.DisplayName == "" {
		request.DisplayName = request.Username
	}
	if len(request.DisplayName) > 128 {
		respondError(w, http.StatusBadRequest, "display name too long (max 128 characters)")
		return
	}

	count, err := h.userRepo.CountUsers()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if count > 0 && !h.cfg.AllowPublicReg {
		respondError(w, http.StatusForbidden, "registration is disabled")
		return
	}
	existing, err := h.userRepo.GetByUsername(request.Username)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if existing != nil {
		respondError(w, http.StatusConflict, "username already taken")
		return
	}

	passwordHash, err := auth.HashPassword(request.Password)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	role := "member"
	if count == 0 {
		role = "admin"
	}
	user, err := h.userRepo.Create(repository.CreateUserInput{
		Username: request.Username, DisplayName: request.DisplayName,
		PasswordHash: passwordHash, Role: role,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, expiresAt, err := h.createSession(user.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	setSessionCookie(w, r, token, expiresAt)
	respondJSON(w, http.StatusCreated, authResponse{Token: token, ExpiresAt: expiresAt, User: toUserResponse(user)})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" || request.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := h.userRepo.GetByUsername(request.Username)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if user == nil || !auth.CheckPassword(user.PasswordHash, request.Password) {
		respondError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	// Rotate sessions so a successful login revokes previously exposed tokens.
	_ = h.sessionRepo.DeleteByUserID(user.ID)
	token, expiresAt, err := h.createSession(user.ID)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	setSessionCookie(w, r, token, expiresAt)
	respondJSON(w, http.StatusOK, authResponse{Token: token, ExpiresAt: expiresAt, User: toUserResponse(user)})
}

func (h *AuthHandler) createSession(userID string) (string, time.Time, error) {
	token, err := auth.GenerateToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().UTC().Add(auth.SessionDuration)
	if _, err := h.sessionRepo.Create(userID, token, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if token := auth.TokenFromRequest(r); token != "" {
		_ = h.sessionRepo.DeleteByToken(token)
	}
	clearSessionCookie(w, r)
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"id": "anonymous", "username": "anonymous", "display_name": "Local User",
			"role": "admin", "solo_mode": true,
		})
		return
	}
	respondJSON(w, http.StatusOK, toUserResponse(user))
}

func (h *AuthHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	count, err := h.userRepo.CountUsers()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"auth_enabled": count > 0, "has_users": count > 0})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: auth.SessionCookieName, Value: token, Path: "/v1", Expires: expiresAt,
		MaxAge: int(time.Until(expiresAt).Seconds()), HttpOnly: true,
		Secure: requestIsHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: auth.SessionCookieName, Value: "", Path: "/v1",
		MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true,
		Secure: requestIsHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func toUserResponse(user *models.User) models.UserWithoutPassword {
	return models.UserWithoutPassword{
		ID: user.ID, Username: user.Username, DisplayName: user.DisplayName,
		Role: user.Role, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}
