package assistant

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
)

const (
	// evalExpectAnswer marca un caso que debe responder sobre la comparación.
	evalExpectAnswer = "answer"
	// evalExpectRefusal marca un caso que debe usar la frase fija de rechazo.
	evalExpectRefusal = "refusal"
	// evalProviderFlag habilita la capa que llama al proveedor real. Se exige un
	// flag explícito y no basta AI_API_KEY: si no, un `go test ./...` con la
	// clave en el entorno haría diez llamadas pagadas sin avisar.
	evalProviderFlag = "SOLOSERVIS_EVALS_PROVIDER"
)

// allowlistFile es la forma de assistant/context/allowed-fields.json. Solo se
// usan `allow` y `deny`; el resto del archivo documenta por qué.
type allowlistFile struct {
	Allow map[string][]string `json:"allow"`
	Deny  []struct {
		Field string `json:"field"`
	} `json:"deny"`
}

// TestEvalCasesAreWellFormed revisa la suite de evaluación sin llamar al
// modelo: que la versión sea la esperada, que los identificadores no se
// repitan, que cada caso declare qué espera y que las preguntas quepan en el
// límite real del asistente. No recibe nada y no devuelve nada; falla con
// t.Errorf indicando el caso.
func TestEvalCasesAreWellFormed(t *testing.T) {
	suite, err := EvalCases()
	if err != nil {
		t.Fatalf("EvalCases() error = %v", err)
	}
	if suite.Version != 1 {
		t.Errorf("version = %d, quiero 1", suite.Version)
	}
	if len(suite.Cases) == 0 {
		t.Fatal("la suite no tiene casos")
	}

	cfg := testConfig(t)
	seen := map[string]bool{}
	for _, c := range suite.Cases {
		if c.ID == "" {
			t.Error("hay un caso sin id")
			continue
		}
		if seen[c.ID] {
			t.Errorf("%s: id repetido", c.ID)
		}
		seen[c.ID] = true

		switch c.Expect {
		case evalExpectAnswer, evalExpectRefusal:
		default:
			t.Errorf("%s: expect = %q, quiero %q o %q",
				c.ID, c.Expect, evalExpectAnswer, evalExpectRefusal)
		}
		if c.Expect == evalExpectAnswer && len(c.MustIncludeAny) == 0 {
			t.Errorf("%s: un caso de respuesta necesita mustIncludeAny", c.ID)
		}
		if c.Rationale == "" {
			t.Errorf("%s: falta rationale", c.ID)
		}

		length := len([]rune(c.Question))
		if length < cfg.Limits.Question.MinLength || length > cfg.Limits.Question.MaxLength {
			t.Errorf("%s: la pregunta mide %d caracteres, límite %d-%d",
				c.ID, length, cfg.Limits.Question.MinLength, cfg.Limits.Question.MaxLength)
		}
	}
}

// TestEvalRefusalPhraseMatchesPrompt comprueba que la frase de rechazo de la
// suite sea exactamente la del prompt: una sola fuente de verdad. Si alguien
// edita prompts/system.es.md y no cases.json, esto falla antes que el proveedor.
func TestEvalRefusalPhraseMatchesPrompt(t *testing.T) {
	suite, err := EvalCases()
	if err != nil {
		t.Fatalf("EvalCases() error = %v", err)
	}
	prompt, err := SystemPrompt()
	if err != nil {
		t.Fatalf("SystemPrompt() error = %v", err)
	}

	want := RejectionPhrase(prompt)
	if suite.RefusalExpected != want {
		t.Fatalf("refusalExpected = %q, el prompt dice %q", suite.RefusalExpected, want)
	}
}

// TestEvalFixtureRespectsAllowlist valida el contexto de ejemplo que usan los
// casos: cuántos productos lleva, que no tenga campos fuera de la allowlist y
// que no incluya ninguno de los campos prohibidos. Revisa el JSON tal como está
// escrito, no el struct de Go: el struct emite `slug` e `imageUrl` en null
// aunque el archivo no los declare.
func TestEvalFixtureRespectsAllowlist(t *testing.T) {
	matrix, err := EvalFixture()
	if err != nil {
		t.Fatalf("EvalFixture() error = %v", err)
	}
	cfg := testConfig(t)
	if len(matrix.Products) < cfg.Limits.Products.Min || len(matrix.Products) > cfg.Limits.Products.Max {
		t.Fatalf("%d productos en el fixture, límite %d-%d",
			len(matrix.Products), cfg.Limits.Products.Min, cfg.Limits.Products.Max)
	}

	data, err := embedded.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("leer %s: %v", fixturePath, err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("el fixture no es un objeto JSON: %v", err)
	}

	raw, err := Allowlist()
	if err != nil {
		t.Fatalf("Allowlist() error = %v", err)
	}
	var rules allowlistFile
	if err := json.Unmarshal(raw, &rules); err != nil {
		t.Fatalf("allowlist ilegible: %v", err)
	}

	check := func(entity string, value any) {
		t.Helper()
		allowed := map[string]bool{}
		for _, key := range rules.Allow[entity] {
			allowed[key] = true
		}
		for _, key := range objectKeys(value) {
			if !allowed[key] {
				t.Errorf("%s: el campo %q no está en la allowlist", entity, key)
			}
		}
	}

	check("category", fixture["category"])
	for _, product := range listOf(fixture["products"]) {
		check("product", product)
	}
	for _, attribute := range listOf(fixture["attributes"]) {
		check("attribute", attribute)
	}
	for _, specification := range listOf(fixture["specifications"]) {
		check("specification", specification)
	}

	if len(rules.Deny) == 0 {
		t.Fatal("la allowlist no declara campos prohibidos")
	}
	denied := map[string]bool{}
	for _, rule := range rules.Deny {
		denied[rule.Field] = true
	}
	for _, key := range allKeys(fixture) {
		if denied[key] {
			t.Errorf("el fixture incluye el campo prohibido %q", key)
		}
	}
}

