package ingestion

import (
	"strings"
	"testing"
)

// TestEvaluateDegradation fija la regla de TAL-211: cuándo una corrida deja de
// entregar datos. Cada caso usa los umbrales por defecto.
func TestEvaluateDegradation(t *testing.T) {
	thresholds := Thresholds{RejectRatio: DefaultRejectRatio, DropRatio: DefaultDropRatio, BaselineRuns: DefaultBaselineRuns, PriceRatio: DefaultPriceRatio}
	tests := []struct {
		name     string
		counters Counters
		baseline []int32
		degraded bool
		contains string
	}{
		{
			name:     "sin contadores no hay veredicto",
			counters: Counters{},
		},
		{
			name:     "primera corrida con envíos normales no alerta",
			counters: Counters{Found: 28, Sent: 28},
		},
		{
			name:     "nada enviado alerta",
			counters: Counters{Found: 28, Sent: 0},
			degraded: true,
			contains: "no se envió ningún ítem",
		},
		{
			name:     "nada enviado con páginas caídas explica el porqué",
			counters: Counters{Found: 28, Sent: 0, PagesFailed: 3},
			degraded: true,
			contains: "3 página(s) del listado no se pudieron leer",
		},
		{
			name:     "muchos rechazos alertan",
			counters: Counters{Found: 20, Sent: 12, Rejected: 8},
			degraded: true,
			contains: "8 de 20 tarjetas rechazadas",
		},
		{
			name:     "pocos rechazos no alertan",
			counters: Counters{Found: 20, Sent: 18, Rejected: 2},
		},
		{
			name:     "caída fuerte contra la mediana alerta",
			counters: Counters{Found: 30, Sent: 8},
			baseline: []int32{30, 28, 31, 29, 30},
			degraded: true,
			contains: "envió 8 ítems contra una mediana de 30",
		},
		{
			name:     "caída leve contra la mediana no alerta",
			counters: Counters{Found: 30, Sent: 26},
			baseline: []int32{30, 28, 31, 29, 30},
		},
		{
			name:     "una página caída con envíos normales no alerta",
			counters: Counters{Found: 30, Sent: 30, PagesFailed: 1},
		},
		{
			name:     "historial en cero no sirve de referencia",
			counters: Counters{Found: 5, Sent: 5},
			baseline: []int32{0, 0, 0},
		},
		{
			name:     "los duplicados explican la caída",
			counters: Counters{Found: 12, Sent: 12, Duplicated: 18},
			baseline: []int32{30, 30, 30},
		},
		{
			name:     "los duplicados no alcanzan y se nombran",
			counters: Counters{Found: 6, Sent: 6, Duplicated: 2},
			baseline: []int32{30, 30, 30},
			degraded: true,
			contains: "6 ítems contra una mediana de 30",
		},
		{
			name:     "los duplicados no evitan alertar por rechazos",
			counters: Counters{Found: 20, Sent: 10, Rejected: 10, Duplicated: 10},
			degraded: true,
			contains: "10 de 20 tarjetas rechazadas",
		},
		{
			name:     "demasiados precios nulos o en cero alertan",
			counters: Counters{Found: 20, Sent: 20, ZeroPrices: 8, PricesTotal: 20},
			degraded: true,
			contains: "8 de 20 ítems con precio nulo o 0",
		},
		{
			name:     "pocos precios nulos o en cero no alertan",
			counters: Counters{Found: 20, Sent: 20, ZeroPrices: 2, PricesTotal: 20},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := EvaluateDegradation(test.counters, test.baseline, thresholds)
			if got.Degraded != test.degraded {
				t.Fatalf("Degraded = %v; quiero %v (razones: %v)", got.Degraded, test.degraded, got.Reasons)
			}
			if !test.degraded && len(got.Reasons) != 0 {
				t.Fatalf("sin degradación no debe haber razones: %v", got.Reasons)
			}
			if test.contains != "" && !containsReason(got.Reasons, test.contains) {
				t.Fatalf("razones = %v; quiero una que contenga %q", got.Reasons, test.contains)
			}
		})
	}
}

// TestEvaluateDegradationAcumulaRazones comprueba que dos causas a la vez se
// informan por separado, para que el administrador vea el cuadro completo.
func TestEvaluateDegradationAcumulaRazones(t *testing.T) {
	got := EvaluateDegradation(
		Counters{Found: 10, Sent: 0, Rejected: 9, PagesFailed: 2},
		nil,
		Thresholds{RejectRatio: DefaultRejectRatio, DropRatio: DefaultDropRatio, PriceRatio: DefaultPriceRatio},
	)

	if !got.Degraded || len(got.Reasons) != 2 {
		t.Fatalf("razones = %v; quiero dos", got.Reasons)
	}
}

