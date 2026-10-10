package features

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
)

const passwordIterations = 120000

type authInput struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decode(r, &input) || strings.TrimSpace(input.Name) == "" || !strings.Contains(input.Email, "@") || len(input.Password) < 10 {
		badRequest(w, "name, valid email and password of at least 10 characters are required")
		return
	}
	hash, err := passwordHash(input.Password)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not secure password")
		return
	}
	var userID int32
	err = h.db.QueryRow(r.Context(), `INSERT INTO user_account(name,email,password_hash) VALUES ($1,$2,$3) RETURNING id`,
		strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Email)), hash).Scan(&userID)
	if err != nil {
		apiutil.WriteError(w, http.StatusConflict, "email is already registered")
		return
	}
	h.issueTokens(w, r, userID, http.StatusCreated)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decode(r, &input) || input.Email == "" || input.Password == "" {
		badRequest(w, "email and password are required")
		return
	}
	var userID int32
	var hash string
	err := h.db.QueryRow(r.Context(), `SELECT id,password_hash FROM user_account WHERE email=$1 AND active`,
		strings.ToLower(strings.TrimSpace(input.Email))).Scan(&userID, &hash)
	if err != nil || !checkPassword(hash, input.Password) {
		apiutil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	h.issueTokens(w, r, userID, http.StatusOK)
}

func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, userID int32, status int) {
	var name, email string
	if err := h.db.QueryRow(r.Context(), `SELECT name,email FROM user_account WHERE id=$1 AND active`,
		userID).Scan(&name, &email); err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	access, err := newToken()
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	refresh, err := newToken()
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	accessExpiry := time.Now().Add(15 * time.Minute)
	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
	_, err = h.db.Exec(r.Context(), `INSERT INTO user_session
		(user_id,access_token_hash,refresh_token_hash,access_expires_at,refresh_expires_at)
		VALUES ($1,$2,$3,$4,$5)`, userID, tokenHash(access), tokenHash(refresh), accessExpiry, refreshExpiry)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not save session")
		return
	}
	apiutil.WriteJSON(w, status, map[string]any{"userId": userID, "accessToken": access, "refreshToken": refresh,
		"name": name, "email": email, "tokenType": "Bearer", "expiresIn": 900,
		"refreshExpiresAt": refreshExpiry})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decode(r, &input) || input.RefreshToken == "" {
		badRequest(w, "refreshToken is required")
		return
	}
	var userID int32
	err := h.db.QueryRow(r.Context(), `DELETE FROM user_session WHERE refresh_token_hash=$1 AND refresh_expires_at>NOW()
		RETURNING user_id`, tokenHash(input.RefreshToken)).Scan(&userID)
	if err != nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	h.issueTokens(w, r, userID, http.StatusOK)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		var input authInput
		_ = decode(r, &input)
		token = input.RefreshToken
	}
	if token != "" {
		_, _ = h.db.Exec(r.Context(), `DELETE FROM user_session WHERE access_token_hash=$1 OR refresh_token_hash=$1`, tokenHash(token))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		apiutil.WriteError(w, http.StatusUnauthorized, "bearer token is required")
		return
	}
	var userID int32
	err := h.db.QueryRow(r.Context(), `SELECT user_id FROM user_session WHERE access_token_hash=$1 AND access_expires_at>NOW()`, tokenHash(token)).Scan(&userID)
	if err != nil {
		apiutil.WriteError(w, http.StatusUnauthorized, "invalid or expired access token")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]any{"valid": true, "userId": userID})
}

func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func newToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := pbkdf2([]byte(password), salt, passwordIterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)), nil
}

func checkPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations := passwordIterations
	if _, err := fmt.Sscanf(parts[1], "%d", &iterations); err != nil || iterations < 100000 || iterations > 1000000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got := pbkdf2([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func pbkdf2(password, salt []byte, iterations, keyLength int) []byte {
	result := make([]byte, 0, keyLength)
	for block := uint32(1); len(result) < keyLength; block++ {
		mac := hmacSHA256(password, append(append([]byte(nil), salt...), byte(block>>24), byte(block>>16), byte(block>>8), byte(block)))
		u := mac
		value := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			u = hmacSHA256(password, u)
			for j := range value {
				value[j] ^= u[j]
			}
		}
		result = append(result, value...)
	}
	return result[:keyLength]
}

func hmacSHA256(key, data []byte) []byte {
	block := make([]byte, 64)
	if len(key) > len(block) {
		hash := sha256.Sum256(key)
		key = hash[:]
	}
	copy(block, key)
	inner, outer := make([]byte, 64), make([]byte, 64)
	for i := range block {
		inner[i], outer[i] = block[i]^0x36, block[i]^0x5c
	}
	innerHash := sha256.Sum256(append(inner, data...))
	outerHash := sha256.Sum256(append(outer, innerHash[:]...))
	return outerHash[:]
}
