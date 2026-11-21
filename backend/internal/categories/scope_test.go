package categories

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
)

// fakeNode es una fila activa de product_category en el fake.
type fakeNode struct {
	id       int32
	parentID *int32
	name     string
	slug     string
	level    Level
}

// fakeScopeStore imita la consulta de CategoryScope sobre un árbol en memoria.
type fakeScopeStore struct {
	nodes []fakeNode
}

// newFakeScopeStore arma el árbol de prueba de tres niveles: Hogar >
// Electrodomésticos > Refrigeradores; Tecnología > Computación > Notebooks,
// Accesorios de computación; Tecnología > Pantallas sin hojas; y un nivel 2
// con un nivel 2 intermedio (Audio > Parlantes) para comprobar la recursión.
func newFakeScopeStore() *fakeScopeStore {
	parent := func(id int32) *int32 { return &id }
	return &fakeScopeStore{nodes: []fakeNode{
		{id: 1, name: "Hogar", slug: "hogar", level: Level1},
		{id: 2, name: "Tecnología", slug: "tecnologia", level: Level1},
		{id: 10, parentID: parent(1), name: "Electrodomésticos", slug: "electrodomesticos", level: Level2},
		{id: 11, parentID: parent(10), name: "Refrigeradores", slug: "refrigeradores", level: Level3},
		{id: 12, parentID: parent(10), name: "Cocina", slug: "cocina", level: Level3},
		{id: 20, parentID: parent(2), name: "Computación", slug: "computacion", level: Level2},
		{id: 21, parentID: parent(20), name: "Notebooks", slug: "notebooks", level: Level3},
		{id: 22, parentID: parent(20), name: "Accesorios de computación", slug: "accesorios-de-computacion", level: Level3},
		{id: 30, parentID: parent(2), name: "Pantallas", slug: "pantallas", level: Level2},
		{id: 40, parentID: parent(2), name: "Audio", slug: "audio", level: Level2},
		{id: 41, parentID: parent(40), name: "Parlantes", slug: "parlantes", level: Level2},
		{id: 42, parentID: parent(41), name: "Parlantes portátiles", slug: "parlantes-portatiles", level: Level3},
	}}
}

// CategoryScope imita la consulta real: busca el nodo activo por id o slug,
// recorre su subárbol y devuelve los ids de las hojas (nivel 3) ordenados.
func (f *fakeScopeStore) CategoryScope(_ context.Context, ref Ref) (Scope, error) {
	var root *fakeNode
	for i := range f.nodes {
		node := &f.nodes[i]
		if (ref.ID != nil && node.id == *ref.ID) || (ref.Slug != nil && node.slug == *ref.Slug) {
			root = node
			break
		}
	}
	if root == nil {
		return Scope{}, ErrNotFound
	}

	inSubtree := map[int32]bool{root.id: true}
	changed := true
	for changed {
		changed = false
		for _, node := range f.nodes {
			if node.parentID != nil && inSubtree[*node.parentID] && !inSubtree[node.id] {
				inSubtree[node.id] = true
				changed = true
			}
		}
	}

	ids := []int32{}
	for _, node := range f.nodes {
		if node.level == Level3 && inSubtree[node.id] {
			ids = append(ids, node.id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return Scope{Level: root.level, Name: root.name, CategoryIDs: ids}, nil
}

// int32Ptr devuelve el puntero a un int32 del fixture.
func int32Ptr(value int32) *int32 {
	return &value
}

// TestResolveScope cubre slug e id de cada nivel, hojas sin productos, un
// nivel 2 intermedio, categorías inexistentes y refs inválidos.
func TestResolveScope(t *testing.T) {
	store := newFakeScopeStore()

	tests := []struct {
		name    string
		ref     Ref
		want    Scope
		wantErr error
	}{
		{
			name: "slug de nivel 1 devuelve todas sus hojas",
			ref:  Ref{Slug: strPtr("hogar")},
			want: Scope{Level: Level1, Name: "Hogar", CategoryIDs: []int32{11, 12}},
		},
		{
			name: "slug de nivel 2 devuelve sus hojas",
			ref:  Ref{Slug: strPtr("electrodomesticos")},
			want: Scope{Level: Level2, Name: "Electrodomésticos", CategoryIDs: []int32{11, 12}},
		},
		{
			name: "slug de nivel 3 devuelve solo ella misma",
			ref:  Ref{Slug: strPtr("notebooks")},
			want: Scope{Level: Level3, Name: "Notebooks", CategoryIDs: []int32{21}},
		},
		{
			name: "id de nivel 1 devuelve sus hojas",
			ref:  Ref{ID: int32Ptr(2)},
			want: Scope{Level: Level1, Name: "Tecnología", CategoryIDs: []int32{21, 22, 42}},
		},
		{
			name: "id de nivel 2 intermedio baja hasta las hojas",
			ref:  Ref{ID: int32Ptr(41)},
			want: Scope{Level: Level2, Name: "Parlantes", CategoryIDs: []int32{42}},
		},
		{
			name: "nivel 2 sin hojas devuelve alcance vacío, no error",
			ref:  Ref{Slug: strPtr("pantallas")},
			want: Scope{Level: Level2, Name: "Pantallas", CategoryIDs: []int32{}},
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
			name:    "ref sin identificador",
			ref:     Ref{},
			wantErr: ErrInvalidScopeRef,
		},
		{
			name:    "ref con dos identificadores",
			ref:     Ref{ID: int32Ptr(1), Slug: strPtr("hogar")},
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
