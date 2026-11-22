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

// containsReason informa si alguna razón contiene `fragment`.
func containsReason(reasons []string, fragment string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, fragment) {
			return true
		}
	}
	return false
}
