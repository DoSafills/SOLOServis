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
}

// ThresholdsFromEnv arma los umbrales con los valores por defecto y los
// sobrescribe con INGESTION_DEGRADED_REJECT_RATIO, INGESTION_DEGRADED_DROP_RATIO,
// INGESTION_DEGRADED_PRICE_RATIO e INGESTION_DEGRADED_BASELINE_RUNS. Un valor
// inválido o fuera de rango se ignora en silencio: una variable mal escrita no
// puede apagar las alertas.
func ThresholdsFromEnv() Thresholds {
	thresholds := Thresholds{
		RejectRatio:  DefaultRejectRatio,
		DropRatio:    DefaultDropRatio,
		BaselineRuns: DefaultBaselineRuns,
		PriceRatio:   DefaultPriceRatio,
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
