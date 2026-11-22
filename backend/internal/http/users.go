package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/authz"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// maxProfileNameLength es el largo máximo de name; coincide con
	// user_account.name VARCHAR(150) y con la regla de register.
	maxProfileNameLength = 150
	// maxAvatarBytes es el máximo de la imagen (2 MB) y maxAvatarBodyBytes
	// deja espacio para el sobre multipart (boundaries y cabeceras).
	maxAvatarBytes     = 2 << 20
	maxAvatarBodyBytes = maxAvatarBytes + (512 << 10)
)

// allowedAvatarMIMEs son los tipos que acepta la foto de perfil. Se decide por
// el contenido real (http.DetectContentType), no por el nombre ni el header.
var allowedAvatarMIMEs = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// profileDB es lo que los endpoints de perfil usan de PostgreSQL. *pgxpool.Pool
// lo implementa; los tests pasan un fake (TAL-78 §5).
type profileDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// profileHandler atiende el nombre y la foto del usuario autenticado. La
// sesión llega en el contexto que deja authz.RequireSession.
type profileHandler struct {
	db     profileDB
	logger *slog.Logger
}

// newProfileHandler arma el handler de perfil con la base y el logger dados.
func newProfileHandler(db profileDB, logger *slog.Logger) *profileHandler {
	return &profileHandler{db: db, logger: logger}
}

// profileResponse es el usuario que devuelve PATCH /users/me. avatarUrl es
// null mientras el usuario no haya subido una foto.
type profileResponse struct {
	ID        int32   `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatarUrl"`
}

// PatchMe atiende PATCH /users/me: actualiza el nombre del usuario de la
// sesión. Recibe {"name"} y responde 200 con el usuario actualizado, 401 sin
// sesión, 422 si el nombre está vacío o supera maxProfileNameLength y 404 si
// la cuenta ya no está activa. El id sale de la sesión, nunca del body.
func (h *profileHandler) PatchMe(w http.ResponseWriter, r *http.Request) {
	session, ok := authz.SessionFrom(r.Context())
	if !ok {
		writeProfileUnauthorized(w, r)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(w, r, &input, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteDecodeError(w, r, err)
		return
	}
	input.Name = strings.TrimSpace(input.Name)

	var v httpx.Validator
	v.Required("name", input.Name)
	v.MaxLength("name", input.Name, maxProfileNameLength)
	if !v.Valid() {
		httpx.WriteValidation(w, r, v.Errors())
		return
	}

	var (
		id        int32
		name      string
		email     string
		publicID  string
		hasAvatar bool
	)
	err := h.db.QueryRow(r.Context(), `UPDATE user_account SET name=$1, updated_at=NOW()
		WHERE id=$2 AND active
		RETURNING id, name, email, public_id::text, avatar_mime IS NOT NULL`,
		input.Name, session.UserID).Scan(&id, &name, &email, &publicID, &hasAvatar)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Usuario no encontrado.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, profileResponse{
		ID: id, Name: name, Email: email, AvatarURL: avatarURL(publicID, hasAvatar),
	})
}

// UploadAvatar atiende POST /users/me/avatar: guarda la foto del usuario de la
// sesión en la misma fila de user_account (avatar_data y avatar_mime). Recibe
// multipart/form-data con el campo "avatar"; acepta JPG, PNG o WebP de hasta
// 2 MB y responde 200 con {"avatarUrl"}. Responde 401 sin sesión, 415 si no es
// multipart, 413 si el body excede el sobre permitido, 422 si falta el archivo,
// está vacío, supera 2 MB o no es una imagen permitida, y 404 si la cuenta ya
// no está activa.
func (h *profileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	session, ok := authz.SessionFrom(r.Context())
	if !ok {
		writeProfileUnauthorized(w, r)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		httpx.WriteProblem(w, r, http.StatusUnsupportedMediaType, httpx.CodeUnsupportedMediaType,
			"El Content-Type debe ser multipart/form-data.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBodyBytes)
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpx.WriteProblem(w, r, http.StatusRequestEntityTooLarge, httpx.CodeBodyTooLarge,
				fmt.Sprintf("La imagen no puede superar %d MB.", maxAvatarBytes>>20))
			return
		}
		writeAvatarError(w, r, "required", "La solicitud debe incluir el archivo avatar.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeAvatarError(w, r, "required", "La solicitud debe incluir el archivo avatar.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if len(data) == 0 {
		writeAvatarError(w, r, "required", "El archivo avatar está vacío.")
		return
	}
	if len(data) > maxAvatarBytes {
		writeAvatarError(w, r, "max_size",
			fmt.Sprintf("La imagen no puede superar %d MB.", maxAvatarBytes>>20))
		return
	}
	mimeType := http.DetectContentType(data)
	if !allowedAvatarMIMEs[mimeType] {
		writeAvatarError(w, r, "format", "La imagen debe ser JPG, PNG o WebP.")
		return
	}

	var publicID string
	err = h.db.QueryRow(r.Context(), `UPDATE user_account SET avatar_data=$1, avatar_mime=$2, updated_at=NOW()
		WHERE id=$3 AND active RETURNING public_id::text`, data, mimeType, session.UserID).Scan(&publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Usuario no encontrado.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, map[string]string{"avatarUrl": "/users/" + publicID + "/avatar"})
}

// GetAvatar atiende GET /users/{publicID}/avatar: sirve la foto de perfil sin
// exigir sesión, porque un <img> no puede enviar el bearer token. Recibe el
// public_id UUID del usuario y responde la imagen con ETag y Cache-Control; si
// el cliente ya tiene la misma versión (If-None-Match) responde 304, y 404 si
// el UUID no existe, no es válido o el usuario no tiene foto.
func (h *profileHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	var id pgtype.UUID
	if err := id.Scan(chi.URLParam(r, "publicID")); err != nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Avatar no encontrado.")
		return
	}
	var data []byte
	var mimeType string
	err := h.db.QueryRow(r.Context(), `SELECT avatar_data, avatar_mime FROM user_account
		WHERE public_id=$1 AND active AND avatar_data IS NOT NULL`, id).Scan(&data, &mimeType)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteProblem(w, r, http.StatusNotFound, httpx.CodeNotFound, "Avatar no encontrado.")
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	etag := avatarETag(data)
	w.Header().Set("ETag", etag)
	// La URL no cambia al subir otra foto: el navegador debe revalidar siempre
	// con el ETag. Con max-age servia la foto anterior hasta 5 minutos.
	w.Header().Set("Cache-Control", "public, no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// avatarURL devuelve la ruta pública de la foto, o nil si el usuario no tiene.
// Usa el public_id UUID para no exponer el id secuencial de la tabla.
func avatarURL(publicID string, hasAvatar bool) *string {
	if !hasAvatar {
		return nil
	}
	url := "/users/" + publicID + "/avatar"
	return &url
}

// avatarETag calcula el ETag fuerte de la imagen con el SHA-256 de su
// contenido: cambia solo cuando la foto cambia.
func avatarETag(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// writeAvatarError responde 422 con el error del campo avatar.
func writeAvatarError(w http.ResponseWriter, r *http.Request, code, message string) {
	httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "avatar", Code: code, Message: message}})
}

// writeProfileUnauthorized responde 401 cuando falta la sesión. El middleware
// ya la exige; esto cubre el caso defensivo de un handler montado sin él.
func writeProfileUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.WriteProblem(w, r, http.StatusUnauthorized, httpx.CodeUnauthorized,
		"Se requiere un token de acceso válido (Authorization: Bearer <token>).")
}
