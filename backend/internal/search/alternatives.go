package search

// Respuestas alternativas sin IA (TAL-226).
//
// La idea es simple: emparejar el texto del usuario con el catálogo de presets
// (TAL-222) de forma determinista. Si hay coincidencia, se responde con esa
// consulta curada; si no, se devuelve la búsqueda libre y hasta tres presets
// cercanos como sugerencia.
//
// No se genera texto. El campo `source` dice por qué el cliente recibió lo que
// recibió, y todo lo demás son datos de la base. Sin prosa generada no hay nada
// que alucinar: es la misma línea de TAL-221.

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/DoSafills/SOLOServis/backend/internal/categories"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
)

// Umbral de coincidencia: proporción de tokens relevantes de la consulta que
// aparecen en el vocabulario del preset. Por debajo, se prefiere la búsqueda
// libre antes que adivinar.
const alternativeMatchThreshold = 0.5

// alternativeSuggestions es cuántos presets se ofrecen cuando no hay match.
const alternativeSuggestions = 3

// AlternativePreset es la vista pública y mínima de un preset.
type AlternativePreset struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// AlternativesResponse es el cuerpo de GET /search/alternatives.
type AlternativesResponse struct {
	Query    string              `json:"query"`
	Source   string              `json:"source"` // "preset" | "text"
	Matched  *AlternativePreset  `json:"matched"`
	Presets  []AlternativePreset `json:"presets"`
	Products []ProductResult     `json:"products"`
}

// stopwords son palabras que no aportan intención. Si todas las palabras de la
// consulta caen aquí, no se intenta adivinar nada.
var stopwords = map[string]bool{
	"a": true, "al": true, "algo": true, "algun": true, "alguna": true,
	"ante": true, "buscar": true, "busco": true, "como": true, "con": true,
	"cual": true, "cuando": true, "de": true, "del": true, "donde": true,
	"el": true, "en": true, "entre": true, "es": true, "esa": true, "ese": true,
	"esta": true, "este": true, "hay": true, "la": true, "las": true,
	"lo": true, "los": true, "mas": true, "me": true, "mejor": true,
	"mi": true, "necesito": true, "o": true, "para": true, "pero": true,
	"por": true, "que": true, "quiero": true, "se": true, "sin": true,
	"sobre": true, "su": true, "sus": true, "un": true, "una": true,
	"uno": true, "y": true, "ya": true,
}

// sinAcentos reduce las vocales acentuadas y la ñ para comparar sin depender de
// x/text. Cubre el español que aparece en este catálogo.
func sinAcentos(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
		"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U",
		"ñ", "n", "Ñ", "N",
	)
	return replacer.Replace(s)
}

// singular recorta el plural español de forma conservadora: "notebooks" →
// "notebook", "televisores" → "televisor". No es un stemmer, y no pretende serlo.
func singular(token string) string {
	if len(token) > 5 && strings.HasSuffix(token, "es") {
		return strings.TrimSuffix(token, "es")
	}
	if len(token) > 4 && strings.HasSuffix(token, "s") {
		return strings.TrimSuffix(token, "s")
	}
	return token
}

// tokens normaliza un texto: minúsculas, sin acentos, sólo letras y dígitos,
// sin stopwords y en singular.
func tokens(text string) []string {
	lowered := strings.ToLower(sinAcentos(text))
	fields := strings.FieldsFunc(lowered, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) < 2 || stopwords[field] {
			continue
		}
		out = append(out, singular(field))
	}
	return out
}

// presetVocabulary junta slug, título y aliases en un conjunto de tokens.
func presetVocabulary(preset Preset) map[string]bool {
	parts := []string{strings.ReplaceAll(preset.Slug, "-", " "), preset.Title}
	parts = append(parts, preset.Aliases...)
	vocabulary := make(map[string]bool)
	for _, part := range parts {
		for _, token := range tokens(part) {
			vocabulary[token] = true
		}
	}
	return vocabulary
}

type presetScore struct {
	preset Preset
	score  float64
	order  int
}

// rankPresets ordena los presets por coincidencia con la consulta. Devuelve
// también el mejor, aunque no alcance el umbral, para poder sugerirlo.
func rankPresets(text string) []presetScore {
	queryTokens := tokens(text)
	if len(queryTokens) == 0 {
		return nil
	}
	ranked := make([]presetScore, 0, len(Presets))
	for index, preset := range Presets {
		vocabulary := presetVocabulary(preset)
		matched := 0
		for _, token := range queryTokens {
			if vocabulary[token] {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		ranked = append(ranked, presetScore{
			preset: preset,
			score:  float64(matched) / float64(len(queryTokens)),
			order:  index,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].order < ranked[j].order
	})
	return ranked
}

func publicPreset(preset Preset) AlternativePreset {
	return AlternativePreset{
		Slug:        preset.Slug,
		Title:       preset.Title,
		Description: preset.Description,
	}
}

// Alternatives atiende GET /search/alternatives?q=&limit=&offset=.
//
//	Con coincidencia: source="preset", matched=<preset>, products=ese preset.
//	Sin coincidencia: source="text", matched=null, products=búsqueda libre y
//	                  hasta tres presets cercanos como sugerencia.
func (h *Handler) Alternatives(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	text := strings.TrimSpace(query.Get("q"))

	var v httpx.Validator
	v.Required("q", text)
	v.MaxLength("q", text, MaxQueryLength)
	words := strings.Fields(text)
	v.Check(len(words) <= MaxWords, "q", "max", "q admite hasta 5 palabras")
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}

	q := Query{Words: words, Sort: "name", Limit: DefaultLimit}
	var page Query
	parsePage(&v, query, &page)
	if !v.Valid() {
		writeInvalidQuery(w, r, v)
		return
	}
	if page.Limit > 0 {
		q.Limit = page.Limit
	}
	q.Offset = page.Offset

	ranked := rankPresets(text)
	response := AlternativesResponse{Query: text, Presets: []AlternativePreset{}}

	if len(ranked) > 0 && ranked[0].score >= alternativeMatchThreshold {
		matched := ranked[0].preset
		presetQuery := matched.toQuery()
		// La paginación del cliente manda; si no la envía, vale la del preset
		// (novedades y precios-bajos piden 12, no 20).
		if query.Has("limit") {
			presetQuery.Limit = q.Limit
		}
		presetQuery.Offset = q.Offset
		if matched.Query.Category != "" {
			slug := matched.Query.Category
			if scope, err := h.searcher.ResolveScope(r.Context(), categories.Ref{Slug: &slug}); err == nil {
				presetQuery.CategoryIDs = scope.CategoryIDs
			}
		}
		q = presetQuery

		public := publicPreset(matched)
		response.Source = "preset"
		response.Matched = &public
	} else {
		response.Source = "text"
	}

	for _, candidate := range ranked {
		if response.Matched != nil && candidate.preset.Slug == response.Matched.Slug {
			continue
		}
		if len(response.Presets) == alternativeSuggestions {
			break
		}
		response.Presets = append(response.Presets, publicPreset(candidate.preset))
	}

	results, total, err := h.searcher.SearchProducts(r.Context(), q)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if results == nil {
		results = []ProductResult{}
	}
	response.Products = results

	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	writeJSON(w, response)
}
