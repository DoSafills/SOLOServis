package allapis

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// Rutas del repo que el test lee. El gateway vive en docker/frontend porque
// es el nginx del frontend, pero el contrato que se comprueba es del backend:
// qué prefijos publica y con qué upstream.
const (
	gatewayTemplatePath = "../../../docker/frontend/default.conf.template"
	gatewayDockerfile   = "../../../docker/frontend/Dockerfile"
)

// gatewayPrivados son los prefijos que el backend sirve y el gateway NO
// publica a propósito: solo se usan por la red interna o desde otra API.
// Si uno deja de ser privado, se saca de esta lista en el mismo cambio.
var gatewayPrivados = []string{"ingestion", "notifications", "offers"}

// alternationRE captura los prefijos de una location con alternancia, como
// `location ~ ^/(products|product-categories|categories)(/.*)?$`.
var alternationRE = regexp.MustCompile(`\^/\(([^)]+)\)`)

// singlePrefixRE captura el prefijo de una location simple, como
// `location ~ ^/auth(/.*)?$`. No matchea `^/(a|b)` porque después de `^/`
// viene un paréntesis.
var singlePrefixRE = regexp.MustCompile(`\^/([a-z0-9][a-z0-9-]*)(?:\(|/|\$)`)

// upstreamVarRE captura las variables *UPSTREAM usadas en proxy_pass.
var upstreamVarRE = regexp.MustCompile(`\$\{([A-Z_]+_UPSTREAM)\}`)

// upstreamEnvRE captura las variables *UPSTREAM definidas en el Dockerfile.
var upstreamEnvRE = regexp.MustCompile(`([A-Z_]+_UPSTREAM)=`)

// readRepoFile lee un archivo del repo relativo al paquete y devuelve su
// contenido. Si no existe, salta el test: permite correr `go test` en un
// checkout que solo trae backend/ sin fallar por un archivo de infraestructura.
func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Skipf("no se pudo leer %s: %v", path, err)
	}
	return string(data)
}

// gatewayPrefixes devuelve los prefijos de ruta que el nginx del frontend
// publica a las APIs, en orden alfabético. Recibe la plantilla de nginx.
func gatewayPrefixes(template string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(template, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "location ~ ") {
			continue
		}
		for _, m := range alternationRE.FindAllStringSubmatch(trimmed, -1) {
			for _, prefix := range strings.Split(m[1], "|") {
				seen[strings.TrimSpace(prefix)] = true
			}
		}
		if m := singlePrefixRE.FindStringSubmatch(trimmed); m != nil {
			seen[m[1]] = true
		}
	}
	prefixes := make([]string, 0, len(seen))
	for prefix := range seen {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	return prefixes
}

// templateUpstreams devuelve las variables *UPSTREAM que la plantilla usa en
// proxy_pass, sin repetir. Recibe la plantilla de nginx.
func templateUpstreams(template string) []string {
	seen := map[string]bool{}
	for _, m := range upstreamVarRE.FindAllStringSubmatch(template, -1) {
		seen[m[1]] = true
	}
	return sortedKeys(seen)
}

// dockerfileUpstreams devuelve las variables *UPSTREAM con valor por defecto
// en la imagen del frontend, sin repetir. Recibe el Dockerfile.
func dockerfileUpstreams(dockerfile string) []string {
	seen := map[string]bool{}
	for _, m := range upstreamEnvRE.FindAllStringSubmatch(dockerfile, -1) {
		seen[m[1]] = true
	}
	return sortedKeys(seen)
}

// sortedKeys devuelve las claves de set en orden alfabético. Recibe el mapa
// de nombres y devuelve el arreglo ordenado.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestGateway_ExponeExactamenteLosPrefijosPublicos comprueba que el nginx del
// frontend publique todos los prefijos públicos del backend y ninguno que no
// exista. Es el contrato que se rompe en silencio: un prefijo nuevo sin
// location deja la API inalcanzable desde el navegador, y una location sin
// prefijo en el backend devuelve la SPA con 200 en vez de un error.
func TestGateway_ExponeExactamenteLosPrefijosPublicos(t *testing.T) {
	exposed := gatewayPrefixes(readRepoFile(t, gatewayTemplatePath))

	privados := map[string]bool{}
	for _, prefix := range gatewayPrivados {
		privados[prefix] = true
	}
	want := make([]string, 0, len(Prefixes))
	for prefix := range Prefixes {
		name := strings.TrimPrefix(prefix, "/")
		if !privados[name] {
			want = append(want, name)
		}
	}
	sort.Strings(want)

	if !slices.Equal(exposed, want) {
		t.Errorf("el gateway publica %v, quiero %v (privados: %v)", exposed, want, gatewayPrivados)
	}
	for _, prefix := range exposed {
		if _, ok := Prefixes["/"+prefix]; !ok {
			t.Errorf("el gateway publica /%s, que ninguna API sirve", prefix)
		}
	}
	for _, prefix := range gatewayPrivados {
		if slices.Contains(exposed, prefix) {
			t.Errorf("/%s está en gatewayPrivados pero el gateway lo publica", prefix)
		}
	}
}

// TestGateway_UpstreamsDefinidos comprueba que cada ${*_UPSTREAM} que usa la
// plantilla tenga valor por defecto en la imagen. Si falta, envsubst deja la
// variable vacía, `proxy_pass http://;` es inválido y nginx no arranca: el
// sitio entero se cae por una location nueva.
func TestGateway_UpstreamsDefinidos(t *testing.T) {
	used := templateUpstreams(readRepoFile(t, gatewayTemplatePath))
	defined := dockerfileUpstreams(readRepoFile(t, gatewayDockerfile))

	if len(used) == 0 {
		t.Fatal("no se encontró ningún ${*_UPSTREAM} en la plantilla")
	}
	for _, name := range used {
		if !slices.Contains(defined, name) {
			t.Errorf("%s se usa en la plantilla pero no tiene valor por defecto en el Dockerfile", name)
		}
	}
	for _, name := range defined {
		if !slices.Contains(used, name) {
			t.Errorf("%s está en el Dockerfile pero la plantilla no la usa", name)
		}
	}
}
