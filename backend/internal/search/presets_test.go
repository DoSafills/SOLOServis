package search

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// TAL-222: el catálogo de consultas predefinidas. Estos tests fijan tres
// cosas: el catálogo es válido y portable, la ejecución reutiliza la búsqueda,
// y un slug desconocido responde 404 sin tocar la base.

func TestPresets_CatalogoValido(t *testing.T) {
	if err := validatePresets(Presets); err != nil {
		t.Fatalf("catálogo inválido: %v", err)
	}
	if len(Presets) < 6 {
		t.Fatalf("se esperaban al menos 6 presets, hay %d", len(Presets))
	}
}

// Las categorías de los presets deben existir en las semillas
// (013_categories_seed.sql). Si se agrega una categoría con datos reales, se
// agrega aquí y al preset.
func TestPresets_CategoriasSembradas(t *testing.T) {
	sembradas := map[string]bool{
		"refrigeradores": true,
		"notebooks":      true,
		"televisores":    true,
	}
	for _, preset := range Presets {
		if preset.Query.Category == "" {
			continue
		}
		if !sembradas[preset.Query.Category] {
			t.Errorf("preset %q apunta a la categoría %q, que no está sembrada",
				preset.Slug, preset.Query.Category)
		}
	}
}

func TestPresets_Respuesta(t *testing.T) {
	rec, _ := serve(t, &fakeSearcher{}, "/search/presets")

	requireStatus(t, rec, http.StatusOK)
	var body PresetList
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Presets) != len(Presets) {
		t.Fatalf("presets = %d, quiero %d", len(body.Presets), len(Presets))
	}
	for _, preset := range body.Presets {
		if preset.Slug == "" || preset.Title == "" || preset.Description == "" {
			t.Fatalf("preset incompleto: %+v", preset)
		}
	}
	if body.Presets[0].Slug != "refrigeradores" {
		t.Fatalf("el orden editorial no se respetó: primero %q", body.Presets[0].Slug)
	}
}

func TestPresetProducts_EjecutaElPreset(t *testing.T) {
	s := &fakeSearcher{results: []ProductResult{{ID: "x"}}, total: 3,
		scope: categories.Scope{CategoryIDs: []int32{4}}}

	rec, _ := serve(t, s, "/search/presets/notebooks-economicos?limit=5")

	requireStatus(t, rec, http.StatusOK)
	if rec.Header().Get("X-Total-Count") != "3" {
		t.Fatalf("X-Total-Count = %q", rec.Header().Get("X-Total-Count"))
	}
	if !strings.HasPrefix(rec.Body.String(), "[") {
		t.Fatalf("cuerpo = %s, se esperaba un arreglo", rec.Body.String())
	}

	q := *s.gotQuery
	if s.gotScopeRef == nil || s.gotScopeRef.Slug == nil || *s.gotScopeRef.Slug != "notebooks" {
		t.Fatalf("category = %+v", s.gotScopeRef)
	}
	if len(q.CategoryIDs) != 1 || q.CategoryIDs[0] != 4 {
		t.Fatalf("categorías = %v", q.CategoryIDs)
	}
	if q.MaxPrice == nil || *q.MaxPrice != "550000" || q.MinPrice != nil {
		t.Fatalf("precios = min %v max %v", q.MinPrice, q.MaxPrice)
	}
	if q.Sort != "price_asc" || q.Limit != 5 || q.Offset != 0 {
		t.Fatalf("sort/limit/offset = %q %d %d", q.Sort, q.Limit, q.Offset)
	}
}

func TestPresetProducts_UsaSuPropioLimite(t *testing.T) {
	s := &fakeSearcher{}

	rec, _ := serve(t, s, "/search/presets/novedades")

	requireStatus(t, rec, http.StatusOK)
	q := *s.gotQuery
	if q.Sort != "newest" || q.Limit != 12 || q.CategoryIDs != nil || s.gotScopeRef != nil {
		t.Fatalf("query de novedades = %+v (alcance %+v)", q, s.gotScopeRef)
	}
}

func TestPresetProducts_PaginacionInvalida(t *testing.T) {
	s := &fakeSearcher{}

	rec, _ := serve(t, s, "/search/presets/notebooks?limit=0")

	requireStatus(t, rec, http.StatusBadRequest)
	if codes := fieldCodes(t, rec); len(codes) != 1 || codes[0] != "limit/min" {
		t.Fatalf("errores = %v", codes)
	}
	if s.dbTouched != 0 {
		t.Fatal("una paginación inválida no debe llegar a la base")
	}
}

func TestPresetProducts_SlugDesconocido(t *testing.T) {
	s := &fakeSearcher{}

	rec, _ := serve(t, s, "/search/presets/no-existe")

	requireStatus(t, rec, http.StatusNotFound)
	var problem httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "preset_not_found" {
		t.Fatalf("code = %q", problem.Code)
	}
	if s.dbTouched != 0 {
		t.Fatal("un preset desconocido no debe consultar la base")
	}
}

// TestPresetProducts_AlcanceInvalido cubre que un preset con categoría que no
// resuelve en el árbol responde 400 con el código del campo, no 500.
func TestPresetProducts_AlcanceInvalido(t *testing.T) {
	rec, _ := serve(t, &fakeSearcher{scopeErr: categories.ErrNotFound}, "/search/presets/notebooks")
	requireStatus(t, rec, http.StatusBadRequest)
	if codes := fieldCodes(t, rec); len(codes) != 1 || codes[0] != "category/not_found" {
		t.Fatalf("errores = %v", codes)
	}
}

func TestPresetProducts_ErrorInternoNoFiltraDetalles(t *testing.T) {
	s := &fakeSearcher{err: errBoom}

	rec, logs := serve(t, s, "/search/presets/refrigeradores")

	requireStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "relation") {
		t.Fatalf("la respuesta filtró el error interno: %s", rec.Body.String())
	}
	if logs == "" {
		t.Fatal("el error debe quedar en el log")
	}
}

func TestValidatePresets_RechazaDefinicionesInvalidas(t *testing.T) {
	base := Preset{
		Slug:        "ok",
		Title:       "Título",
		Description: "Descripción",
		Query:       PresetQuery{Sort: "name"},
	}
	casos := []struct {
		nombre string
		preset Preset
	}{
		{"slug vacío", Preset{Slug: "", Title: "T", Description: "D"}},
		{"slug con mayúsculas", Preset{Slug: "Notebooks", Title: "T", Description: "D"}},
		{"sin título", Preset{Slug: "x", Title: " ", Description: "D"}},
		{"sin descripción", Preset{Slug: "x", Title: "T", Description: ""}},
		{"sort inválido", func() Preset { p := base; p.Query.Sort = "caro"; return p }()},
		{"precio inválido", func() Preset { p := base; p.Query.MaxPrice = "100.000"; return p }()},
		{"min mayor que max", func() Preset { p := base; p.Query.MinPrice = "900"; p.Query.MaxPrice = "100"; return p }()},
		{"limit fuera de rango", func() Preset { p := base; p.Query.Limit = MaxLimit + 1; return p }()},
		{"demasiadas palabras", func() Preset { p := base; p.Query.Q = "uno dos tres cuatro cinco seis"; return p }()},
		{"categoría inválida", func() Preset { p := base; p.Query.Category = "Refrigeradores"; return p }()},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if err := validatePresets([]Preset{caso.preset}); err == nil {
				t.Fatalf("se esperaba error para %q", caso.nombre)
			}
		})
	}

	duplicado := []Preset{base, base}
	if err := validatePresets(duplicado); err == nil {
		t.Fatal("un slug duplicado debía rechazarse")
	}
}
