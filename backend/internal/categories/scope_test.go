package categories

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// fakeNV1 es una fila de categories_nv1 en el fake.
type fakeNV1 struct {
	id   int32
	name string
	slug string
}

// fakeNV2 es una fila de categories_nv2 en el fake.
type fakeNV2 struct {
	id    int32
	nv1ID int32
	name  string
	slug  string
}

// fakeNV3 es una fila de categories_nv3 en el fake; legacyID es el puente.
type fakeNV3 struct {
	id       int32
	nv2ID    int32
	name     string
	slug     string
	legacyID *int32
}

// fakeLegacy es una fila de product_category en el fake.
type fakeLegacy struct {
	id       int32
	parentID *int32
	name     string
}

// fakeScopeStore imita las dos consultas de alcance sobre un árbol en memoria.
type fakeScopeStore struct {
	nv1    []fakeNV1
	nv2    []fakeNV2
	nv3    []fakeNV3
	legacy []fakeLegacy
}

// newFakeScopeStore arma el árbol de prueba: Hogar > Electrodomésticos >
// Refrigeradores, Cocina; Tecnología > Computación > Notebooks, Accesorios
// (sin puente) y Tecnología > Pantallas (sin NV3). El legacy espeja esas
// ramas con una hoja extra Microondas bajo Cocina.
func newFakeScopeStore() *fakeScopeStore {
	legacyID := func(id int32) *int32 { return &id }
	parentID := func(id int32) *int32 { return &id }
	return &fakeScopeStore{
		nv1: []fakeNV1{
			{id: 1, name: "Hogar", slug: "hogar"},
			{id: 2, name: "Tecnología", slug: "tecnologia"},
		},
		nv2: []fakeNV2{
			{id: 101, nv1ID: 1, name: "Electrodomésticos", slug: "electrodomesticos"},
			{id: 102, nv1ID: 2, name: "Computación", slug: "computacion"},
			{id: 103, nv1ID: 2, name: "Pantallas", slug: "pantallas"},
			{id: 104, nv1ID: 1, name: "Computación (duplicado)", slug: "computacion"},
		},
		nv3: []fakeNV3{
			{id: 201, nv2ID: 101, name: "Refrigeradores", slug: "refrigeradores", legacyID: legacyID(11)},
			{id: 202, nv2ID: 101, name: "Cocina", slug: "cocina", legacyID: legacyID(12)},
			{id: 203, nv2ID: 102, name: "Notebooks", slug: "notebooks", legacyID: legacyID(22)},
			{id: 204, nv2ID: 102, name: "Accesorios de computación", slug: "accesorios-de-computacion"},
		},
		legacy: []fakeLegacy{
			{id: 10, name: "Electrodomesticos"},
			{id: 11, parentID: parentID(10), name: "Refrigeradores"},
			{id: 12, parentID: parentID(10), name: "Cocina"},
			{id: 13, parentID: parentID(12), name: "Microondas"},
			{id: 20, name: "Tecnología"},
			{id: 21, parentID: parentID(20), name: "Computación"},
			{id: 22, parentID: parentID(21), name: "Notebooks"},
		},
	}
}

// ResolveCategoryNVScope imita la consulta NV: busca por id o slug en los
// tres niveles y devuelve una fila por NV3 descendiente.
func (f *fakeScopeStore) ResolveCategoryNVScope(_ context.Context, params generated.ResolveCategoryNVScopeParams) ([]generated.ResolveCategoryNVScopeRow, error) {
	var rows []generated.ResolveCategoryNVScopeRow
	for _, node := range f.nv1 {
		if matchesScope(params, node.id, node.slug) {
			rows = append(rows, f.scopeRows(LevelNV1, node.id, node.name)...)
		}
	}
	for _, node := range f.nv2 {
		if matchesScope(params, node.id, node.slug) {
			rows = append(rows, f.scopeRows(LevelNV2, node.id, node.name)...)
		}
	}
	for _, node := range f.nv3 {
		if matchesScope(params, node.id, node.slug) {
			rows = append(rows, f.scopeRows(LevelNV3, node.id, node.name)...)
		}
	}
	return rows, nil
}

