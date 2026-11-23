package features

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/jackc/pgx/v5"
)

const (
	passwordIterations = 120000
	maxAuthBodyBytes   = 16 << 10
	maxPasswordBytes   = 1024
	maxNameLength      = 150
	maxEmailLength     = 254
)

// minPasswordLength es el largo mínimo de contraseña al registrarse.
const minPasswordLength = 10

// Límites de la Auth API contra solicitudes excesivas (TAL-201, hallazgo H1
// de TAL-195). Cada PBKDF2 cuesta ~59 ms de CPU (benchmark de checkPassword)
// y el pod usuarios tiene un límite de 200m: sin tope, unos 3 logins por
// segundo lo saturan.
const (
	// maxLoginFailures intentos fallidos por email dentro de
	// loginFailureWindow bloquean los siguientes hasta que la ventana vence.
	maxLoginFailures   = 10
	loginFailureWindow = 15 * time.Minute
	// maxConcurrentPasswordHashes cálculos de PBKDF2 a la vez; el resto espera
	// hasta passwordSlotWait y, si no hay cupo, recibe 429.
	maxConcurrentPasswordHashes = 2
	passwordSlotWait            = 2 * time.Second
	// passwordBusyRetry es el Retry-After cuando no hubo cupo para PBKDF2.
	passwordBusyRetry = 2 * time.Second
)

const (
	loginBlockedDetail = "Demasiados intentos fallidos para este email. Espera antes de reintentar."
	passwordBusyDetail = "El servicio está ocupado. Reintenta en unos segundos."
)

const (
	// refreshCookieName es la cookie httpOnly donde viaja el refresh token.
	refreshCookieName = "refresh_token"
	// refreshTokenLifetime es la vida del refresh token y de su cookie.
	refreshTokenLifetime = 30 * 24 * time.Hour
)

type authInput struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	RefreshToken string `json:"refreshToken"`
}

