package ingestion

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Counters son los números que el scraper reporta al cerrar una corrida.
// No incluyen los ítems procesados por ingestion: esos ya viven en
// scrape_run (records_found/processed/failed).
type Counters struct {
	// Found es cuántas tarjetas vio el scraper en el listado.
	Found int
	// Sent es cuántos ítems llegaron a enviarse a ingestion.
	Sent int
	// Rejected es cuántas tarjetas descartó el motor por no cumplir el
	// contrato (atributo obligatorio, URL, precio).
	Rejected int
	// Duplicated es cuántas tarjetas se descartaron por repetir un producto
	// ya visto en la misma corrida. No son un fallo: el listado muestra el
	// mismo producto dos veces, pero sin este número la caída de `Sent`
	// parecería una tienda rota (TAL-172).
	Duplicated int
	// PagesFailed es cuántas páginas del listado no se pudieron leer.
	PagesFailed int
	// ZeroPrices es cuántos ítems del lote llegaron con precio nulo o en 0.
	ZeroPrices int
	// PricesTotal es cuántos ítems se recibieron en los lotes de la corrida;
	// es el denominador de la proporción de precios inválidos.
	PricesTotal int
}

// Thresholds son los límites con los que se decide que una corrida quedó
// degradada. Se leen del entorno para poder ajustarlos sin recompilar.
type Thresholds struct {
	// RejectRatio es la proporción de rechazos sobre encontrados que ya
	// se considera anormal (0.30 por defecto).
	RejectRatio float64
	// DropRatio es la caída, respecto de la mediana histórica, a partir de
	// la cual la corrida se marca (0.50 por defecto).
	DropRatio float64
	// BaselineRuns es cuántas corridas exitosas previas forman la mediana.
	BaselineRuns int
	// PriceRatio es la proporción de ítems con precio nulo o en 0 sobre los
	// recibidos que ya se considera anormal (0.20 por defecto).
	PriceRatio float64
	// RootDropRatio es la caída de tarjetas vistas (rootMatches) contra la
	// mediana histórica que marca un cambio en la fuente (0.50 por defecto).
	RootDropRatio float64
	// SelectorMinRatio es la proporción mínima de tarjetas en que debe
	// coincidir un selector required/rejectIfMissing (0.10 por defecto).
	SelectorMinRatio float64
	// HTTPFailRatio es la proporción de páginas con HTTP >= 400 o contenido no
	// HTML que marca un cambio de endpoint (0.20 por defecto).
	HTTPFailRatio float64
	// ResponseMissingRatio es la proporción de claves JSON-LD esperadas que
	// puede faltar antes de marcar un cambio de respuesta (0.50 por defecto).
	ResponseMissingRatio float64
	// MinCards es el mínimo de tarjetas de la mediana para que valga la pena
	// comparar: con tiendas pequeñas el ruido supera la señal (10 por defecto).
	MinCards int
	// MinPages es el mínimo de páginas leídas para evaluar el endpoint (5).
	MinPages int
}

