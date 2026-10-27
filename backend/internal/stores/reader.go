package stores

import (
	"context"
	"errors"

	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/jackc/pgx/v5/pgtype"
)

// Errores de dominio que el handler traduce a 404 (TAL-78 §3.5).
var (
	ErrNotFound        = errors.New("store not found")
	ErrProductNotFound = errors.New("product not found")
)

// Límites de paginación de los listados de Store API. Los cuerpos siguen
// siendo arreglos; el total va en el header X-Total-Count.
const (
	DefaultLimit = 100
	MaxLimit     = 100
)

// Page son los parámetros de paginación ya validados.
type Page struct {
	Limit  int
	Offset int
}

// OfferFilter filtra ofertas por disponibilidad (nil = todas) y pagina.
type OfferFilter struct {
	Available *bool
	Page
}

// Reader es lo que el handler necesita de la capa de datos (solo lectura).
// *Repository la implementa y los tests usan un fake en memoria.
type Reader interface {
	List(ctx context.Context, page Page) ([]dto.Store, int64, error)
	GetDetail(ctx context.Context, storeID int32) (dto.StoreDetail, error)
	ListProductOffers(ctx context.Context, productID pgtype.UUID, filter OfferFilter) ([]dto.Offer, int64, error)
	ListStoreOffers(ctx context.Context, storeID int32, filter OfferFilter) ([]dto.Offer, int64, error)
}
