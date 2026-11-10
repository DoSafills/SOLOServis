package products

import (
	"context"
	"errors"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/jackc/pgx/v5/pgtype"
)

// Errores de dominio que el handler traduce a HTTP (TAL-78 §3.5).
var (
	ErrNotFound         = errors.New("product not found")
	ErrCategoryNotFound = errors.New("category not found")
)

// Límites de paginación de GET /products. El cuerpo sigue siendo un arreglo
// (lo consumen Search API y el frontend); el total va en X-Total-Count.
const (
	DefaultListLimit = 100
	MaxListLimit     = 100
)

// ListParams son los filtros ya validados de GET /products.
type ListParams struct {
	LegacyCategoryID *int32  // product_category, incluye subcategorías
	CategorySlug     *string // categories (TAL-71)
	Limit            int
	Offset           int
}

// CategoryRef es la categoría tal como llega en el request: slug del modelo
// nuevo o id de product_category (modelo anterior). Uno de los dos.
type CategoryRef struct {
	Slug     string
	LegacyID int32
}

// WriteCategory es la categoría resuelta para guardar un producto.
type WriteCategory struct {
	ID         *int32 // categories.id → product.category_ref; nil sin equivalente nuevo
	Slug       *string
	ItemType   string
	LegacyID   *int32 // product_category.id → product.category_id (NOT NULL)
	Attributes []categories.Attribute
}

// ProductWrite son los datos validados de un alta o edición.
type ProductWrite struct {
	LegacyCategoryID int32
	CategoryRef      *int32
	BrandID          *int32
	Name             string
	Model            string
	SKU              string
	Description      string
	ImageURL         string // solo en creación
	Attributes       []categories.Value
}

// Store es lo que el handler necesita de la capa de datos; *Repository la
// implementa y los tests usan un fake en memoria.
type Store interface {
	List(ctx context.Context, params ListParams) ([]dto.ProductListItem, int64, error)
	GetDetail(ctx context.Context, publicID pgtype.UUID) (dto.ProductDetail, error)
	ResolveCategory(ctx context.Context, ref CategoryRef) (WriteCategory, error)
	Create(ctx context.Context, product ProductWrite) (pgtype.UUID, error)
	Update(ctx context.Context, publicID pgtype.UUID, product ProductWrite) error
	Deactivate(ctx context.Context, publicID pgtype.UUID) error
	ListCategories(ctx context.Context) ([]generated.ListProductCategoriesRow, error)
	ListReviewsByPublicID(ctx context.Context, publicID pgtype.UUID) ([]generated.ListProductReviewsRow, error)
}
