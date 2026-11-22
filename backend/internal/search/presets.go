package search

// Consultas predefinidas (TAL-222). Son búsquedas curadas que el frontend puede
// ofrecer sin que el usuario escriba nada y que el fallback sin IA de TAL-226
// reutiliza para responder.
//
// El catálogo vive en el código, no en una tabla:
//   - cambiarlo es un PR revisado, con diff legible y rollback con el código;
//   - no necesita migración, CRUD ni permisos;
//   - se valida en CI, así que un typo rompe un test y no producción.
//
// Los presets sólo usan datos portables entre entornos: slug de categoría,
// texto, precio y orden. Nunca storeId ni brandId, porque los IDs cambian según
// el entorno (ver AGENTS.md §8).

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// PresetQuery son los filtros de un preset, en la misma forma en que viajan por
// la API. Los campos vacíos se omiten en el JSON.
type PresetQuery struct {
	Q         string `json:"q,omitempty"`
	Category  string `json:"category,omitempty"`
	MinPrice  string `json:"minPrice,omitempty"`
	MaxPrice  string `json:"maxPrice,omitempty"`
	Available *bool  `json:"available,omitempty"`
	Sort      string `json:"sort,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// Preset es una consulta curada. Aliases alimenta el emparejamiento
// determinista de TAL-226 y no se expone por la API.
type Preset struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Query       PresetQuery `json:"query"`
	Aliases     []string    `json:"-"`
}

// PresetList es el cuerpo de GET /search/presets.
type PresetList struct {
	Presets []Preset `json:"presets"`
}

// Presets es el catálogo. El orden es el de presentación: no se ordena
// alfabéticamente para que el criterio editorial quede explícito.
//
// Las categorías son las sembradas en 013_categories_seed.sql. Si se agrega
// una, se actualiza el test del catálogo además de este archivo.
var Presets = []Preset{
	{
		Slug:        "refrigeradores",
		Title:       "Refrigeradores",
		Description: "Refrigeradores y equipos de frío de todas las tiendas, del más barato al más caro.",
		Query:       PresetQuery{Category: "refrigeradores", Sort: "price_asc", Limit: DefaultLimit},
		Aliases:     []string{"refrigerador", "nevera", "heladera", "frigorifico", "frio"},
	},
	{
		Slug:        "refrigeradores-economicos",
		Title:       "Refrigeradores económicos",
		Description: "Refrigeradores de hasta $400.000, ordenados por precio.",
		Query:       PresetQuery{Category: "refrigeradores", MaxPrice: "400000", Sort: "price_asc", Limit: DefaultLimit},
		Aliases:     []string{"refrigerador barato", "nevera economica", "refrigerador pequeño"},
	},
	{
		Slug:        "notebooks",
		Title:       "Notebooks",
		Description: "Notebooks y computadores portátiles, del más barato al más caro.",
		Query:       PresetQuery{Category: "notebooks", Sort: "price_asc", Limit: DefaultLimit},
		Aliases:     []string{"notebook", "laptop", "portatil", "computador portatil"},
	},
	{
		Slug:        "notebooks-economicos",
		Title:       "Notebooks económicos",
		Description: "Notebooks de hasta $550.000, ordenados por precio.",
		Query:       PresetQuery{Category: "notebooks", MaxPrice: "550000", Sort: "price_asc", Limit: DefaultLimit},
		Aliases:     []string{"notebook barato", "laptop economica", "notebook estudiante"},
	},
	{
		Slug:        "televisores",
		Title:       "Televisores",
		Description: "Televisores y pantallas disponibles en las tiendas.",
		Query:       PresetQuery{Category: "televisores", Sort: "price_asc", Limit: DefaultLimit},
		Aliases:     []string{"televisor", "tv", "pantalla", "smart tv"},
	},
	{
		Slug:        "precios-bajos",
		Title:       "Precios bajos",
		Description: "Productos de hasta $100.000, del más barato al más caro.",
		Query:       PresetQuery{MaxPrice: "100000", Sort: "price_asc", Limit: 12},
		Aliases:     []string{"barato", "economico", "bajo presupuesto", "oferta", "ofertas"},
	},
	{
		Slug:        "novedades",
		Title:       "Novedades",
		Description: "Lo último que ingresó al catálogo.",
		Query:       PresetQuery{Sort: "newest", Limit: 12},
		Aliases:     []string{"novedad", "nuevo", "reciente", "ultimos ingresos"},
	},
}

// presetsBySlug permite resolver sin recorrer el arreglo en cada request.
var presetsBySlug = func() map[string]Preset {
	index := make(map[string]Preset, len(Presets))
	for _, preset := range Presets {
		index[preset.Slug] = preset
	}
	return index
}()

// LookupPreset devuelve el preset con ese slug.
func LookupPreset(slug string) (Preset, bool) {
	preset, ok := presetsBySlug[slug]
	return preset, ok
}

// toQuery convierte el preset en la consulta que consume el repositorio.
func (p Preset) toQuery() Query {
	q := Query{Sort: "name", Limit: DefaultLimit}
	q.Words = strings.Fields(p.Query.Q)
	if p.Query.MinPrice != "" {
		minPrice := p.Query.MinPrice
		q.MinPrice = &minPrice
	}
	if p.Query.MaxPrice != "" {
		maxPrice := p.Query.MaxPrice
		q.MaxPrice = &maxPrice
	}
	if p.Query.Available != nil {
		q.Available = *p.Query.Available
	}
	if p.Query.Sort != "" {
		q.Sort = p.Query.Sort
	}
	if p.Query.Limit > 0 {
		q.Limit = p.Query.Limit
	}
	return q
}

// validatePresets revisa el catálogo completo. Se llama desde los tests: un
// preset inválido rompe CI en vez de responder 500 en producción.
func validatePresets(presets []Preset) error {
	seen := make(map[string]bool, len(presets))
	for _, preset := range presets {
		if !categories.ValidSlug(preset.Slug) {
			return fmt.Errorf("preset con slug inválido: %q", preset.Slug)
		}
		if seen[preset.Slug] {
			return fmt.Errorf("preset duplicado: %q", preset.Slug)
		}
		seen[preset.Slug] = true

		if strings.TrimSpace(preset.Title) == "" {
			return fmt.Errorf("preset %q sin título", preset.Slug)
		}
		if strings.TrimSpace(preset.Description) == "" {
			return fmt.Errorf("preset %q sin descripción", preset.Slug)
		}
		if preset.Query.Category != "" && !categories.ValidSlug(preset.Query.Category) {
			return fmt.Errorf("preset %q con categoría inválida: %q", preset.Slug, preset.Query.Category)
		}
		if preset.Query.Sort != "" {
			if _, ok := orderBySQL[preset.Query.Sort]; !ok {
				return fmt.Errorf("preset %q con sort inválido: %q", preset.Slug, preset.Query.Sort)
			}
		}
		if len(strings.Fields(preset.Query.Q)) > MaxWords {
			return fmt.Errorf("preset %q supera %d palabras en q", preset.Slug, MaxWords)
		}
		if preset.Query.Limit < 0 || preset.Query.Limit > MaxLimit {
			return fmt.Errorf("preset %q con limit fuera de rango: %d", preset.Slug, preset.Query.Limit)
		}
		for _, price := range []struct {
			field string
			value string
		}{
			{"minPrice", preset.Query.MinPrice},
			{"maxPrice", preset.Query.MaxPrice},
		} {
			if price.value != "" && !moneyPattern.MatchString(price.value) {
				return fmt.Errorf("preset %q con %s inválido: %q", preset.Slug, price.field, price.value)
			}
		}
		if preset.Query.MinPrice != "" && preset.Query.MaxPrice != "" {
			min, _ := strconv.ParseFloat(preset.Query.MinPrice, 64)
			max, _ := strconv.ParseFloat(preset.Query.MaxPrice, 64)
			if min > max {
				return fmt.Errorf("preset %q con minPrice > maxPrice", preset.Slug)
			}
		}
	}
	return nil
}

// Presets atiende GET /search/presets: el catálogo completo, sin ejecutar nada.
func (h *Handler) Presets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, PresetList{Presets: Presets})
}

// PresetProducts atiende GET /search/presets/{slug}: ejecuta el preset y
// responde igual que /search/products, con X-Total-Count.
func (h *Handler) PresetProducts(w http.ResponseWriter, r *http.Request) {
	preset, ok := LookupPreset(chi.URLParam(r, "slug"))
	if !ok {
		httpx.WriteProblem(w, r, http.StatusNotFound, "preset_not_found",
			"La consulta predefinida no existe.")
		return
	}

	q := preset.toQuery()
	var v httpx.Validator
	parsePage(&v, r.URL.Query(), &q)
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}
	if preset.Query.Category != "" {
		slug := preset.Query.Category
		scope, err := h.searcher.ResolveScope(r.Context(), categories.Ref{Slug: &slug})
		switch {
		case errors.Is(err, categories.ErrNotFound):
			httpx.WriteProblem(w, r, http.StatusBadRequest, httpx.CodeInvalidParameter,
				"Los parámetros de la consulta no son válidos.",
				httpx.FieldError{Field: "category", Code: "not_found", Message: "category no existe"})
			return
		case err != nil:
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}
		q.CategoryIDs = scope.CategoryIDs
	}

	results, total, err := h.searcher.SearchProducts(r.Context(), q)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if results == nil {
		results = []ProductResult{}
	}
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, results)
}
