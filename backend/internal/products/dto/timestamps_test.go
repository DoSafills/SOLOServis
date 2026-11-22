package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// testInstant es una marca de tiempo fija para que las aserciones no dependan
// del reloj ni de la zona horaria de la máquina.
var testInstant = time.Date(2026, 11, 22, 15, 4, 5, 0, time.UTC)

// TestFormatTime comprueba la conversión de las columnas de PostgreSQL al
// formato que consume el frontend: RFC 3339 en UTC, y cadena vacía cuando la
// columna es nula.
func TestFormatTime(t *testing.T) {
	cases := []struct {
		name  string
		value time.Time
		valid bool
		want  string
	}{
		{"válida", testInstant, true, "2026-11-22T15:04:05Z"},
		{"nula", time.Time{}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatTime(tc.value, tc.valid); got != tc.want {
				t.Fatalf("formatTime() = %q, quiero %q", got, tc.want)
			}
		})
	}
}

// TestProductListItemExposesUpdatedAt verifica que TAL-237 llegue al JSON del
// listado: el campo `updatedAt` con la fecha del producto. Es el nombre que ya
// espera el frontend en `fronted/src/types.ts`.
func TestProductListItemExposesUpdatedAt(t *testing.T) {
	row := generated.ListProductsRow{
		Name:      "Refrigerador de prueba",
		UpdatedAt: pgtype.Timestamp{Time: testInstant, Valid: true},
	}

	item := FromListProduct(row)
	if item.UpdatedAt != "2026-11-22T15:04:05Z" {
		t.Fatalf("UpdatedAt = %q", item.UpdatedAt)
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("json: %v", err)
	}
	if generic["updatedAt"] != "2026-11-22T15:04:05Z" {
		t.Fatalf("la clave updatedAt no está en el JSON: %s", data)
	}
}

// TestProductDetailExposesUpdatedAt hace lo mismo con el detalle del producto,
// que es la pantalla donde el usuario ve «última actualización».
func TestProductDetailExposesUpdatedAt(t *testing.T) {
	row := generated.GetProductDetailByPublicIDRow{
		Name:      "Refrigerador de prueba",
		UpdatedAt: pgtype.Timestamp{Time: testInstant, Valid: true},
	}

	detail := FromProduct(row)
	if detail.UpdatedAt != "2026-11-22T15:04:05Z" {
		t.Fatalf("UpdatedAt = %q", detail.UpdatedAt)
	}
}

// TestProductOfferExposesLastUpdated comprueba la fecha de verificación de la
// oferta, que es la que dice si el precio es de hoy o de hace dos semanas.
func TestProductOfferExposesLastUpdated(t *testing.T) {
	row := generated.ListProductOffersRow{
		StoreName:   "Falabella",
		LastUpdated: pgtype.Timestamptz{Time: testInstant, Valid: true},
	}

	offer := FromProductOffer(row)
	if offer.LastUpdated != "2026-11-22T15:04:05Z" {
		t.Fatalf("LastUpdated = %q", offer.LastUpdated)
	}

	data, err := json.Marshal(offer)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("json: %v", err)
	}
	if generic["lastUpdated"] != "2026-11-22T15:04:05Z" {
		t.Fatalf("la clave lastUpdated no está en el JSON: %s", data)
	}
}
