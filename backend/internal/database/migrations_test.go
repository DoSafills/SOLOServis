package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TAL-174: el orden de las migraciones es explícito en migration_order.txt.
// Un archivo que no está ahí simplemente nunca se aplica —pasó con
// 021_dedupe_scraped_products.sql, cuya limpieza de duplicados quedó sin
// ejecutar durante días—. Este test convierte ese olvido en un fallo de CI.

const (
	migrationsDir  = "../../database/migrations"
	migrationsList = "../../database/migration_order.txt"
)

func readMigrationOrder(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(migrationsList)
	if err != nil {
		t.Fatalf("leer %s: %v", migrationsList, err)
	}
	var entries []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	return entries
}

func TestMigrationOrderListsEveryFileOnce(t *testing.T) {
	entries := readMigrationOrder(t)
	if len(entries) == 0 {
		t.Fatal("migration_order.txt está vacío")
	}

	listed := make(map[string]bool, len(entries))
	for _, name := range entries {
		if listed[name] {
			t.Errorf("%s aparece dos veces en migration_order.txt", name)
			continue
		}
		listed[name] = true
		if _, err := os.Stat(filepath.Join(migrationsDir, name)); err != nil {
			t.Errorf("migration_order.txt lista %s, que no existe en %s", name, migrationsDir)
		}
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("leer %s: %v", migrationsDir, err)
	}
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		if !listed[name] {
			t.Errorf("%s existe en %s pero no está en migration_order.txt: nunca se aplicaría",
				name, migrationsDir)
		}
	}
}

func TestDedupeCleanupRunsAfterTheSchemaItNeeds(t *testing.T) {
	// La limpieza usa normalize_product_offer_url (009) y product_offer ya
	// consolidada (018). Debe ir después de ambas, sin exigir ser la última:
	// migraciones futuras pueden agregarse detrás.
	index := make(map[string]int)
	for i, name := range readMigrationOrder(t) {
		index[name] = i
	}

	const dedupe = "021_dedupe_scraped_products.sql"
	pos, ok := index[dedupe]
	if !ok {
		t.Fatalf("%s no está en migration_order.txt", dedupe)
	}
	for _, dependency := range []string{
		"009_product_offer_identity_and_timestamps.sql",
		"018_category_artworks.sql",
	} {
		depPos, ok := index[dependency]
		if !ok {
			t.Fatalf("falta la dependencia %s en migration_order.txt", dependency)
		}
		if pos < depPos {
			t.Errorf("%s debe aplicarse después de %s", dedupe, dependency)
		}
	}
}
