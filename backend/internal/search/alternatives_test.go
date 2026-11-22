package search

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TAL-226: respuestas alternativas sin IA. El emparejamiento es determinista y
// no genera texto: `source` explica qué pasó y `products` sale siempre de la
// base. Estos tests fijan el umbral, la normalización y los casos límite.

func decodeAlternatives(t *testing.T, body string) AlternativesResponse {
	t.Helper()
	var response AlternativesResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("respuesta ilegible: %v (%s)", err, body)
	}
	return response
}

func TestTokens_NormalizaAcentosPluralesYStopwords(t *testing.T) {
	casos := []struct {
		texto  string
		quiero []string
	}{
		{"Notebooks", []string{"notebook"}},
		{"neveras", []string{"nevera"}},
		{"televisores", []string{"televisor"}},
		{"refrigeradores económicos", []string{"refrigerador", "economico"}},
		{"quiero algo para estudiar", []string{"estudiar"}},
		{"TV 4K", []string{"tv", "4k"}},
	}
	for _, caso := range casos {
		if got := tokens(caso.texto); !reflect.DeepEqual(got, caso.quiero) {
			t.Errorf("tokens(%q) = %v, quiero %v", caso.texto, got, caso.quiero)
		}
	}
}

func TestAlternatives_EmparejaElPresetMasEspecifico(t *testing.T) {
	s := &fakeSearcher{results: []ProductResult{{ID: "x"}}, total: 7}

	rec, _ := serve(t, s, "/search/alternatives?q=refrigerador%20barato")

	requireStatus(t, rec, http.StatusOK)
	response := decodeAlternatives(t, rec.Body.String())
	if response.Source != "preset" {
		t.Fatalf("source = %q", response.Source)
	}
	if response.Matched == nil || response.Matched.Slug != "refrigeradores-economicos" {
		t.Fatalf("matched = %+v", response.Matched)
	}
	q := *s.gotQuery
	if s.gotScopeRef == nil || s.gotScopeRef.Slug == nil || *s.gotScopeRef.Slug != "refrigeradores" ||
		q.MaxPrice == nil || *q.MaxPrice != "400000" {
		t.Fatalf("query = %+v (alcance %+v)", q, s.gotScopeRef)
	}
	if rec.Header().Get("X-Total-Count") != "7" {
		t.Fatalf("X-Total-Count = %q", rec.Header().Get("X-Total-Count"))
	}
}

func TestAlternatives_AceptaAcentoYPlural(t *testing.T) {
	s := &fakeSearcher{}

	rec, _ := serve(t, s, "/search/alternatives?q=neveras")

	requireStatus(t, rec, http.StatusOK)
	response := decodeAlternatives(t, rec.Body.String())
	if response.Matched == nil || response.Matched.Slug != "refrigeradores" {
		t.Fatalf("matched = %+v", response.Matched)
	}
}

func TestAlternatives_SinCoincidenciaUsaBusquedaLibre(t *testing.T) {
	s := &fakeSearcher{}

	rec, _ := serve(t, s, "/search/alternatives?q=taladro%20percutor")

	requireStatus(t, rec, http.StatusOK)
	response := decodeAlternatives(t, rec.Body.String())
	if response.Source != "text" || response.Matched != nil {
		t.Fatalf("source=%q matched=%+v", response.Source, response.Matched)
	}
	if !reflect.DeepEqual(s.gotQuery.Words, []string{"taladro", "percutor"}) {
		t.Fatalf("words = %v", s.gotQuery.Words)
	}
	if response.Presets == nil || len(response.Presets) != 0 {
		t.Fatalf("presets = %v, se esperaba arreglo vacío", response.Presets)
	}
	if strings.Contains(rec.Body.String(), `"presets":null`) {
		t.Fatal("presets no debe ser null")
	}
}

func TestAlternatives_SugierePresetsDebajoDelUmbral(t *testing.T) {
	s := &fakeSearcher{}

	// "notebook" y "pantalla" apuntan a presets distintos; con "4k" cada uno
	// acierta 1 de 3 y ninguno alcanza el umbral.
	rec, _ := serve(t, s, "/search/alternatives?q=notebook%20con%20pantalla%204k")

	requireStatus(t, rec, http.StatusOK)
	response := decodeAlternatives(t, rec.Body.String())
	if response.Source != "text" || response.Matched != nil {
		t.Fatalf("source=%q matched=%+v", response.Source, response.Matched)
	}
	if len(response.Presets) != 3 {
		t.Fatalf("sugerencias = %+v", response.Presets)
	}
	if response.Presets[0].Slug != "notebooks" ||
		response.Presets[1].Slug != "notebooks-economicos" ||
		response.Presets[2].Slug != "televisores" {
		t.Fatalf("orden de sugerencias = %+v", response.Presets)
	}
}

func TestAlternatives_RespetaLosLimites(t *testing.T) {
	t.Run("el cliente manda", func(t *testing.T) {
		s := &fakeSearcher{}
		rec, _ := serve(t, s, "/search/alternatives?q=notebooks&limit=5")
		requireStatus(t, rec, http.StatusOK)
		if s.gotQuery.Limit != 5 {
			t.Fatalf("limit = %d", s.gotQuery.Limit)
		}
	})

	t.Run("sin limit vale el del preset", func(t *testing.T) {
		s := &fakeSearcher{}
		rec, _ := serve(t, s, "/search/alternatives?q=novedades")
		requireStatus(t, rec, http.StatusOK)
		if s.gotQuery.Limit != 12 || s.gotQuery.Sort != "newest" {
			t.Fatalf("query = %+v", s.gotQuery)
		}
	})
}

func TestAlternatives_ValidaLaConsulta(t *testing.T) {
	casos := []struct {
		nombre string
		target string
		campo  string
	}{
		{"sin q", "/search/alternatives", "q/required"},
		{"demasiadas palabras", "/search/alternatives?q=uno%20dos%20tres%20cuatro%20cinco%20seis", "q/max"},
		{"limit inválido", "/search/alternatives?q=notebooks&limit=0", "limit/min"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			s := &fakeSearcher{}
			rec, _ := serve(t, s, caso.target)

			requireStatus(t, rec, http.StatusBadRequest)
			codes := fieldCodes(t, rec)
			if len(codes) != 1 || codes[0] != caso.campo {
				t.Fatalf("errores = %v", codes)
			}
			if s.dbTouched != 0 {
				t.Fatal("una consulta inválida no debe tocar la base")
			}
		})
	}
}

func TestAlternatives_ErrorInternoNoFiltraDetalles(t *testing.T) {
	s := &fakeSearcher{err: errBoom}

	rec, logs := serve(t, s, "/search/alternatives?q=notebooks")

	requireStatus(t, rec, http.StatusInternalServerError)
	if strings.Contains(rec.Body.String(), "relation") {
		t.Fatalf("la respuesta filtró el error interno: %s", rec.Body.String())
	}
	if logs == "" {
		t.Fatal("el error debe quedar en el log")
	}
}

func TestAlternatives_SinResultadosDevuelveArreglos(t *testing.T) {
	s := &fakeSearcher{results: nil, total: 0}

	rec, _ := serve(t, s, "/search/alternatives?q=notebooks")

	requireStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"products":[]`) {
		t.Fatalf("products debe ser arreglo vacío: %s", rec.Body.String())
	}
}
