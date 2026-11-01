package allapis

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAllAPIs_SonLas14(t *testing.T) {
	got := slices.Clone(allAPIs)
	sort.Strings(got)
	want := make([]string, 0, len(factories))
	for api := range factories {
		want = append(want, api)
	}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Fatalf("allAPIs = %v, quiero %v", got, want)
	}
}

// Cada API existe y está en un solo dominio, y ninguna queda sin dominio.
func TestDomains_CadaAPIEnUnSoloDominio(t *testing.T) {
	owner := map[string]string{}
	for domain, apis := range Domains {
		for _, api := range apis {
			if _, ok := factories[api]; !ok {
				t.Errorf("dominio %s: la API %q no existe", domain, api)
			}
			if prev, ok := owner[api]; ok {
				t.Errorf("la API %s está en %s y en %s", api, prev, domain)
			}
			owner[api] = domain
		}
	}
	for _, api := range allAPIs {
		if owner[api] == "" {
			t.Errorf("la API %s no está en ningún dominio", api)
		}
	}
}

func TestDomains_UsuariosEIngestion(t *testing.T) {
	if got := Domains["usuarios"]; !slices.Equal(got, []string{"auth", "user"}) {
		t.Errorf("usuarios = %v, quiero [auth user]", got)
	}
	if got := Domains["ingestion"]; !slices.Equal(got, []string{"ingestion"}) {
		t.Errorf("ingestion = %v, quiero [ingestion]", got)
	}
}

// La única llamada HTTP que cruza de un dominio a otro es ingestion →
// notification (auditoría TAL-105, H8). Si aparece otra, el despliegue
// necesita una URL nueva entre pods.
func TestCalls_UnicaLlamadaEntreDominios(t *testing.T) {
	owner := map[string]string{}
	for domain, apis := range Domains {
		for _, api := range apis {
			owner[api] = domain
		}
	}
	var cross []string
	for caller, callees := range Calls {
		for _, callee := range callees {
			if owner[caller] != owner[callee] {
				cross = append(cross, caller+"→"+callee)
			}
		}
	}
	if !slices.Equal(cross, []string{"ingestion→notification"}) {
		t.Errorf("llamadas entre dominios = %v, quiero [ingestion→notification]", cross)
	}
}

func TestDomains_Comercial(t *testing.T) {
	want := []string{"cart", "comparison", "favorites", "notification", "pricing", "product",
		"review", "search", "service", "store", "watchlist"}
	got := slices.Clone(Domains["comercial"])
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("comercial = %v, quiero %v", got, want)
	}
	if Domains["comercial"][0] != "product" {
		t.Errorf("/health de comercial lo responde %s, quiero product", Domains["comercial"][0])
	}
}

func TestAPIs(t *testing.T) {
	if apis, err := APIs(""); err != nil || len(apis) != 14 {
		t.Errorf(`APIs("") = %v, %v; quiero las 14`, apis, err)
	}
	if apis, err := APIs("comercial"); err != nil || len(apis) != 11 {
		t.Errorf(`APIs("comercial") = %v, %v; quiero 11`, apis, err)
	}
	if apis, err := APIs("usuarios"); err != nil || len(apis) != 2 {
		t.Errorf(`APIs("usuarios") = %v, %v; quiero 2`, apis, err)
	}
	if apis, err := APIs("ingestion"); err != nil || len(apis) != 1 {
		t.Errorf(`APIs("ingestion") = %v, %v; quiero 1`, apis, err)
	}
	if _, err := APIs("catalogo"); err == nil || !strings.Contains(err.Error(), "comercial, ingestion, usuarios") {
		t.Errorf(`APIs("catalogo") = %v; quiero un error que liste los dominios`, err)
	}
}

// Las rutas de comercial llegan a su API, y las de usuarios e ingestion
// responden 404: ese proceso no las sirve.
func TestNewDomainRouter_ComercialSoloSirveSusRutas(t *testing.T) {
	r := NewDomainRouter(Domains["comercial"])(nil, "http://localhost:8443")
	cases := []struct {
		method, path string
		status       int
		code         string
	}{
		{http.MethodGet, "/categories/NO_VALIDO", http.StatusBadRequest, `"code":"invalid_parameter"`},
		{http.MethodGet, "/comparisons/products", http.StatusUnauthorized, `"code":"unauthorized"`},
		{http.MethodPost, "/auth/login", http.StatusNotFound, `"code":"not_found"`},
		{http.MethodGet, "/users/1", http.StatusNotFound, `"code":"not_found"`},
		{http.MethodGet, "/ingestion/runs", http.StatusNotFound, `"code":"not_found"`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%s %s = %d %s, quiero %d %s", c.method, c.path, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}

	// El preflight CORS lo responde la API dueña sin tocar la base: sirve para
	// probar también las APIs de features.
	for path, want := range map[string]int{"/cart/items": http.StatusNoContent, "/auth/login": http.StatusNotFound} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, path, nil))
		if rec.Code != want {
			t.Errorf("OPTIONS %s = %d, quiero %d", path, rec.Code, want)
		}
	}
}