// TestMedian fija el criterio: sin elementos devuelve 0 y con un número par
// toma el menor de los dos centrales, que es un valor que existió de verdad.
func TestMedian(t *testing.T) {
	tests := []struct {
		values []int32
		want   int
	}{
		{nil, 0},
		{[]int32{7}, 7},
		{[]int32{5, 1, 3}, 3},
		{[]int32{4, 1, 3, 2}, 2},
		{[]int32{30, 28, 31, 29, 30}, 30},
	}
	for _, test := range tests {
		if got := median(test.values); got != test.want {
			t.Errorf("median(%v) = %d; quiero %d", test.values, got, test.want)
		}
	}
}

// TestThresholdsFromEnv comprueba que los umbrales se pueden ajustar sin
// recompilar y que un valor inválido no apaga las alertas.
func TestThresholdsFromEnv(t *testing.T) {
	t.Run("por defecto", func(t *testing.T) {
		got := ThresholdsFromEnv()
		if got.RejectRatio != DefaultRejectRatio || got.DropRatio != DefaultDropRatio ||
			got.BaselineRuns != DefaultBaselineRuns || got.PriceRatio != DefaultPriceRatio {
			t.Fatalf("umbrales = %+v", got)
		}
	})

	t.Run("desde el entorno", func(t *testing.T) {
		t.Setenv("INGESTION_DEGRADED_REJECT_RATIO", "0.5")
		t.Setenv("INGESTION_DEGRADED_DROP_RATIO", "0.8")
		t.Setenv("INGESTION_DEGRADED_BASELINE_RUNS", "10")
		t.Setenv("INGESTION_DEGRADED_PRICE_RATIO", "0.1")

		got := ThresholdsFromEnv()
		if got.RejectRatio != 0.5 || got.DropRatio != 0.8 || got.BaselineRuns != 10 || got.PriceRatio != 0.1 {
			t.Fatalf("umbrales = %+v", got)
		}
	})

	t.Run("valores inválidos se ignoran", func(t *testing.T) {
		t.Setenv("INGESTION_DEGRADED_REJECT_RATIO", "dos")
		t.Setenv("INGESTION_DEGRADED_DROP_RATIO", "3")
		t.Setenv("INGESTION_DEGRADED_BASELINE_RUNS", "0")
		t.Setenv("INGESTION_DEGRADED_PRICE_RATIO", "-1")

		got := ThresholdsFromEnv()
		if got.RejectRatio != DefaultRejectRatio || got.DropRatio != DefaultDropRatio ||
			got.BaselineRuns != DefaultBaselineRuns || got.PriceRatio != DefaultPriceRatio {
			t.Fatalf("umbrales = %+v", got)
		}
	})
}

// healthyFingerprint es la huella de una tienda estable: 30 tarjetas, ambos
// selectores requeridos coincidiendo en todas y diez páginas HTML correctas.
func healthyFingerprint() Fingerprint {
	return Fingerprint{
		RootMatches:       30,
		SelectorHits:      map[string]int{"product.name": 30, "offer.price": 30},
		RequiredSelectors: []string{"product.name", "offer.price"},
		JSONLDKeys:        []string{"image", "name", "sku", "url"},
		JSONLDTypes:       []string{"Product"},
		PagesChecked:      10,
	}
}

