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
	// La limpieza debe correr al final, tras las migraciones de las que depende.
	entries := readMigrationOrder(t)
	index := make(map[string]int, len(entries))
	for i, name := range entries {
		index[name] = i
	}

	const dedupe = "021_dedupe_scraped_products.sql"
	pos, ok := index[dedupe]
	if !ok {
		t.Fatalf("%s no está en migration_order.txt", dedupe)
	}
	if pos != len(entries)-1 {
		t.Errorf("%s debe permanecer al final de migration_order.txt", dedupe)
	}
	for _, dependency := range []string{
		"004_platform_apis.sql",
		"009_product_offer_identity_and_timestamps.sql",
		"010_product_price_history_on_price_change.sql",
		"011_catalog_history_uniqueness_and_indexes.sql",
		"014_product_attributes.sql",
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