// changePasswordInput es el cuerpo de POST /auth/change-password. No reusa
// authInput porque ese acepta name, email y refreshToken, que aquí no aplican.
type changePasswordInput struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// Register atiende POST /auth/register: crea la cuenta y emite tokens.
// Recibe name, email y password; responde 201 con la sesión, 422 si no
// cumplen las reglas, 409 si el email ya existe y 429 si no hay cupo para
// calcular el hash de la contraseña (TAL-201).
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decodeAuth(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	var v httpx.Validator
	v.Required("name", input.Name)
	v.MaxLength("name", input.Name, maxNameLength)
	v.Check(validEmail(input.Email), "email", "format", "email debe ser un correo válido")
	v.Check(len(input.Password) >= minPasswordLength, "password", "min",
		fmt.Sprintf("password debe tener al menos %d caracteres", minPasswordLength))
	v.Check(len(input.Password) <= maxPasswordBytes, "password", "max_length",
		fmt.Sprintf("password no puede superar %d bytes", maxPasswordBytes))
	if !valid(w, r, &v) {
		return
	}

	if !h.passwordSlots.Acquire(r.Context()) {
		httpx.WriteRateLimited(w, r, passwordBusyRetry, passwordBusyDetail)
		return
	}
	hash, err := passwordHash(input.Password)
	h.passwordSlots.Release()
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

// Login atiende POST /auth/login: valida email y password y emite tokens.
// Responde 200 con la sesión, 422 si faltan datos, 401 si las credenciales no
// coinciden y 429 si el email acumuló maxLoginFailures fallos en la ventana o
// si no hay cupo para PBKDF2 (TAL-201). Un email inexistente cuenta como
// fallo, para que el bloqueo no revele qué cuentas existen; un login exitoso
// reinicia la cuenta de fallos.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if !decodeAuth(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	var v httpx.Validator
	v.Required("email", input.Email)
	if input.Email != "" {
		v.Check(validEmail(input.Email), "email", "format", "email debe ser un correo válido")
	}
	v.Required("password", input.Password)
	v.Check(len(input.Password) <= maxPasswordBytes, "password", "max_length",
		fmt.Sprintf("password no puede superar %d bytes", maxPasswordBytes))
	if !valid(w, r, &v) {
		return
	}
	if blocked, retryAfter := h.loginFailures.Blocked(input.Email); blocked {
		httpx.WriteRateLimited(w, r, retryAfter, loginBlockedDetail)
		return
	}
	var userID int32
	var hash string
	err := h.db.QueryRow(r.Context(), `SELECT id,password_hash FROM user_account WHERE email=$1 AND active`,
		input.Email).Scan(&userID, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		h.loginFailures.Fail(input.Email)
	}
	if err != nil {
		h.dbError(w, r, err, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	if !h.passwordSlots.Acquire(r.Context()) {
		httpx.WriteRateLimited(w, r, passwordBusyRetry, passwordBusyDetail)
		return
	}
	ok := checkPassword(hash, input.Password)
	h.passwordSlots.Release()
	if !ok {
		h.loginFailures.Fail(input.Email)
		writeError(w, r, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	h.loginFailures.Reset(input.Email)
	h.issueTokens(w, r, userID, http.StatusOK)
}

// ChangePassword atiende POST /auth/change-password: verifica la contraseña
// actual del usuario de la sesión y guarda la nueva con el mismo hash de
// register y login. Recibe currentPassword y newPassword; responde 204 sin
// cuerpo, 401 con un mensaje genérico si la actual no coincide (no revela si
// la cuenta existe), 422 si la nueva no cumple las reglas de registro y 429
// si no hay cupo para PBKDF2 (TAL-201). La sesión la exige el middleware, así
// que el id del usuario sale del token y nunca del body.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := authz.SessionFrom(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized,
			"Se requiere un token de acceso válido (Authorization: Bearer <token>).")
		return
	}
	var input changePasswordInput
	if err := httpx.DecodeJSON(w, r, &input, maxAuthBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}

	var v httpx.Validator
	v.Required("currentPassword", input.CurrentPassword)
	v.Check(len(input.CurrentPassword) <= maxPasswordBytes, "currentPassword", "max_length",
		fmt.Sprintf("currentPassword no puede superar %d bytes", maxPasswordBytes))
	v.Required("newPassword", input.NewPassword)
	v.Check(len(input.NewPassword) >= minPasswordLength, "newPassword", "min",
		fmt.Sprintf("newPassword debe tener al menos %d caracteres", minPasswordLength))
	v.Check(len(input.NewPassword) <= maxPasswordBytes, "newPassword", "max_length",
		fmt.Sprintf("newPassword no puede superar %d bytes", maxPasswordBytes))
	if !valid(w, r, &v) {
		return
	}

	var currentHash string
	err := h.db.QueryRow(r.Context(), `SELECT password_hash FROM user_account WHERE id=$1 AND active`,
		session.UserID).Scan(&currentHash)
	if err != nil {
		h.dbError(w, r, err, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	if !h.passwordSlots.Acquire(r.Context()) {
		httpx.WriteRateLimited(w, r, passwordBusyRetry, passwordBusyDetail)
		return
	}
	matches := checkPassword(currentHash, input.CurrentPassword)
	h.passwordSlots.Release()
	if !matches {
		writeError(w, r, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	if !h.passwordSlots.Acquire(r.Context()) {
		httpx.WriteRateLimited(w, r, passwordBusyRetry, passwordBusyDetail)
		return
	}
	newHash, err := passwordHash(input.NewPassword)
	h.passwordSlots.Release()
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	tag, err := h.db.Exec(r.Context(), `UPDATE user_account SET password_hash=$1, updated_at=NOW()
		WHERE id=$2 AND active`, newHash, session.UserID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, r, http.StatusUnauthorized, "Credenciales inválidas.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// issueTokens crea una sesión para userID y responde con status el access
// token, el perfil y, solo si la request trae X-Refresh-In-Body: 1, también el
// refresh token. Siempre deja el refresh token en la cookie httpOnly, donde
// reemplaza al anterior. Guarda solo el SHA-256 de cada token. Responde 404 si
// el usuario ya no está activo y 500 si falla la base o el generador aleatorio.
func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, userID int32, status int) {
	var (
		name      string
		email     string
		publicID  string
		hasAvatar bool
	)
	if err := h.db.QueryRow(r.Context(), `SELECT name,email,public_id::text,avatar_mime IS NOT NULL
		FROM user_account WHERE id=$1 AND active`,
		userID).Scan(&name, &email, &publicID, &hasAvatar); err != nil {
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
	refreshExpiry := time.Now().Add(refreshTokenLifetime)
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
	setRefreshCookie(w, refresh)
	avatarPath := avatarURL(publicID, hasAvatar)
	payload := map[string]any{"userId": userID, "accessToken": access,
		"name": name, "email": email, "avatarUrl": avatarPath, "tokenType": "Bearer", "expiresIn": 900,
		"refreshExpiresAt": refreshExpiry}
	if wantsRefreshInBody(r) {
		payload["refreshToken"] = refresh
	}
	apiutil.WriteJSON(w, status, payload)
}

// Refresh atiende POST /auth/refresh: canjea un refresh token vigente por un
// par nuevo y borra la sesión anterior (rotación), dejando el token nuevo en
// la cookie. Toma el token del body o, si no viene, de la cookie. Responde 200
// con la sesión nueva, 401 si no hay token (sin sesión), no es válido o venció
// y 400/422 si el body viene mal formado.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input authInput
	if r.Body != nil && r.ContentLength != 0 {
		if !decodeAuth(w, r, &input) {
			return
		}
	}
	token := strings.TrimSpace(input.RefreshToken)
	if token == "" {
		token = refreshCookieToken(r)
	}
	if token == "" {
		writeError(w, r, http.StatusUnauthorized, "No hay una sesión activa.")
		return
	}
	var userID int32
	err := h.db.QueryRow(r.Context(), `DELETE FROM user_session WHERE refresh_token_hash=$1 AND refresh_expires_at>NOW()
		RETURNING user_id`, tokenHash(token)).Scan(&userID)
	if err != nil {
		h.dbError(w, r, err, http.StatusUnauthorized, "El refresh token no es válido o venció.")
		return
	}
	h.issueTokens(w, r, userID, http.StatusOK)
}

// Logout cierra la sesión del bearer token o, si no viene, la del
// refreshToken del body o de la cookie, y siempre vence la cookie. Sin sesión
// que cerrar responde igual 204.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" && r.Body != nil && r.ContentLength != 0 {
		var input authInput
		if !decodeAuth(w, r, &input) {
			return
		}
		token = strings.TrimSpace(input.RefreshToken)
	}
	if token == "" {
		token = refreshCookieToken(r)
	}
	if token != "" {
		if _, err := h.db.Exec(r.Context(), `DELETE FROM user_session WHERE access_token_hash=$1 OR refresh_token_hash=$1`, tokenHash(token)); err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}
	}
	clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// LogoutAll atiende POST /auth/logout-all: cierra todas las sesiones del
// usuario de la sesión, cuyo id sale del token y nunca del body, vence la
// cookie del refresh token y responde 204. Sin sesión responde 401.
func (h *Handler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	session, ok := authz.SessionFrom(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized,
			"Se requiere un token de acceso válido (Authorization: Bearer <token>).")
		return
	}
	if _, err := h.db.Exec(r.Context(), `DELETE FROM user_session WHERE user_id=$1`,
		session.UserID); err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// decodeAuth es decode con un límite de body menor (16 KiB): los endpoints de
// auth son públicos y no necesitan más.
func decodeAuth(w http.ResponseWriter, r *http.Request, input *authInput) bool {
	if err := httpx.DecodeJSON(w, r, input, maxAuthBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return false
	}
	return true
}

// avatarURL devuelve la ruta pública de la foto del usuario, o nil si no
// subió una.
func avatarURL(publicID string, hasAvatar bool) *string {
	if !hasAvatar {
		return nil
	}
	url := "/users/" + publicID + "/avatar"
	return &url
}

// refreshCookieToken devuelve el refresh token de la cookie refresh_token, o
// "" si la request no la trae.
func refreshCookieToken(r *http.Request) string {
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// wantsRefreshInBody indica si la request pidió el refresh token también en el
// JSON (X-Refresh-In-Body: 1). Sin ese header solo viaja en la cookie.
func wantsRefreshInBody(r *http.Request) bool {
	return r.Header.Get("X-Refresh-In-Body") == "1"
}

// refreshCookie arma la cookie del refresh token con los atributos comunes a
// la emisión y al vencimiento: httpOnly, SameSite=Strict y Path=/auth.
func refreshCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/auth",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   refreshCookieSecure(),
		SameSite: http.SameSiteStrictMode,
	}
}

// setRefreshCookie deja el refresh token en la cookie, con la misma vida del
// token. Una emisión nueva reemplaza el valor anterior.
func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, refreshCookie(token, int(refreshTokenLifetime.Seconds())))
}

// clearRefreshCookie vence la cookie con Max-Age=0 (http.Cookie lo serializa
// con MaxAge -1) y los mismos atributos con que se emitió.
func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, refreshCookie("", -1))
}

