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
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

const passwordIterations = 120000

// minPasswordLength es el largo mínimo de contraseña al registrarse.
const minPasswordLength = 10

type authInput struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	var v httpx.Validator
	v.Required("name", input.Name)
	v.Check(strings.Contains(input.Email, "@"), "email", "format", "email debe ser un correo válido")
	v.Check(len(input.Password) >= minPasswordLength, "password", "min",
		fmt.Sprintf("password debe tener al menos %d caracteres", minPasswordLength))
	if !valid(w, r, &v) {
		return
	}

	hash, err := passwordHash(input.Password)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	var userID int32
	err = h.db.QueryRow(r.Context(), `INSERT INTO user_account(name,email,password_hash) VALUES ($1,$2,$3) RETURNING id`,
		input.Name, input.Email, hash).Scan(&userID)
	if err != nil {
		h.dbError(w, r, err, http.StatusConflict, "El email ya está registrado.")
		return
	}
	h.issueTokens(w, r, userID, http.StatusCreated)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("email", input.Email)
	v.Required("password", input.Password)
	if !valid(w, r, &v) {
		return
	}
	var userID int32
	var hash string
	err := h.db.QueryRow(r.Context(), `SELECT id,password_hash FROM user_account WHERE email=$1 AND active`,
		strings.ToLower(strings.TrimSpace(input.Email))).Scan(&userID, &hash)
	if err != nil {
		h.dbError(w, r, err, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	if !checkPassword(hash, input.Password) {
		writeError(w, r, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	h.issueTokens(w, r, userID, http.StatusOK)
}

func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, userID int32, status int) {
	var name, email string
	if err := h.db.QueryRow(r.Context(), `SELECT name,email FROM user_account WHERE id=$1 AND active`,
		userID).Scan(&name, &email); err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	access, err := newToken()
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	refresh, err := newToken()
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	refreshExpiry := time.Now().Add(30 * 24 * time.Hour)
	// Los vencimientos los calcula la base con NOW(), el mismo reloj con que
	// se validan: las columnas son TIMESTAMP sin zona y un time.Time de Go
	// llega con la hora local del proceso (en -03:00 el token nacía vencido).
	_, err = h.db.Exec(r.Context(), `INSERT INTO user_session
		(user_id,access_token_hash,refresh_token_hash,access_expires_at,refresh_expires_at)
		VALUES ($1,$2,$3,NOW()+INTERVAL '15 minutes',NOW()+INTERVAL '30 days')`, userID, tokenHash(access), tokenHash(refresh))
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, status, map[string]any{"userId": userID, "accessToken": access, "refreshToken": refresh,
		"name": name, "email": email, "tokenType": "Bearer", "expiresIn": 900,
		"refreshExpiresAt": refreshExpiry})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("refreshToken", input.RefreshToken)
	if !valid(w, r, &v) {
		return
	}
	var userID int32
	err := h.db.QueryRow(r.Context(), `DELETE FROM user_session WHERE refresh_token_hash=$1 AND refresh_expires_at>NOW()
		RETURNING user_id`, tokenHash(input.RefreshToken)).Scan(&userID)
	if err != nil {
		h.dbError(w, r, err, http.StatusUnauthorized, "El refresh token no es válido o venció.")
		return
	}
	h.issueTokens(w, r, userID, http.StatusOK)
}

// Logout cierra la sesión del bearer token o, si no viene, la del
// refreshToken del body. Sin ninguno de los dos no hay nada que cerrar: 204.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" && r.ContentLength != 0 {
		var input authInput
		if !decode(w, r, &input) {
			return
		}
		token = input.RefreshToken
	}
	if token != "" {
		if _, err := h.db.Exec(r.Context(), `DELETE FROM user_session WHERE access_token_hash=$1 OR refresh_token_hash=$1`, tokenHash(token)); err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, r, http.StatusUnauthorized, "Falta el bearer token.")
		return
	}
	var userID int32
	err := h.db.QueryRow(r.Context(), `SELECT user_id FROM user_session WHERE access_token_hash=$1 AND access_expires_at>NOW()`, tokenHash(token)).Scan(&userID)
	if err != nil {
		h.dbError(w, r, err, http.StatusUnauthorized, "El access token no es válido o venció.")
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
