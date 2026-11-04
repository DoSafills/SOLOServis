package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// DefaultMaxBodyBytes es el límite de body por defecto (1 MB).
const DefaultMaxBodyBytes int64 = 1 << 20

// DecodeError es un fallo de decodificación ya clasificado (400, 413 o 415).
type DecodeError struct {
	Status int
	Code   string
	Detail string
	Field  string
}

func (e *DecodeError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (field %s)", e.Code, e.Detail, e.Field)
	}
	return e.Code + ": " + e.Detail
}

// DecodeJSON decodifica un único objeto JSON del body en dst.
//
// Exige Content-Type application/json (415), limita el body a maxBytes (413;
// usa DefaultMaxBodyBytes si maxBytes <= 0) y rechaza JSON malformado, body
// vacío, datos extra, campos desconocidos y tipos incorrectos (400).
// Las reglas de negocio (422) no son responsabilidad del decodificador.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		return &DecodeError{
			Status: http.StatusUnsupportedMediaType,
			Code:   CodeUnsupportedMediaType,
			Detail: "El Content-Type debe ser application/json.",
		}
	}

	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return classifyDecodeError(err, maxBytes)
	}

	// Solo se admite un valor JSON por body.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if tooLarge := asTooLarge(err, maxBytes); tooLarge != nil {
			return tooLarge
		}
		return &DecodeError{
			Status: http.StatusBadRequest,
			Code:   CodeMalformedJSON,
			Detail: "El body debe contener un único objeto JSON.",
		}
	}

	return nil
}

// WriteDecodeError responde el error devuelto por DecodeJSON.
func WriteDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		decodeErr = &DecodeError{Status: http.StatusBadRequest, Code: CodeMalformedJSON, Detail: "El body no es JSON válido."}
	}

	var fieldErrors []FieldError
	if decodeErr.Field != "" {
		fieldErrors = []FieldError{{Field: decodeErr.Field, Code: decodeErr.Code, Message: decodeErr.Detail}}
	}
	WriteProblem(w, r, decodeErr.Status, decodeErr.Code, decodeErr.Detail, fieldErrors...)
}

func isJSONContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func classifyDecodeError(err error, maxBytes int64) *DecodeError {
	if tooLarge := asTooLarge(err, maxBytes); tooLarge != nil {
		return tooLarge
	}

	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError

	switch {
	case errors.Is(err, io.EOF):
		return &DecodeError{Status: http.StatusBadRequest, Code: CodeMalformedJSON, Detail: "El body está vacío."}
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return &DecodeError{Status: http.StatusBadRequest, Code: CodeMalformedJSON, Detail: "El body no es JSON válido."}
	case errors.As(err, &typeErr):
		field := typeErr.Field
		return &DecodeError{
			Status: http.StatusBadRequest,
			Code:   CodeInvalidType,
			Detail: fmt.Sprintf("El campo %s tiene un tipo inválido.", field),
			Field:  field,
		}
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		// encoding/json no expone un tipo para este error; el formato es estable.
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return &DecodeError{
			Status: http.StatusBadRequest,
			Code:   CodeUnknownField,
			Detail: fmt.Sprintf("El campo %s no está permitido.", field),
			Field:  field,
		}
	default:
		return &DecodeError{Status: http.StatusBadRequest, Code: CodeMalformedJSON, Detail: "El body no es JSON válido."}
	}
}

func asTooLarge(err error, maxBytes int64) *DecodeError {
	var maxErr *http.MaxBytesError
	if !errors.As(err, &maxErr) {
		return nil
	}
	return &DecodeError{
		Status: http.StatusRequestEntityTooLarge,
		Code:   CodeBodyTooLarge,
		Detail: fmt.Sprintf("El body supera el máximo de %d bytes.", maxBytes),
	}
}
