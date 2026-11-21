package categories

import (
	"context"
	"errors"
	"fmt"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// Level identifica el nivel del ref dentro del árbol de exactamente tres
// niveles (NV1 > NV2 > NV3) o el modelo legacy (product_category).
type Level string

const (
	LevelNV1    Level = "nv1"
	LevelNV2    Level = "nv2"
	LevelNV3    Level = "nv3"
	LevelLegacy Level = "legacy"
)

// Ref identifica una categoría para resolver su alcance. Se debe informar
// exactamente uno de los tres campos:
//   - ID: id interno de una NV1, NV2 o NV3;
//   - Slug: slug de una NV1, NV2 o NV3;
//   - LegacyID: id de product_category, traducido por el puente de 034.
type Ref struct {
	ID       *int32
	Slug     *string
	LegacyID *int32
}

// Scope es el resultado de ResolveScope: el nivel y el nombre de la
// categoría pedida y los ids de sus NV3 activas descendientes (solo ella
// misma cuando la categoría ya es una NV3).
type Scope struct {
	Level  Level
	Name   string
	NV3IDs []int32
}

var (
	// ErrInvalidScopeRef indica que Ref no trae exactamente un campo.
	ErrInvalidScopeRef = errors.New("scope ref must set exactly one of id, slug or legacy id")
	// ErrAmbiguousScopeRef indica que el id o el slug coincide con más de
	// una categoría (los ids de las tres tablas son independientes).
	ErrAmbiguousScopeRef = errors.New("scope ref matches more than one category")
)

// ScopeStore son las consultas que necesita ResolveScope; *PostgresRepository
// las implementa y los tests usan un fake en memoria.
type ScopeStore interface {
	ResolveCategoryNVScope(ctx context.Context, params generated.ResolveCategoryNVScopeParams) ([]generated.ResolveCategoryNVScopeRow, error)
	ResolveCategoryNVScopeByLegacy(ctx context.Context, legacyID int32) ([]generated.ResolveCategoryNVScopeByLegacyRow, error)
}

// ResolveScope devuelve el nivel, el nombre y las NV3 descendientes de la
// categoría indicada. Acepta NV1, NV2 y NV3 (nunca un cuarto nivel) y, por
// LegacyID, un id de product_category que se traduce recorriendo su subárbol
// y mapeando las hojas por categories_nv3.legacy_product_category_id.
// Una categoría inexistente o inactiva devuelve ErrNotFound (nunca un
// alcance vacío silencioso).
func ResolveScope(ctx context.Context, store ScopeStore, ref Ref) (Scope, error) {
	if refCount(ref) != 1 {
		return Scope{}, ErrInvalidScopeRef
	}
	if ref.LegacyID != nil {
		return resolveLegacyScope(ctx, store, *ref.LegacyID)
	}
	return resolveNVScope(ctx, store, ref)
}

// ResolveScope resuelve el alcance con la base de datos del repositorio.
func (r *PostgresRepository) ResolveScope(ctx context.Context, ref Ref) (Scope, error) {
	return ResolveScope(ctx, r, ref)
}

// ResolveCategoryNVScope delega la consulta de alcance NV en sqlc.
func (r *PostgresRepository) ResolveCategoryNVScope(ctx context.Context, params generated.ResolveCategoryNVScopeParams) ([]generated.ResolveCategoryNVScopeRow, error) {
	return r.queries.ResolveCategoryNVScope(ctx, params)
}

// ResolveCategoryNVScopeByLegacy delega la consulta de alcance legacy en sqlc.
func (r *PostgresRepository) ResolveCategoryNVScopeByLegacy(ctx context.Context, legacyID int32) ([]generated.ResolveCategoryNVScopeByLegacyRow, error) {
	return r.queries.ResolveCategoryNVScopeByLegacy(ctx, legacyID)
}

// refCount cuenta cuántos identificadores trae Ref.
func refCount(ref Ref) int {
	count := 0
	if ref.ID != nil {
		count++
	}
	if ref.Slug != nil {
		count++
	}
	if ref.LegacyID != nil {
		count++
	}
	return count
}

// resolveNVScope resuelve un id o slug de NV1, NV2 o NV3.
func resolveNVScope(ctx context.Context, store ScopeStore, ref Ref) (Scope, error) {
	var params generated.ResolveCategoryNVScopeParams
	var description string
	if ref.ID != nil {
		params.ID = pgtype.Int4{Int32: *ref.ID, Valid: true}
		description = fmt.Sprintf("id %d", *ref.ID)
	} else {
		params.Slug = pgtype.Text{String: *ref.Slug, Valid: true}
		description = fmt.Sprintf("slug %q", *ref.Slug)
	}

	rows, err := store.ResolveCategoryNVScope(ctx, params)
	if err != nil {
		return Scope{}, err
	}
	if len(rows) == 0 {
		return Scope{}, fmt.Errorf("%w: la categoría %s no existe o está inactiva", ErrNotFound, description)
	}

	type matchKey struct {
		level string
		id    int32
	}
	matches := make(map[matchKey]struct{}, 1)
	for _, row := range rows {
		matches[matchKey{level: row.Level, id: row.RefID}] = struct{}{}
	}
	if len(matches) > 1 {
		return Scope{}, fmt.Errorf("%w: %s coincide con más de una categoría del árbol", ErrAmbiguousScopeRef, description)
	}

	scope := Scope{
		Level:  Level(rows[0].Level),
		Name:   rows[0].RefName,
		NV3IDs: make([]int32, 0, len(rows)),
	}
	for _, row := range rows {
		if row.Nv3ID.Valid {
			scope.NV3IDs = append(scope.NV3IDs, row.Nv3ID.Int32)
		}
	}
	return scope, nil
}

// resolveLegacyScope traduce un id de product_category a las NV3 alcanzadas
// por el puente, recorriendo su subárbol legacy.
func resolveLegacyScope(ctx context.Context, store ScopeStore, legacyID int32) (Scope, error) {
	rows, err := store.ResolveCategoryNVScopeByLegacy(ctx, legacyID)
	if err != nil {
		return Scope{}, err
	}
	if len(rows) == 0 {
		return Scope{}, fmt.Errorf("%w: la categoría legacy id %d no existe o está inactiva", ErrNotFound, legacyID)
	}

	scope := Scope{
		Level:  LevelLegacy,
		Name:   rows[0].RefName,
		NV3IDs: make([]int32, 0, len(rows)),
	}
	for _, row := range rows {
		if row.Nv3ID.Valid {
			scope.NV3IDs = append(scope.NV3IDs, row.Nv3ID.Int32)
		}
	}
	return scope, nil
}
