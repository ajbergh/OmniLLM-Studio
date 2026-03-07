package api

import (
	"net/http"
	"regexp"
	"strings"
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
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(userRepo *repository.UserRepo, sessionRepo *repository.SessionRepo, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		cfg:         cfg,
	}
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
	Token     string                     `json:"token"`
	ExpiresAt time.Time                  `json:"expires_at"`
	User      models.UserWithoutPassword `json:"user"`
}

// Register creates a new user account. The first user becomes admin.
// Subsequent registrations require AllowPublicReg to be true.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Gate registration: after the first user, only allow if config permits.
	count, err := h.userRepo.CountUsers()
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if count > 0 && !h.cfg.AllowPublicReg {
		respondError(w, http.StatusForbidden, "registration is disabled")
		return
	}

	// Validate
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if len(req.Username) > 64 {
		respondError(w, http.StatusBadRequest, "username too long (max 64 characters)")
		return
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`).MatchString(req.Username) {
		respondError(w, http.StatusBadRequest, "username contains invalid characters (allowed: a-z, 0-9, _, ., -)")
		return
	}
	if len(req.Password) > 256 {
		respondError(w, http.StatusBadRequest, "password too long (max 256 characters)")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}

	// Check if username is taken
	existing, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check username")
		return
	}
	if existing != nil {
		respondError(w, http.StatusConflict, "username already taken")
		return
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// First user = admin, subsequent = member
	role := "member"
	if count == 0 {
		role = "admin"
	}

	// Create user
	user, err := h.userRepo.Create(repository.CreateUserInput{
		Username:     req.Username,
		DisplayName:  req.DisplayName,
		PasswordHash: hash,
		Role:         role,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Create session
	token, err := auth.GenerateToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	expiresAt := time.Now().Add(auth.SessionDuration)
	if _, err := h.sessionRepo.Create(user.ID, token, expiresAt); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	respondJSON(w, http.StatusCreated, authResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      toUserResponse(user),
	})
}

// Login authenticates a user and returns a session token.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	// Find user
	user, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to find user")
		return
	}
	if user == nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		respondError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	// Session rotation: revoke previous sessions for this user, then create new
	_ = h.sessionRepo.DeleteByUserID(user.ID) // best-effort revocation
	token, err := auth.GenerateToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	expiresAt := time.Now().Add(auth.SessionDuration)
	if _, err := h.sessionRepo.Create(user.ID, token, expiresAt); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	respondJSON(w, http.StatusOK, authResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      toUserResponse(user),
	})
}

// Logout invalidates the current session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Extract token from header
	header := r.Header.Get("Authorization")
	parts := strings.SplitN(header, " ", 2)
	if len(parts) == 2 {
		token := strings.TrimSpace(parts[1])
		_ = h.sessionRepo.DeleteByToken(token)
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Me returns the current authenticated user's profile.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		// Solo mode — return a synthetic anonymous user
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"id":           "anonymous",
			"username":     "anonymous",
			"display_name": "Local User",
			"role":         "admin",
			"solo_mode":    true,
		})
		return
	}

	respondJSON(w, http.StatusOK, toUserResponse(user))
}

// AuthStatus returns whether auth is enabled (users exist) or solo mode.
func (h *AuthHandler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	count, err := h.userRepo.CountUsers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check auth status")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"auth_enabled": count > 0,
		"has_users":    count > 0,
	})
}

func toUserResponse(u *models.User) models.UserWithoutPassword {
	return models.UserWithoutPassword{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
