package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader es el header de correlación aceptado y devuelto.
const RequestIDHeader = "X-Request-ID"

// MaxRequestIDLength es el largo máximo aceptado desde el cliente.
const MaxRequestIDLength = 64

type requestIDKey struct{}

// RequestID acepta el X-Request-ID del cliente solo si es válido; si no,
// genera uno nuevo. Lo guarda en el contexto y lo devuelve en la respuesta.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !ValidRequestID(id) {
			id = newRequestID()
		}

		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom devuelve el request_id del contexto, o "" si no hay.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// ValidRequestID exige 1..64 caracteres del conjunto [A-Za-z0-9-].
func ValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > MaxRequestIDLength {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		isAlnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlnum && c != '-' {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [16]byte
	// crypto/rand.Read no falla en plataformas soportadas (Go >= 1.24).
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
