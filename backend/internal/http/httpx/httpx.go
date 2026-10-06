// Package httpx reúne la lectura y escritura de JSON y el parseo de
// parámetros que comparten los handlers de todos los dominios.
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}

// DecodeJSON rechaza campos desconocidos para detectar payloads mal formados.
func DecodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(target)
}

// UUIDParam lee un parámetro de ruta con formato UUID. Si no es válido,
// responde 400 con invalidMessage y devuelve false.
func UUIDParam(w http.ResponseWriter, r *http.Request, name string, invalidMessage string) (pgtype.UUID, bool) {
	var uuid pgtype.UUID

	if err := uuid.Scan(chi.URLParam(r, name)); err != nil {
		http.Error(w, invalidMessage, http.StatusBadRequest)
		return uuid, false
	}

	return uuid, true
}
