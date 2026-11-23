package database

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TAL-241: el script para bases sin runner (la de pruebas en Neon) copia las
// migraciones 044 y 045. Este test falla si alguien cambia una migración y
// olvida regenerar el script, o si el checksum que registra ya no es el que
// calcula el runner.

const neonScript = "../../database/scripts/TAL-241-neon.sql"

// skuPattern reconoce los SKU del seed entre comillas simples.
var skuPattern = regexp.MustCompile(`'((?:SKU|DEMO|[A-Z]+)-[A-Z0-9-]+)'`)

// readWithoutCR lee un archivo y quita los CR, como hace migrate.sh antes de
// calcular el checksum: así da igual si la copia es Windows (CRLF) o Linux.
func readWithoutCR(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer %s: %v", path, err)
	}
	return strings.ReplaceAll(string(raw), "\r", "")
}

// skuSet devuelve los SKU distintos de un texto, ordenados.
func skuSet(text string) []string {
	seen := map[string]bool{}
	for _, m := range skuPattern.FindAllStringSubmatch(text, -1) {
		seen[m[1]] = true
	}
	out := make([]string, 0, len(seen))
	for sku := range seen {
		out = append(out, sku)
	}
	sort.Strings(out)
	return out
}

func TestNeonScriptEmbedsMigrationsAndRunnerChecksums(t *testing.T) {
	script := readWithoutCR(t, neonScript)
	for _, name := range []string{"044_delete_demo_seed.sql", "045_fix_seed_encoding_cp850.sql"} {
		body := readWithoutCR(t, filepath.Join(migrationsDir, name))
		embedded := "-- >>> " + name + "\n" + strings.TrimRight(body, "\n") + "\n-- <<< " + name + "\n"
		if !strings.Contains(script, embedded) {
			t.Errorf("%s no trae %s sin cambios: regenera el script", neonScript, name)
		}
		sum := sha256.Sum256([]byte(body))
		if !strings.Contains(script, "'"+name+"', '"+hex.EncodeToString(sum[:])+"'") {
			t.Errorf("%s no registra %s con el checksum del runner", neonScript, name)
		}
	}

	// La revisión (parte 1) cuenta los mismos SKU que borra 044.
	review, _, found := strings.Cut(script, `\if :{?aplicar}`)
	if !found {
		t.Fatalf("%s no separa la revisión de la parte que aplica", neonScript)
	}
	migration := readWithoutCR(t, filepath.Join(migrationsDir, "044_delete_demo_seed.sql"))
	start := strings.Index(migration, "skus_demo CONSTANT")
	end := strings.Index(migration, "marcas_demo CONSTANT")
	if start < 0 || end < start {
		t.Fatal("044 no declara skus_demo antes de marcas_demo")
	}
	want := skuSet(migration[start:end])
	if got := skuSet(review); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("la revisión cuenta %d SKU y 044 borra %d:\n%v\n%v", len(got), len(want), got, want)
	}
	if len(want) != 38 {
		t.Errorf("044 lista %d SKU, quiero los 38 del seed", len(want))
	}
}