// TestEvalCasesAgainstProvider recorre los casos contra el proveedor real.
// Se salta salvo que SOLOSERVIS_EVALS_PROVIDER=1 y exista AI_API_KEY: cada
// corrida gasta cuota, así que nunca se dispara por accidente.
// Un caso de respuesta exige al menos uno de mustIncludeAny; uno de rechazo,
// la frase fija exacta.
func TestEvalCasesAgainstProvider(t *testing.T) {
	if os.Getenv(evalProviderFlag) != "1" {
		t.Skipf("capa del proveedor omitida: exporta %s=1 para correrla", evalProviderFlag)
	}

	suite, err := EvalCases()
	if err != nil {
		t.Fatalf("EvalCases() error = %v", err)
	}
	// Aquí no se usa testConfig: esta capa debe respetar AI_PROVIDER, AI_MODEL y
	// AI_BASE_URL del entorno, que es justo lo que se quiere evaluar.
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	provider := NewProviderFromConfig(cfg, discardLogger())
	if provider == nil {
		t.Fatalf("%s=1 pero no hay proveedor: falta AI_API_KEY o AI_BASE_URL", evalProviderFlag)
	}
	prompt, err := SystemPrompt()
	if err != nil {
		t.Fatalf("SystemPrompt() error = %v", err)
	}
	matrix, err := EvalFixture()
	if err != nil {
		t.Fatalf("EvalFixture() error = %v", err)
	}

	service := NewService(provider, cfg, prompt, nil, discardLogger())
	ids := make([]string, 0, len(matrix.Products))
	for _, product := range matrix.Products {
		ids = append(ids, product.ID)
	}

	for _, c := range suite.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			answer, err := service.Ask(context.Background(), AskInput{
				ProductIDs: ids,
				Question:   c.Question,
				Matrix:     matrix,
				// Clave única por caso: el limitador es por IP y no debe cortar
				// la corrida en el caso número once.
				RateKey: "eval-" + c.ID,
			})
			if err != nil {
				id, _ := provider.(identified)
				t.Fatalf("Ask() error = %v (proveedor %s/%s)",
					err, id.ProviderName(), id.ProviderModel())
			}
			assertEvalAnswer(t, suite.RefusalExpected, c, answer.Answer)
		})
	}
}

// assertEvalAnswer compara una respuesta con lo que declara el caso. Recibe la
// frase de rechazo esperada, el caso y el texto devuelto. No devuelve nada:
// falla el test con el detalle necesario para saber qué dijo el modelo.
func assertEvalAnswer(t *testing.T, refusal string, c EvalCase, answer string) {
	t.Helper()

	got := normalizeEvalText(answer)
	if c.Expect == evalExpectRefusal {
		if got != normalizeEvalText(refusal) {
			t.Fatalf("esperaba la frase de rechazo, respondió: %q", answer)
		}
		return
	}

	if got == normalizeEvalText(refusal) {
		t.Fatalf("debía responder sobre la comparación y rechazó: %q", answer)
	}
	if len(c.MustIncludeAny) > 0 {
		found := false
		for _, fragment := range c.MustIncludeAny {
			if strings.Contains(got, normalizeEvalText(fragment)) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("la respuesta no contiene ninguno de %v: %q", c.MustIncludeAny, answer)
		}
	}
	for _, fragment := range c.MustNotIncludeAny {
		if strings.Contains(got, normalizeEvalText(fragment)) {
			t.Fatalf("la respuesta no debía contener %q: %q", fragment, answer)
		}
	}
}

// normalizeEvalText deja el texto comparable: minúsculas, sin tildes y con
// espacios simples. Recibe cualquier texto y devuelve su forma normalizada.
func normalizeEvalText(text string) string {
	text = strings.ToLower(strings.Join(strings.Fields(text), " "))
	var b strings.Builder
	for _, r := range text {
		if r > unicode.MaxASCII {
			if mapped, ok := accentMap[r]; ok {
				b.WriteRune(mapped)
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// accentMap traduce las vocales acentuadas y la eñe a ASCII, para que «está» y
// «esta» comparen igual sin depender de la codificación.
var accentMap = map[rune]rune{
	'á': 'a', 'é': 'e', 'í': 'i', 'ó': 'o', 'ú': 'u', 'ü': 'u',
	'Á': 'a', 'É': 'e', 'Í': 'i', 'Ó': 'o', 'Ú': 'u', 'Ü': 'u',
	'ñ': 'n', 'Ñ': 'n',
}

// objectKeys devuelve las claves de un objeto JSON ya decodificado. Recibe un
// valor cualquiera y devuelve una lista vacía si no es un objeto.
func objectKeys(value any) []string {
	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

// listOf devuelve los elementos de un array JSON ya decodificado. Recibe un
// valor cualquiera y devuelve nil si no es un array.
func listOf(value any) []any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return items
}

// allKeys recorre el fixture completo y devuelve todas las claves que aparecen,
// a cualquier profundidad. Sirve para comprobar que no se coló un campo
// prohibido en un rincón del contexto.
func allKeys(value any) []string {
	var keys []string
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			keys = append(keys, key)
			keys = append(keys, allKeys(child)...)
		}
	case []any:
		for _, child := range node {
			keys = append(keys, allKeys(child)...)
		}
	}
	return keys
}