// ThresholdsFromEnv arma los umbrales con los valores por defecto y los
// sobrescribe con INGESTION_DEGRADED_REJECT_RATIO, INGESTION_DEGRADED_DROP_RATIO,
// INGESTION_DEGRADED_PRICE_RATIO e INGESTION_DEGRADED_BASELINE_RUNS. Un valor
// inválido o fuera de rango se ignora en silencio: una variable mal escrita no
// puede apagar las alertas.
func ThresholdsFromEnv() Thresholds {
	thresholds := Thresholds{
		RejectRatio:          DefaultRejectRatio,
		DropRatio:            DefaultDropRatio,
		BaselineRuns:         DefaultBaselineRuns,
		PriceRatio:           DefaultPriceRatio,
		RootDropRatio:        DefaultRootDropRatio,
		SelectorMinRatio:     DefaultSelectorMinRatio,
		HTTPFailRatio:        DefaultHTTPFailRatio,
		ResponseMissingRatio: DefaultResponseMissingRatio,
		MinCards:             DefaultMinCards,
		MinPages:             DefaultMinPages,
	}
	if ratio, ok := readFloat("INGESTION_DEGRADED_REJECT_RATIO", 0, 1); ok {
		thresholds.RejectRatio = ratio
	}
	if ratio, ok := readFloat("INGESTION_DEGRADED_DROP_RATIO", 0, 1); ok {
		thresholds.DropRatio = ratio
	}
	if ratio, ok := readFloat("INGESTION_DEGRADED_PRICE_RATIO", 0, 1); ok {
		thresholds.PriceRatio = ratio
	}
	if runs, ok := readInt("INGESTION_DEGRADED_BASELINE_RUNS", 1, 50); ok {
		thresholds.BaselineRuns = runs
	}
	if ratio, ok := readFloat("INGESTION_FP_ROOT_DROP_RATIO", 0, 1); ok {
		thresholds.RootDropRatio = ratio
	}
	if ratio, ok := readFloat("INGESTION_FP_SELECTOR_MIN_RATIO", 0, 1); ok {
		thresholds.SelectorMinRatio = ratio
	}
	if ratio, ok := readFloat("INGESTION_FP_HTTP_FAIL_RATIO", 0, 1); ok {
		thresholds.HTTPFailRatio = ratio
	}
	if ratio, ok := readFloat("INGESTION_FP_RESPONSE_MISSING_RATIO", 0, 1); ok {
		thresholds.ResponseMissingRatio = ratio
	}
	if cards, ok := readInt("INGESTION_FP_MIN_CARDS", 1, 100000); ok {
		thresholds.MinCards = cards
	}
	if pages, ok := readInt("INGESTION_FP_MIN_PAGES", 1, 100000); ok {
		thresholds.MinPages = pages
	}
	return thresholds
}

// Degradation es el veredicto sobre una corrida cerrada.
type Degradation struct {
	// Degraded indica si hay que avisar a los administradores.
	Degraded bool
	// Reasons explica cada detección, en español, listo para el mensaje.
	Reasons []string
}

// EvaluateDegradation decide si una corrida dejó de entregar datos.
//
// Compara los contadores del scraper contra sí mismos (nada encontrado, nada
// enviado, demasiados rechazos, exceso de precios nulos o en 0) y contra la
// mediana de las últimas corridas exitosas de la misma tienda. Los duplicados
// descartados cuentan como entrega: un listado que repite tarjetas no es una
// tienda caída (TAL-172). Nunca inventa una alerta: sin contadores, con `Found`
// en cero o sin historial suficiente la respuesta es «no degradada».
// Devuelve el veredicto con las razones que lo sostienen.
func EvaluateDegradation(counters Counters, baseline []int32, thresholds Thresholds) Degradation {
	if counters.Found <= 0 {
		return Degradation{}
	}

	var reasons []string
	switch {
	case counters.Sent == 0:
		reason := "no se envió ningún ítem a ingestion"
		if counters.PagesFailed > 0 {
			reason += fmt.Sprintf(" (%d página(s) del listado no se pudieron leer)", counters.PagesFailed)
		}
		reasons = append(reasons, reason)
	default:
		// Con envíos reales sólo se compara contra el historial; una página
		// caída de veinte no justifica una alerta si el catálogo llegó igual.
		if reference := median(baseline); reference > 0 {
			// Los duplicados no son una pérdida: si el listado trajo el mismo
			// producto dos veces, `Sent` cae sin que la tienda esté rota.
			effective := counters.Sent + counters.Duplicated
			if limit := int(float64(reference) * thresholds.DropRatio); effective < limit {
				reason := fmt.Sprintf(
					"envió %d ítems contra una mediana de %d en las últimas corridas",
					counters.Sent, reference,
				)
				if counters.Duplicated > 0 {
					reason += fmt.Sprintf(" (%d tarjetas repetidas descartadas)", counters.Duplicated)
				}
				reasons = append(reasons, reason)
			}
		}
	}
	if counters.Rejected > 0 {
		ratio := float64(counters.Rejected) / float64(counters.Found)
		if ratio > thresholds.RejectRatio {
			reasons = append(reasons, fmt.Sprintf(
				"%d de %d tarjetas rechazadas (%.0f%% supera el umbral de %.0f%%)",
				counters.Rejected, counters.Found, ratio*100, thresholds.RejectRatio*100,
			))
		}
	}
	if counters.PricesTotal > 0 {
		ratio := float64(counters.ZeroPrices) / float64(counters.PricesTotal)
		if ratio > thresholds.PriceRatio {
			reasons = append(reasons, fmt.Sprintf(
				"%d de %d ítems con precio nulo o 0 (%.0f%% supera el umbral de %.0f%%)",
				counters.ZeroPrices, counters.PricesTotal, ratio*100, thresholds.PriceRatio*100,
			))
		}
	}

	return Degradation{Degraded: len(reasons) > 0, Reasons: reasons}
}

