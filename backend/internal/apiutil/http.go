package apiutil

import (
	"encoding/json"
	"net/http"
)

// WriteJSON responde value como JSON con el código de estado indicado. Para
// leer bodies se usa httpx.DecodeJSON y para errores httpx.WriteProblem
// (estándar TAL-78); aquí no hay equivalentes para no saltarse esas reglas.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