func TestNewDomainRouter_UsuariosSoloSirveSusRutas(t *testing.T) {
	r := NewDomainRouter(Domains["usuarios"])(nil, "http://localhost:8443")
	cases := []struct {
		method, path string
		status       int
		body         string
	}{
		{http.MethodGet, "/users", http.StatusOK, `"user-api-ready"`},
		{http.MethodGet, "/users/1", http.StatusUnauthorized, `"code":"unauthorized"`},
		{http.MethodOptions, "/auth/login", http.StatusNoContent, ""},
		{http.MethodGet, "/products", http.StatusNotFound, `"code":"not_found"`},
		{http.MethodPost, "/cart/items", http.StatusNotFound, `"code":"not_found"`},
		{http.MethodGet, "/ingestion/runs", http.StatusNotFound, `"code":"not_found"`},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.body) {
			t.Errorf("%s %s = %d %s, quiero %d %s", c.method, c.path, rec.Code, rec.Body.String(), c.status, c.body)
		}
	}
}

func TestNewDomainRouter_IngestionSoloSirveSusRutas(t *testing.T) {
	r := NewDomainRouter(Domains["ingestion"])(nil, "http://localhost:8443")
	cases := map[string]int{
		"/ingestion/runs": http.StatusUnauthorized,
		"/products":       http.StatusNotFound,
		"/auth/me":        http.StatusNotFound,
		"/notifications":  http.StatusNotFound,
	}
	for path, want := range cases {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d %s, quiero %d", path, rec.Code, rec.Body.String(), want)
		}
	}
}

// Todas las rutas de cada API caen en una API de su dominio.
func TestNewDomainRouter_CadaDominioTieneTodasSusRutas(t *testing.T) {
	for domain, apis := range Domains {
		inDomain := map[string]bool{}
		for _, api := range apis {
			inDomain[api] = true
		}
		routes := 0
		for api, router := range build(apis, nil, "http://localhost:8443") {
			walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
				routes++
				if route != "/health" && !inDomain[APIFor(route)] {
					t.Errorf("%s %s (%s) no lo serviría %s", method, route, api, domain)
				}
				return nil
			}
			if err := chi.Walk(router, walk); err != nil {
				t.Fatal(err)
			}
		}
		if routes == 0 {
			t.Errorf("%s no tiene rutas", domain)
		}
	}
}

func env(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

// Comercial tiene adentro a todas las APIs que llama: no necesita URLs.
func TestInternalURLs_ComercialSeLlamaASiMismo(t *testing.T) {
	urls, err := InternalURLs(Domains["comercial"], "http://127.0.0.1:8080", env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(urls) != len(URLEnv) {
		t.Errorf("urls = %v, quiero las %d variables", urls, len(URLEnv))
	}
	for name, url := range urls {
		if url != "http://127.0.0.1:8080" {
			t.Errorf("%s = %s, quiero el propio proceso", name, url)
		}
	}
}

func TestInternalURLs_RespetaLasDefinidas(t *testing.T) {
	urls, err := InternalURLs(Domains["comercial"], "http://127.0.0.1:8080", env(map[string]string{"PRICING_API_URL": "http://pricing:8080"}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := urls["PRICING_API_URL"]; ok {
		t.Errorf("PRICING_API_URL se sobrescribió: %v", urls)
	}
}

// usuarios no llama a nadie.
func TestInternalURLs_UsuariosNoNecesitaURLs(t *testing.T) {
	urls, err := InternalURLs(Domains["usuarios"], "http://127.0.0.1:8080", env(nil))
	if err != nil || len(urls) != 0 {
		t.Fatalf("urls = %v, err = %v; quiero ninguna", urls, err)
	}
}

// ingestion llama a notification, que está en comercial: necesita la URL del
// Service de comercial, o no arranca.
func TestInternalURLs_IngestionNecesitaNotification(t *testing.T) {
	_, err := InternalURLs(Domains["ingestion"], "http://127.0.0.1:8080", env(nil))
	if err == nil || !strings.Contains(err.Error(), "NOTIFICATION_API_URL (ingestion llama a notification)") {
		t.Fatalf("err = %v, quiero que pida NOTIFICATION_API_URL", err)
	}
	urls, err := InternalURLs(Domains["ingestion"], "http://127.0.0.1:8080", env(map[string]string{"NOTIFICATION_API_URL": "http://comercial:8080"}))
	if err != nil || len(urls) != 0 {
		t.Fatalf("con la URL definida: urls = %v, err = %v", urls, err)
	}
}

func TestCalls_APIsConocidas(t *testing.T) {
	for caller, callees := range Calls {
		if _, ok := factories[caller]; !ok {
			t.Errorf("Calls: %q no es una API", caller)
		}
		for _, callee := range callees {
			if URLEnv[callee] == "" {
				t.Errorf("Calls: %s llama a %s, que no tiene variable en URLEnv", caller, callee)
			}
		}
	}
}

// URLEnv debe tener exactamente las variables *_API_URL que lee el código:
// si alguien agrega una llamada nueva entre APIs, este test lo avisa.
func TestURLEnv_CoincideConElCodigo(t *testing.T) {
	re := regexp.MustCompile(`"([A-Z]+_API_URL)"`)
	used := map[string]bool{}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			filepath.Base(filepath.Dir(path)) == "allapis" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			used[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range URLEnv {
		if !used[name] {
			t.Errorf("%s está en URLEnv pero el código no la lee", name)
		}
		delete(used, name)
	}
	for name := range used {
		t.Errorf("el código lee %s, que no está en URLEnv", name)
	}
}