// allItemsFailed revisa lo que ingestion registró de la corrida, sin mirar
// los contadores del scraper: si no procesó ningún ítem y rechazó al menos
// uno, la tienda no entregó nada utilizable (TAL-250). Sirve también con
// scrapers que no envían contadores. Devuelve el veredicto con su razón.
func allItemsFailed(run Run) Degradation {
	if run.RecordsProcessed > 0 || run.RecordsFailed <= 0 {
		return Degradation{}
	}
	return Degradation{
		Degraded: true,
		Reasons:  []string{fmt.Sprintf("ingestion rechazó los %d ítems enviados", run.RecordsFailed)},
	}
}

// median devuelve la mediana de `values` sin modificar la entrada.
//
// Con un número par de elementos toma el menor de los dos centrales, para
// quedarse con un valor que existió de verdad en el historial. Sin elementos
// devuelve 0, que el llamador interpreta como «sin referencia».
func median(values []int32) int {
	if len(values) == 0 {
		return 0
	}
	ordered := make([]int32, len(values))
	copy(ordered, values)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return int(ordered[(len(ordered)-1)/2])
}

// ChangeEvent es un cambio detectado en una fuente, con su tipo y el motivo.
// Un mismo cierre puede producir como máximo un evento por tipo.
type ChangeEvent struct {
	Tipo    string
	Reasons []string
}

// EvaluateFingerprint compara la huella de la corrida con las anteriores de la
// misma tienda y devuelve los cambios detectados, agrupados por tipo:
// `endpoint` (HTTP/content-type), `estructura` (selectores clave),
// `respuesta` (claves o tipos del JSON-LD) y `registros` (caída de tarjetas).
//
// Es deliberadamente conservador: sin historial suficiente, sin la muestra
// mínima de páginas o con la tienda por debajo de MinCards no alerta. Compara
// conteos y esquemas, nunca el HTML completo, para que un anuncio o contenido
// dinámico no se confunda con un cambio de estructura. Devuelve una lista
// vacía cuando no hay nada que avisar.
func EvaluateFingerprint(current Fingerprint, baseline []Fingerprint, thresholds Thresholds) []ChangeEvent {
	var events []ChangeEvent

	if reason := endpointChange(current, thresholds); reason != "" {
		events = append(events, ChangeEvent{Tipo: ChangeTypeEndpoint, Reasons: []string{reason}})
	}
	var structure []string
	if current.RootMatches >= thresholds.MinCards {
		for _, selector := range current.RequiredSelectors {
			hits := current.SelectorHits[selector]
			if float64(hits) < float64(current.RootMatches)*thresholds.SelectorMinRatio {
				structure = append(structure, fmt.Sprintf(
					"el selector requerido %q coincidió en %d de %d tarjetas",
					selector, hits, current.RootMatches))
			}
		}
	}
	if len(structure) > 0 {
		events = append(events, ChangeEvent{Tipo: ChangeTypeStructure, Reasons: structure})
	}
	if reason := responseChange(current, baseline, thresholds); reason != "" {
		events = append(events, ChangeEvent{Tipo: ChangeTypeResponse, Reasons: []string{reason}})
	}
	if reason := recordsChange(current, baseline, thresholds); reason != "" {
		events = append(events, ChangeEvent{Tipo: ChangeTypeRecords, Reasons: []string{reason}})
	}
	return events
}

