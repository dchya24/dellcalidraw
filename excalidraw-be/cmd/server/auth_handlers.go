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
		AvatarURL: u.GetAvatarURL(),
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
		"avatarUrl": user.GetAvatarURL(),
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

// Password Reset Request Types
type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type ValidateResetTokenRequest struct {
	Token string `json:"token"`
}

// ForgotPassword initiates password reset flow
func (ah *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeJSONError(w, http.StatusBadRequest, "Valid email is required", "invalid_email")
		return
	}

	// Always return success to prevent email enumeration
	successResponse := func() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "If an account exists with this email, a password reset link has been sent",
		})
	}

	user, err := ah.db.GetUserByEmail(req.Email)
	if err != nil {
		slog.Error("Failed to get user for password reset", "error", err)
		successResponse()
		return
	}
	if user == nil {
		// Don't reveal that user doesn't exist
		successResponse()
		return
	}

	// Create password reset token (expires in 1 hour)
	token, err := ah.db.CreatePasswordResetToken(user.ID, time.Hour)
	if err != nil {
		slog.Error("Failed to create password reset token", "error", err)
		successResponse()
		return
	}

	// In production, send email here
	// For now, log the token (development only)
	resetURL := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", token.Token)
	slog.Info("Password reset requested",
		"email", req.Email,
		"userID", user.ID,
		"resetURL", resetURL,
		"expiresAt", token.ExpiresAt,
	)

	// TODO: Send email with reset link
	// emailService.SendPasswordResetEmail(user.Email, resetURL)

	successResponse()
}

// ValidateResetToken checks if a reset token is valid
func (ah *AuthHandler) ValidateResetToken(w http.ResponseWriter, r *http.Request) {
	var req ValidateResetTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if req.Token == "" {
		writeJSONError(w, http.StatusBadRequest, "Token is required", "missing_token")
		return
	}

	token, err := ah.db.GetPasswordResetToken(req.Token)
	if err != nil {
		slog.Error("Failed to get password reset token", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	if token == nil {
		writeJSONError(w, http.StatusNotFound, "Invalid or expired reset token", "invalid_token")
		return
	}

	if token.UsedAt != nil {
		writeJSONError(w, http.StatusGone, "Reset token has already been used", "token_used")
		return
	}

	if time.Now().After(token.ExpiresAt) {
		writeJSONError(w, http.StatusGone, "Reset token has expired", "token_expired")
		return
	}

	// Get user email for display (masked)
	user, _ := ah.db.GetUserByID(token.UserID)
	maskedEmail := ""
	if user != nil {
		maskedEmail = maskEmail(user.Email)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":     true,
		"email":     maskedEmail,
		"expiresAt": token.ExpiresAt,
	})
}

// ResetPassword completes the password reset
func (ah *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", "invalid_body")
		return
	}

	if req.Token == "" {
		writeJSONError(w, http.StatusBadRequest, "Token is required", "missing_token")
		return
	}

	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		writeJSONError(w, http.StatusBadRequest, "Password must be at least 8 characters", "invalid_password")
		return
	}

	token, err := ah.db.GetPasswordResetToken(req.Token)
	if err != nil {
		slog.Error("Failed to get password reset token", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	if token == nil {
		writeJSONError(w, http.StatusNotFound, "Invalid or expired reset token", "invalid_token")
		return
	}

	if token.UsedAt != nil {
		writeJSONError(w, http.StatusGone, "Reset token has already been used", "token_used")
		return
	}

	if time.Now().After(token.ExpiresAt) {
		writeJSONError(w, http.StatusGone, "Reset token has expired", "token_expired")
		return
	}

	// Hash new password
	hashedPassword, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("Failed to hash password", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal error", "internal_error")
		return
	}

	// Update password
	if err := ah.db.UpdateUserPassword(token.UserID, hashedPassword); err != nil {
		slog.Error("Failed to update password", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to update password", "update_failed")
		return
	}

	// Mark token as used
	if err := ah.db.UsePasswordResetToken(req.Token); err != nil {
		slog.Warn("Failed to mark token as used", "error", err)
	}

	// Revoke all existing refresh tokens for security
	if err := ah.db.RevokeAllUserTokens(token.UserID); err != nil {
		slog.Warn("Failed to revoke user tokens", "error", err)
	}

	slog.Info("Password reset completed", "userID", token.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Password has been reset successfully. Please log in with your new password.",
	})
}

// maskEmail masks an email address for privacy (e.g., "j***@example.com")
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) <= 1 {
		return local + "***@" + domain
	}

	return string(local[0]) + "***@" + domain
}