// refreshCookieSecure indica si la cookie lleva Secure: AUTH_COOKIE_SECURE,
// true por defecto; "false" permite HTTP en desarrollo local.
func refreshCookieSecure() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_COOKIE_SECURE")), "false")
}

// validEmail indica si value es un correo con formato válido, sin nombre
// visible ni espacios y de largo dentro de maxEmailLength.
func validEmail(value string) bool {
	if len(value) == 0 || len(value) > maxEmailLength {
		return false
	}
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && !strings.ContainsAny(value, " \t\r\n")
}

// ValidateToken atiende GET /auth/validate: responde 200 con el userId si el
// bearer token es un access token vigente, o 401 si falta, no existe o venció.
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

// bearerToken devuelve el token del header "Authorization: Bearer <token>",
// o "" si el header falta o usa otro esquema.
func bearerToken(r *http.Request) string {
	value := r.Header.Get("Authorization")
	if len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

// newToken genera un token opaco de 32 bytes aleatorios en base64 URL.
// Devuelve error solo si falla el generador del sistema.
func newToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// tokenHash devuelve el SHA-256 del token: es lo único que se guarda en
// user_session, así una fuga de la base no expone sesiones usables.
func tokenHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

// passwordHash deriva el hash de password con PBKDF2-SHA256, sal aleatoria
// de 16 bytes y passwordIterations iteraciones. Devuelve el texto
// "pbkdf2-sha256$<iteraciones>$<sal>$<hash>" que se guarda en user_account.
func passwordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := pbkdf2([]byte(password), salt, passwordIterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)), nil
}

// checkPassword indica si password corresponde al hash encoded (formato de
// passwordHash). Rechaza formatos desconocidos e iteraciones fuera de
// [100000, 1000000], y compara en tiempo constante.
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

// pbkdf2 implementa PBKDF2 (RFC 8018) con HMAC-SHA256: deriva keyLength
// bytes de password y salt con iterations rondas. Ver H15 de TAL-195.
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

// hmacSHA256 calcula HMAC-SHA256 (RFC 2104) de data con key; es la función
// pseudoaleatoria de pbkdf2.
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