// TestEvaluateFingerprint fija la regla de TAL-209: qué cuenta como cambio de
// estructura, respuesta, endpoint o registros, y qué no dispara nada.
func TestEvaluateFingerprint(t *testing.T) {
	thresholds := Thresholds{
		RootDropRatio:        DefaultRootDropRatio,
		SelectorMinRatio:     DefaultSelectorMinRatio,
		HTTPFailRatio:        DefaultHTTPFailRatio,
		ResponseMissingRatio: DefaultResponseMissingRatio,
		MinCards:             DefaultMinCards,
		MinPages:             DefaultMinPages,
	}
	stable := healthyFingerprint()
	baseline := []Fingerprint{stable, stable, stable}

	withHits := func(changes func(*Fingerprint)) Fingerprint {
		fp := healthyFingerprint()
		changes(&fp)
		return fp
	}

	tests := []struct {
		name    string
		current Fingerprint
		base    []Fingerprint
		tipo    string // "" cuando no debe haber eventos
		reason  string
	}{
		{
			name:    "sin cambios no genera evento",
			current: healthyFingerprint(),
			base:    baseline,
		},
		{
			name:    "selector requerido sin coincidencias es cambio de estructura",
			current: withHits(func(fp *Fingerprint) { fp.SelectorHits["offer.price"] = 0 }),
			base:    baseline,
			tipo:    ChangeTypeStructure,
			reason:  "offer.price",
		},
		{
			name:    "pocas coincidencias bajo el 10% también son estructura",
			current: withHits(func(fp *Fingerprint) { fp.SelectorHits["offer.price"] = 2 }),
			base:    baseline,
			tipo:    ChangeTypeStructure,
			reason:  "coincidió en 2 de 30",
		},
		{
			name:    "faltan claves JSON-LD esperadas",
			current: withHits(func(fp *Fingerprint) { fp.JSONLDKeys = []string{"url"} }),
			base:    baseline,
			tipo:    ChangeTypeResponse,
			reason:  "claves JSON-LD esperadas",
		},
		{
			name:    "cambia el tipo JSON-LD de la ficha",
			current: withHits(func(fp *Fingerprint) { fp.JSONLDTypes = []string{"ListItem"} }),
			base:    baseline,
			tipo:    ChangeTypeResponse,
			reason:  "tipos JSON-LD esperados",
		},
		{
			name:    "páginas con HTTP >= 400 son cambio de endpoint",
			current: withHits(func(fp *Fingerprint) { fp.PagesHTTPFailed = 3 }),
			base:    baseline,
			tipo:    ChangeTypeEndpoint,
			reason:  "HTTP >= 400",
		},
		{
			name:    "páginas que dejan de ser HTML son cambio de endpoint",
			current: withHits(func(fp *Fingerprint) { fp.PagesNonHTML = 3 }),
			base:    baseline,
			tipo:    ChangeTypeEndpoint,
			reason:  "contenido no HTML",
		},
		{
			name:    "pocas páginas no bastan para evaluar el endpoint",
			current: withHits(func(fp *Fingerprint) { fp.PagesChecked = 4; fp.PagesHTTPFailed = 4 }),
			base:    baseline,
		},
		{
			name:    "caída de tarjetas contra la mediana es cambio de registros",
			current: withHits(func(fp *Fingerprint) {
				fp.RootMatches = 10
				fp.SelectorHits = map[string]int{"product.name": 10, "offer.price": 10}
			}),
			base:   baseline,
			tipo:   ChangeTypeRecords,
			reason: "vio 10 tarjetas contra una mediana de 30",
		},
		{
			name:    "caída leve no alerta",
			current: withHits(func(fp *Fingerprint) {
				fp.RootMatches = 25
				fp.SelectorHits = map[string]int{"product.name": 25, "offer.price": 25}
			}),
			base: baseline,
		},
		{
			name:    "sin historial suficiente no compara registros ni respuesta",
			current: withHits(func(fp *Fingerprint) { fp.RootMatches = 3; fp.JSONLDKeys = nil }),
			base:    []Fingerprint{healthyFingerprint()},
		},
		{
			name:    "una tienda por debajo del mínimo no alerta por registros",
			current: withHits(func(fp *Fingerprint) {
				fp.RootMatches = 3
				fp.SelectorHits = map[string]int{"product.name": 3, "offer.price": 3}
			}),
			base: []Fingerprint{
				withHits(func(fp *Fingerprint) { fp.RootMatches = 8 }),
				withHits(func(fp *Fingerprint) { fp.RootMatches = 8 }),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events := EvaluateFingerprint(test.current, test.base, thresholds)
			if test.tipo == "" {
				if len(events) != 0 {
					t.Fatalf("eventos = %+v; quiero ninguno", events)
				}
				return
			}
			event := eventOf(events, test.tipo)
			if event == nil {
				t.Fatalf("faltó el evento %q en %+v", test.tipo, events)
			}
			if test.reason != "" && !containsReason(event.Reasons, test.reason) {
				t.Fatalf("razones = %v; quiero una que contenga %q", event.Reasons, test.reason)
			}
		})
	}
}

// TestEvaluateFingerprintUnEventoPorTipo comprueba que varias causas del mismo
// tipo se agrupan: la base tiene UNIQUE (run_id, tipo).
func TestEvaluateFingerprintUnEventoPorTipo(t *testing.T) {
	thresholds := Thresholds{
		RootDropRatio: DefaultRootDropRatio, SelectorMinRatio: DefaultSelectorMinRatio,
		HTTPFailRatio: DefaultHTTPFailRatio, ResponseMissingRatio: DefaultResponseMissingRatio,
		MinCards: DefaultMinCards, MinPages: DefaultMinPages,
	}
	stable := healthyFingerprint()
	current := healthyFingerprint()
	current.SelectorHits["product.name"] = 0
	current.SelectorHits["offer.price"] = 0

	events := EvaluateFingerprint(current, []Fingerprint{stable, stable}, thresholds)

	if len(events) != 1 || events[0].Tipo != ChangeTypeStructure || len(events[0].Reasons) != 2 {
		t.Fatalf("eventos = %+v; quiero un evento de estructura con dos razones", events)
	}
}

// eventOf devuelve el evento del tipo pedido, o nil si no está.
func eventOf(events []ChangeEvent, tipo string) *ChangeEvent {
	for i := range events {
		if events[i].Tipo == tipo {
			return &events[i]
		}
	}
	return nil
}

// containsReason informa si alguna razón contiene `fragment`.
func containsReason(reasons []string, fragment string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, fragment) {
			return true
		}
	}
	return false
}
