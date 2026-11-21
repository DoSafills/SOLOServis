package categories

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Level es el nivel de una categoría del árbol de product_category: 1 (raíz),
// 2 (grupo) o 3 (hoja). Solo las hojas pueden tener productos.
type Level int

const (
	Level1 Level = 1
	Level2 Level = 2
	Level3 Level = 3
)

// Ref identifica una categoría del árbol para resolver su alcance. Se debe
// informar exactamente uno de los dos campos:
//   - ID: id interno de product_category;
//   - Slug: slug de product_category.
type Ref struct {
	ID   *int32
	Slug *string
}

// Scope es el resultado de ResolveScope: el nivel y el nombre de la categoría
// pedida y los ids de sus hojas activas (nivel 3), incluida ella misma cuando
// ya es una hoja. CategoryIDs vacío = alcance sin resultados.
type Scope struct {
	Level       Level
	Name        string
	CategoryIDs []int32
}

// ErrInvalidScopeRef indica que Ref no trae exactamente un campo.
var ErrInvalidScopeRef = errors.New("scope ref must set exactly one of id or slug")

// ParseScopeParam interpreta el parámetro de consulta category de Products y
// Search: un número positivo es un id de product_category y el resto, un slug
// del árbol. Devuelve el Ref a resolver y, si el valor no sirve, el código del
// error de campo: "min" para un id <= 0 y "format" para lo demás. La
// resolución contra la base la hace ResolveScope.
func ParseScopeParam(value string) (Ref, string) {
	value = strings.TrimSpace(value)
	if id, err := strconv.ParseInt(value, 10, 32); err == nil {
		if id <= 0 {
			return Ref{}, "min"
		}
		categoryID := int32(id)
		return Ref{ID: &categoryID}, ""
	}
	if ValidSlug(value) {
		slug := value
		return Ref{Slug: &slug}, ""
	}
	return Ref{}, "format"
}

// ScopeStore es lo que necesita ResolveScope; *PostgresRepository lo
// implementa y los tests usan un fake en memoria.
type ScopeStore interface {
	CategoryScope(ctx context.Context, ref Ref) (Scope, error)
}

// categoryScopeSQL resuelve una categoría activa (por id o slug) y devuelve su
// nombre, nivel y los ids de las hojas activas de su subárbol. No hay cuarto
// nivel: la recursión baja solo por parent_category_id y una hoja se devuelve a
// sí misma. Un id o slug inexistente o inactivo deja las tres columnas en NULL.
const categoryScopeSQL = `
WITH RECURSIVE node AS (
    SELECT pc.id, pc.name, pc.level
    FROM product_category pc
    WHERE pc.active
      AND (($1::int IS NOT NULL AND pc.id = $1::int)
        OR ($2::text IS NOT NULL AND pc.slug = $2::text))
    LIMIT 1
), subtree AS (
    SELECT node.id, node.level
    FROM node
    UNION ALL
    SELECT child.id, child.level
    FROM product_category child
    JOIN subtree parent ON child.parent_category_id = parent.id
    WHERE child.active
)
SELECT (SELECT name FROM node),
       (SELECT level FROM node),
       (SELECT array_agg(subtree.id ORDER BY subtree.id) FROM subtree WHERE subtree.level = 3)`

// ResolveScope devuelve el nivel, el nombre y las hojas del subárbol de la
// categoría indicada, aceptando cualquier nivel del árbol. Una categoría
// inexistente o inactiva devuelve ErrNotFound (nunca un alcance vacío
// silencioso).
func ResolveScope(ctx context.Context, store ScopeStore, ref Ref) (Scope, error) {
	if refCount(ref) != 1 {
		return Scope{}, ErrInvalidScopeRef
	}
	return store.CategoryScope(ctx, ref)
}

// ResolveScope resuelve el alcance con la base de datos del repositorio.
func (r *PostgresRepository) ResolveScope(ctx context.Context, ref Ref) (Scope, error) {
	return ResolveScope(ctx, r, ref)
}

// CategoryScope consulta el subárbol de una categoría activa y lo devuelve
// como Scope. Un id o slug sin fila activa responde ErrNotFound.
func (r *PostgresRepository) CategoryScope(ctx context.Context, ref Ref) (Scope, error) {
	var (
		name  *string
		level *int16
		ids   []int32
	)
	if err := r.db.QueryRow(ctx, categoryScopeSQL, ref.ID, ref.Slug).Scan(&name, &level, &ids); err != nil {
		return Scope{}, err
	}
	if name == nil || level == nil {
		return Scope{}, fmt.Errorf("%w: la categoría %s no existe o está inactiva", ErrNotFound, describeRef(ref))
	}
	if ids == nil {
		ids = []int32{}
	}
	return Scope{Level: Level(*level), Name: *name, CategoryIDs: ids}, nil
}

// describeRef describe el Ref para los mensajes de error.
func describeRef(ref Ref) string {
	if ref.ID != nil {
		return fmt.Sprintf("id %d", *ref.ID)
	}
	return fmt.Sprintf("slug %q", *ref.Slug)
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
	return count
}