// ResolveCategoryNVScopeByLegacy imita la consulta legacy: recorre el
// subárbol y devuelve una fila por NV3 enlazada por el puente.
func (f *fakeScopeStore) ResolveCategoryNVScopeByLegacy(_ context.Context, legacyID int32) ([]generated.ResolveCategoryNVScopeByLegacyRow, error) {
	var name string
	found := false
	for _, node := range f.legacy {
		if node.id == legacyID {
			name = node.name
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}

	subtree := f.legacySubtree(legacyID)
	var leaves []fakeNV3
	for _, leaf := range f.nv3 {
		if leaf.legacyID != nil && subtree[*leaf.legacyID] {
			leaves = append(leaves, leaf)
		}
	}
	sort.Slice(leaves, func(i, j int) bool { return leaves[i].id < leaves[j].id })

	if len(leaves) == 0 {
		return []generated.ResolveCategoryNVScopeByLegacyRow{{Level: string(LevelLegacy), RefName: name}}, nil
	}
	rows := make([]generated.ResolveCategoryNVScopeByLegacyRow, 0, len(leaves))
	for _, leaf := range leaves {
		rows = append(rows, generated.ResolveCategoryNVScopeByLegacyRow{
			Level:   string(LevelLegacy),
			RefName: name,
			Nv3ID:   pgtype.Int4{Int32: leaf.id, Valid: true},
			Nv3Name: pgtype.Text{String: leaf.name, Valid: true},
		})
	}
	return rows, nil
}

// scopeRows devuelve las NV3 del alcance de un nodo; sin descendientes
// activos devuelve una fila con nv3_id NULL, igual que la consulta real.
func (f *fakeScopeStore) scopeRows(level Level, refID int32, refName string) []generated.ResolveCategoryNVScopeRow {
	var leaves []fakeNV3
	for _, leaf := range f.nv3 {
		switch level {
		case LevelNV1:
			for _, parent := range f.nv2 {
				if parent.id == leaf.nv2ID && parent.nv1ID == refID {
					leaves = append(leaves, leaf)
				}
			}
		case LevelNV2:
			if leaf.nv2ID == refID {
				leaves = append(leaves, leaf)
			}
		case LevelNV3:
			if leaf.id == refID {
				leaves = append(leaves, leaf)
			}
		}
	}
	sort.Slice(leaves, func(i, j int) bool { return leaves[i].id < leaves[j].id })

	if len(leaves) == 0 {
		return []generated.ResolveCategoryNVScopeRow{{
			Level: string(level), RefID: refID, RefName: refName,
		}}
	}
	rows := make([]generated.ResolveCategoryNVScopeRow, 0, len(leaves))
	for _, leaf := range leaves {
		rows = append(rows, generated.ResolveCategoryNVScopeRow{
			Level:   string(level),
			RefID:   refID,
			RefName: refName,
			Nv3ID:   pgtype.Int4{Int32: leaf.id, Valid: true},
			Nv3Name: pgtype.Text{String: leaf.name, Valid: true},
		})
	}
	return rows
}

// legacySubtree junta el id pedido y todos sus descendientes legacy.
func (f *fakeScopeStore) legacySubtree(root int32) map[int32]bool {
	subtree := map[int32]bool{root: true}
	changed := true
	for changed {
		changed = false
		for _, node := range f.legacy {
			if node.parentID != nil && subtree[*node.parentID] && !subtree[node.id] {
				subtree[node.id] = true
				changed = true
			}
		}
	}
	return subtree
}

// matchesScope indica si el nodo coincide con el id o el slug consultados.
func matchesScope(params generated.ResolveCategoryNVScopeParams, id int32, slug string) bool {
	if params.ID.Valid && params.ID.Int32 == id {
		return true
	}
	return params.Slug.Valid && params.Slug.String == slug
}

// int32Ptr devuelve el puntero a un int32 del fixture.
func int32Ptr(value int32) *int32 {
	return &value
}

// TestResolveScope cubre slug de cada nivel, id legacy de raíz, hoja y fila
// con hijas, slug inexistente, NV3 sin puente y refs inválidos o ambiguos.
func TestResolveScope(t *testing.T) {
	store := newFakeScopeStore()

	tests := []struct {
		name    string
		ref     Ref
		want    Scope
		wantErr error
	}{
		{
			name: "slug de NV1 devuelve todas sus NV3",
			ref:  Ref{Slug: strPtr("hogar")},
			want: Scope{Level: LevelNV1, Name: "Hogar", NV3IDs: []int32{201, 202}},
		},
		{
			name: "slug de NV2 devuelve sus NV3",
			ref:  Ref{Slug: strPtr("electrodomesticos")},
			want: Scope{Level: LevelNV2, Name: "Electrodomésticos", NV3IDs: []int32{201, 202}},
		},
		{
			name: "slug de NV3 devuelve solo ella misma",
			ref:  Ref{Slug: strPtr("notebooks")},
			want: Scope{Level: LevelNV3, Name: "Notebooks", NV3IDs: []int32{203}},
		},
		{
			name: "id de NV1 devuelve sus NV3",
			ref:  Ref{ID: int32Ptr(2)},
			want: Scope{Level: LevelNV1, Name: "Tecnología", NV3IDs: []int32{203, 204}},
		},
		{
			name: "NV2 sin NV3 activas devuelve alcance vacío, no error",
			ref:  Ref{Slug: strPtr("pantallas")},
			want: Scope{Level: LevelNV2, Name: "Pantallas", NV3IDs: []int32{}},
		},
		{
			name: "NV3 sin puente sigue siendo su propio alcance",
			ref:  Ref{ID: int32Ptr(204)},
			want: Scope{Level: LevelNV3, Name: "Accesorios de computación", NV3IDs: []int32{204}},
		},
		{
			name: "id legacy de raíz con hijas",
			ref:  Ref{LegacyID: int32Ptr(10)},
			want: Scope{Level: LevelLegacy, Name: "Electrodomesticos", NV3IDs: []int32{201, 202}},
		},
		{
			name: "id legacy de hoja",
			ref:  Ref{LegacyID: int32Ptr(22)},
			want: Scope{Level: LevelLegacy, Name: "Notebooks", NV3IDs: []int32{203}},
		},
		{
			name: "id legacy con hija sin puente",
			ref:  Ref{LegacyID: int32Ptr(12)},
			want: Scope{Level: LevelLegacy, Name: "Cocina", NV3IDs: []int32{202}},
		},
		{
			name: "id legacy de NV2 cuyas NV3 incluyen una sin puente",
			ref:  Ref{LegacyID: int32Ptr(21)},
			want: Scope{Level: LevelLegacy, Name: "Computación", NV3IDs: []int32{203}},
		},
		{
			name:    "slug inexistente",
			ref:     Ref{Slug: strPtr("fantasma")},
			wantErr: ErrNotFound,
		},
		{
			name:    "id inexistente",
			ref:     Ref{ID: int32Ptr(99)},
			wantErr: ErrNotFound,
		},
		{
			name:    "id legacy inexistente",
			ref:     Ref{LegacyID: int32Ptr(99)},
			wantErr: ErrNotFound,
		},
		{
			name:    "slug de NV2 repetido en dos padres",
			ref:     Ref{Slug: strPtr("computacion")},
			wantErr: ErrAmbiguousScopeRef,
		},
		{
			name:    "ref sin identificador",
			ref:     Ref{},
			wantErr: ErrInvalidScopeRef,
		},
		{
			name:    "ref con dos identificadores",
			ref:     Ref{ID: int32Ptr(1), LegacyID: int32Ptr(10)},
			wantErr: ErrInvalidScopeRef,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ResolveScope(context.Background(), store, test.ref)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("error = %v, se esperaba %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("alcance = %+v, se esperaba %+v", got, test.want)
			}
		})
	}
}
