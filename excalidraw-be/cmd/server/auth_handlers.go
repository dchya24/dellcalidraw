package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/you/excalidraw-be/internal/auth"
	"github.com/you/excalidraw-be/internal/database"
)

type AuthHandler struct {
	authService *auth.AuthService
	db          *database.PostgresClient
}

func NewAuthHandler(authService *auth.AuthService, db *database.PostgresClient) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		db:          db,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type UpdateProfileRequest struct {
	Username  string `json:"username"`
	AvatarURL string `json:"avatarUrl"`
}

type AuthResponse struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	ExpiresAt    time.Time   `json:"expiresAt"`
	User         UserProfile `json:"user"`
}

type UserProfile struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatarUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

func userProfileFromDB(u *database.User) UserProfile {
	return UserProfile{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt,
	}
}

func (ah *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Username == "" || len(req.Username) < 3 {
		writeJSONError(w, http.StatusBadRequest, "Username must be at least 3 characters", "invalid_username")
		return
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeJSONError(w, http.StatusBadRequest, "Valid email is required", "invalid_email")
		return
	}
	if req.Password == "" || len(req.Password) < 8 {
		writeJSONError(w, http.StatusBadRequest, "Password must be at least 8 characters", "invalid_password")
		return
	}

	existing, err := ah.db.GetUserByEmail(req.Email)
	if err != nil {
		slog.Error("Failed to check existing email", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if existing != nil {
		writeJSONError(w, http.StatusConflict, "Email already registered", "email_taken")
		return
	}

	existing, err = ah.db.GetUserByUsername(req.Username)
	if err != nil {
		slog.Error("Failed to check existing username", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if existing != nil {
		writeJSONError(w, http.StatusConflict, "Username already taken", "username_taken")
		return
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	user, err := ah.db.CreateUser(req.Username, req.Email, hashedPassword)
	if err != nil {
		slog.Error("Failed to create user", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to create account", "create_failed")
		return
	}

	tokenPair, err := ah.authService.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		slog.Error("Failed to generate tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	if err := ah.db.SaveRefreshToken(user.ID, tokenPair.RefreshToken, time.Now().Add(ah.authService.RefreshTokenTTL())); err != nil {
		slog.Error("Failed to save refresh token", "error", err)
	}

	resp := AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User:         userProfileFromDB(user),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)

	slog.Info("User registered", "username", req.Username, "email", req.Email)
}

func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "Email and password required", "missing_credentials")
		return
	}

	user, err := ah.db.GetUserByEmail(req.Email)
	if err != nil {
		slog.Error("Failed to get user", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if user == nil || !auth.CheckPassword(req.Password, user.Password) {
		writeJSONError(w, http.StatusUnauthorized, "Invalid email or password", "invalid_credentials")
		return
	}

	tokenPair, err := ah.authService.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		slog.Error("Failed to generate tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	if err := ah.db.SaveRefreshToken(user.ID, tokenPair.RefreshToken, time.Now().Add(ah.authService.RefreshTokenTTL())); err != nil {
		slog.Error("Failed to save refresh token", "error", err)
	}

	resp := AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User:         userProfileFromDB(user),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	slog.Info("User logged in", "email", req.Email)
}

func (ah *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if req.RefreshToken == "" {
		writeJSONError(w, http.StatusBadRequest, "Refresh token required", "missing_token")
		return
	}

	rt, err := ah.db.GetRefreshToken(req.RefreshToken)
	if err != nil {
		slog.Error("Failed to get refresh token", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if rt == nil || rt.Revoked || time.Now().After(rt.ExpiresAt) {
		writeJSONError(w, http.StatusUnauthorized, "Invalid or expired refresh token", "invalid_token")
		return
	}

	_ = ah.db.RevokeRefreshToken(req.RefreshToken)

	user, err := ah.db.GetUserByID(rt.UserID)
	if err != nil {
		slog.Error("Failed to get user for refresh", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "User not found", "user_not_found")
		return
	}

	tokenPair, err := ah.authService.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		slog.Error("Failed to generate tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	_ = ah.db.SaveRefreshToken(user.ID, tokenPair.RefreshToken, time.Now().Add(ah.authService.RefreshTokenTTL()))

	resp := AuthResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt,
		User:         userProfileFromDB(user),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	slog.Info("Token refreshed", "userID", user.ID)
}

func (ah *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if req.RefreshToken != "" {
		_ = ah.db.RevokeRefreshToken(req.RefreshToken)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out",
	})
}

func (ah *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)

	user, err := ah.db.GetUserByID(userID)
	if err != nil {
		slog.Error("Failed to get user profile", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if user == nil {
		writeJSONError(w, http.StatusNotFound, "User not found", "user_not_found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userProfileFromDB(user))
}

func (ah *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(string)

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username != "" && len(req.Username) < 3 {
		writeJSONError(w, http.StatusBadRequest, "Username must be at least 3 characters", "invalid_username")
		return
	}

	user, err := ah.db.GetUserByID(userID)
	if err != nil || user == nil {
		writeJSONError(w, http.StatusNotFound, "User not found", "user_not_found")
		return
	}

	newUsername := user.Username
	if req.Username != "" {
		existing, _ := ah.db.GetUserByUsername(req.Username)
		if existing != nil && existing.ID != userID {
			writeJSONError(w, http.StatusConflict, "Username already taken", "username_taken")
			return
		}
		newUsername = req.Username
	}

	updated, err := ah.db.UpdateUserProfile(userID, newUsername, req.AvatarURL)
	if err != nil {
		slog.Error("Failed to update profile", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to update profile", "update_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userProfileFromDB(updated))
}

func (ah *AuthHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	if targetID == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing user ID", "missing_id")
		return
	}

	user, err := ah.db.GetUserByID(targetID)
	if err != nil {
		slog.Error("Failed to get user", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}
	if user == nil {
		writeJSONError(w, http.StatusNotFound, "User not found", "user_not_found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"avatarUrl": user.AvatarURL,
	})
}

func (ah *AuthHandler) CleanExpiredTokens(w http.ResponseWriter, r *http.Request) {
	deleted, err := ah.db.CleanExpiredTokens()
	if err != nil {
		slog.Error("Failed to clean tokens", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to clean tokens", "clean_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": deleted,
	})
}

func GenerateGuestUsername() string {
	return fmt.Sprintf("Guest_%s", uuid.New().String()[:8])
}