// endpointChange detecta páginas que dejaron de responder HTML. Devuelve el
// motivo o "" cuando la muestra es insuficiente o la proporción está dentro
// del umbral.
func endpointChange(current Fingerprint, thresholds Thresholds) string {
	if current.PagesChecked < thresholds.MinPages {
		return ""
	}
	bad := current.PagesHTTPFailed + current.PagesNonHTML
	ratio := float64(bad) / float64(current.PagesChecked)
	if ratio <= thresholds.HTTPFailRatio {
		return ""
	}
	return fmt.Sprintf(
		"%d de %d páginas con HTTP >= 400 o contenido no HTML (%.0f%% supera el umbral de %.0f%%)",
		bad, current.PagesChecked, ratio*100, thresholds.HTTPFailRatio*100)
}

// responseChange detecta un JSON-LD que dejó de traer las claves o los tipos
// que el historial considera esperados. La referencia es lo que apareció en al
// menos la mitad de las corridas anteriores, no una corrida suelta.
func responseChange(current Fingerprint, baseline []Fingerprint, thresholds Thresholds) string {
	if len(baseline) < 2 {
		return ""
	}
	expectedKeys := consensus(baseline, func(fp Fingerprint) []string { return fp.JSONLDKeys })
	if len(expectedKeys) >= 3 {
		missing := missingValues(expectedKeys, current.JSONLDKeys)
		if ratio := float64(len(missing)) / float64(len(expectedKeys)); ratio > thresholds.ResponseMissingRatio {
			return fmt.Sprintf(
				"faltan %d de %d claves JSON-LD esperadas (%s); %.0f%% supera el umbral de %.0f%%",
				len(missing), len(expectedKeys), strings.Join(missing, ", "),
				ratio*100, thresholds.ResponseMissingRatio*100)
		}
	}
	expectedTypes := consensus(baseline, func(fp Fingerprint) []string { return fp.JSONLDTypes })
	if len(expectedTypes) > 0 {
		if missing := missingValues(expectedTypes, current.JSONLDTypes); len(missing) > 0 {
			return fmt.Sprintf("faltan tipos JSON-LD esperados (%s)", strings.Join(missing, ", "))
		}
	}
	return ""
}

// recordsChange compara las tarjetas vistas contra la mediana histórica.
// Devuelve el motivo o "" cuando no hay historial suficiente o la caída no
// cruza el umbral.
func recordsChange(current Fingerprint, baseline []Fingerprint, thresholds Thresholds) string {
	roots := make([]int32, 0, len(baseline))
	for _, fp := range baseline {
		if fp.RootMatches > 0 {
			roots = append(roots, int32(fp.RootMatches))
		}
	}
	if len(roots) < 2 {
		return ""
	}
	reference := median(roots)
	if reference < thresholds.MinCards {
		return ""
	}
	if limit := int(float64(reference) * thresholds.RootDropRatio); current.RootMatches < limit {
		return fmt.Sprintf(
			"vio %d tarjetas contra una mediana de %d en las últimas corridas",
			current.RootMatches, reference)
	}
	return ""
}

// consensus devuelve los valores presentes en al menos la mitad de las huellas
// del historial. Un valor que apareció una sola vez no es referencia.
func consensus(baseline []Fingerprint, values func(Fingerprint) []string) []string {
	counts := map[string]int{}
	for _, fp := range baseline {
		seen := map[string]bool{}
		for _, value := range values(fp) {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			counts[value]++
		}
	}
	needed := (len(baseline) + 1) / 2
	out := make([]string, 0, len(counts))
	for value, count := range counts {
		if count >= needed {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// missingValues devuelve los valores esperados que no están en current, en el
// orden de expected.
func missingValues(expected, current []string) []string {
	present := make(map[string]bool, len(current))
	for _, value := range current {
		present[value] = true
	}
	var out []string
	for _, value := range expected {
		if !present[value] {
			out = append(out, value)
		}
	}
	return out
}

// readFloat lee una variable de entorno como número decimal dentro de
// [low, high]. Devuelve (valor, true) sólo cuando existe y es válida.
func readFloat(name string, low, high float64) (float64, bool) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < low || value > high {
		return 0, false
	}
	return value, true
}

// readInt lee una variable de entorno como entero dentro de [low, high].
// Devuelve (valor, true) sólo cuando existe y es válida.
func readInt(name string, low, high int) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < low || value > high {
		return 0, false
	}
	return value, true
}
