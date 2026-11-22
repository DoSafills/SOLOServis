// Command ingestion-import carga en PostgreSQL los archivos que el scraper
// deja en su catálogo local (out/catalog/<tienda>/*.json). No escribe la base
// por su cuenta: envía cada ítem al mismo Processor que usa Ingestion API
// (TAL-77), para que las reglas de productos, ofertas e imágenes sean las
// mismas que en la carga por HTTP.
//
// Uso, desde backend/:
//
//	go run ./cmd/ingestion-import -dir ../scraper/out/catalog/ripley-cl -store-id 4
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/config"
	"github.com/DoSafills/SOLOServis/backend/internal/database"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/ingestion"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
)

const scraperType = "file"

// catalogFile es el documento que el scraper escribe por producto. Sus campos
// product y offer tienen la misma forma que ingestion.Item; source y
// provenance se usan solo para ubicar la tienda y la trazabilidad.
type catalogFile struct {
	ExternalID  string                `json:"externalId"`
	ExternalURL string                `json:"externalUrl"`
	Product     ingestion.ItemProduct `json:"product"`
	Offer       pricing.OfferFields   `json:"offer"`
	Source      catalogSource         `json:"source"`
}

// catalogSource identifica la tienda según el archivo. Su storeId es el de la
// base SQLite del scraper y no coincide con PostgreSQL, por eso se ignora.
type catalogSource struct {
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
}

// pendingItem es un archivo ya convertido en ítem, con su nombre para poder
// informar los fallos por archivo.
type pendingItem struct {
	file string
	url  string
	item ingestion.Item
}

// summary acumula el resultado de la importación para el reporte final.
type summary struct {
	files   int
	created int
	updated int
	omitted int
	failed  int
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run ejecuta la importación completa y devuelve el código de salida: 0 si el
// lote terminó (aunque haya ítems fallidos) y 1 si no se pudo ni empezar.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("ingestion-import", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "directorio del catálogo de una tienda (obligatorio)")
	storeID := flags.Int("store-id", 0, "id de la tienda en PostgreSQL (obligatorio)")
	source := flags.String("source", "", "nombre de la fuente; por defecto, el del archivo")
	baseURL := flags.String("base-url", "", "URL base de la tienda; por defecto, la del archivo")
	batchSize := flags.Int("batch", ingestion.MaxBatchItems, "ítems por lote (1..100)")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if *dir == "" || *storeID <= 0 || *batchSize < 1 || *batchSize > ingestion.MaxBatchItems {
		fmt.Fprintln(stderr, "-dir y -store-id son obligatorios; -batch debe estar entre 1 y 100")
		flags.Usage()
		return 1
	}
	if info, err := os.Stat(*dir); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "el directorio %q no existe o no es una carpeta\n", *dir)
		return 1
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := config.Load()
	db, err := database.NewPostgresPool(cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(stderr, "no se pudo conectar a PostgreSQL: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.Ping(ctx, db); err != nil {
		fmt.Fprintf(stderr, "PostgreSQL no responde: %v\n", err)
		return 1
	}

	paths, err := catalogFiles(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "no se pudo leer %q: %v\n", *dir, err)
		return 1
	}

	result := summary{files: len(paths)}
	items, sourceName, sourceURL := readCatalog(paths, stdout, &result)
	if *source != "" {
		sourceName = *source
	}
	if *baseURL != "" {
		sourceURL = *baseURL
	}
	if len(items) > 0 {
		if sourceName == "" {
			fmt.Fprintln(stderr, "los archivos no traen source.name; indícalo con -source")
			return 1
		}
		items, result.omitted = dropDuplicates(items, stdout)
	}

	if len(items) > 0 {
		runs := ingestion.NewRepository(db)
		processor := ingestion.NewProcessor(runs, products.NewRepository(db), pricing.NewRepository(db),
			pricing.NewHTTPNotifier(), logger)
		runID, err := runs.OpenRun(ctx, sourceName, optional(sourceURL), int32(*storeID), scraperType)
		if err != nil {
			fmt.Fprintf(stderr, "no se pudo abrir la ejecución: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "ejecución %d: %s (%s)\n", runID, sourceName, *dir)

		processBatches(ctx, processor, runID, int32(*storeID), sourceName, items, *batchSize, stdout, &result)
		if err := runs.FinishRun(ctx, runID, ingestion.StatusSuccess, nil); err != nil {
			fmt.Fprintf(stderr, "la ejecución %d quedó abierta: %v\n", runID, err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "creados=%d actualizados=%d omitidos=%d fallidos=%d archivos=%d\n",
		result.created, result.updated, result.omitted, result.failed, result.files)
	return 0
}

// catalogFiles devuelve los archivos de producto del directorio, en orden
// estable. Omite el manifiesto (_index.json), el historial (_changes.jsonl) y
// cualquier archivo que no sea .json.
func catalogFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, "_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	sort.Strings(paths)
	return paths, nil
}

// readCatalog convierte cada archivo en un ítem. Un archivo ilegible o con
// JSON inválido se reporta como fallido y no detiene el resto. Devuelve los
// ítems y la fuente (nombre y URL base) del primer archivo que la traiga.
func readCatalog(paths []string, stdout io.Writer, result *summary) ([]pendingItem, string, string) {
	items := make([]pendingItem, 0, len(paths))
	var sourceName, sourceURL string
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			result.failed++
			fmt.Fprintf(stdout, "fallido %s: %v\n", filepath.Base(path), err)
			continue
		}
		var file catalogFile
		if err := json.Unmarshal(raw, &file); err != nil {
			result.failed++
			fmt.Fprintf(stdout, "fallido %s: JSON inválido: %v\n", filepath.Base(path), err)
			continue
		}
		if sourceName == "" && strings.TrimSpace(file.Source.Name) != "" {
			sourceName, sourceURL = strings.TrimSpace(file.Source.Name), strings.TrimSpace(file.Source.BaseURL)
		}
		items = append(items, pendingItem{
			file: filepath.Base(path),
			url:  strings.TrimSpace(file.ExternalURL),
			item: ingestion.Item{
				ExternalID: strings.TrimSpace(file.ExternalID),
				Product:    file.Product,
				Offer:      file.Offer,
			},
		})
	}
	return items, sourceName, sourceURL
}

// dropDuplicates quita los ítems que repiten la identidad de otro archivo del
// mismo lote, para no procesar dos veces el mismo producto en una corrida.
// Devuelve los ítems conservados y cuántos se omitieron.
func dropDuplicates(items []pendingItem, stdout io.Writer) ([]pendingItem, int) {
	omitted := 0
	seen := make(map[string]string, len(items))
	kept := make([]pendingItem, 0, len(items))
	for _, pending := range items {
		key := identity(pending)
		if key == "" {
			kept = append(kept, pending)
			continue
		}
		if first, exists := seen[key]; exists {
			omitted++
			fmt.Fprintf(stdout, "omitido %s: duplica a %s\n", pending.file, first)
			continue
		}
		seen[key] = pending.file
		kept = append(kept, pending)
	}
	return kept, omitted
}

// identity devuelve la clave con la que se reconoce un ítem repetido: su
// externalId o, si no viene, la URL de la oferta completa. No se quita la
// query: color y otras variantes de la misma ficha son ofertas distintas,
// igual que en el índice único de product_offer.
func identity(pending pendingItem) string {
	if id := strings.TrimSpace(pending.item.ExternalID); id != "" {
		return "id:" + id
	}
	raw := strings.TrimSpace(pending.item.Offer.ProductURL)
	if raw == "" {
		raw = pending.url
	}
	if raw == "" {
		return ""
	}
	if cut := strings.Index(raw, "#"); cut >= 0 {
		raw = raw[:cut]
	}
	return "url:" + raw
}

// processBatches envía los ítems al Processor en lotes de tamaño fijo,
// acumula creados/actualizados/fallidos y escribe un renglón por ítem fallido.
func processBatches(
	ctx context.Context,
	processor *ingestion.Processor,
	runID, storeID int32,
	sourceName string,
	items []pendingItem,
	batchSize int,
	stdout io.Writer,
	result *summary,
) {
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := make([]ingestion.Item, 0, end-start)
		for _, pending := range items[start:end] {
			batch = append(batch, pending.item)
		}
		batchResult := processor.Process(ctx, runID, storeID, sourceName, batch)
		for index, itemResult := range batchResult.Results {
			switch itemResult.Status {
			case ingestion.ItemCreated:
				result.created++
			case ingestion.ItemUpdated:
				result.updated++
			default:
				result.failed++
				file := items[start+index].file
				fmt.Fprintf(stdout, "fallido %s: %s\n", file, describeErrors(itemResult.Errors))
			}
		}
	}
}

// describeErrors resume los errores de un ítem en un solo renglón.
func describeErrors(errs []httpx.FieldError) string {
	if len(errs) == 0 {
		return "error interno (ver el log del servicio)"
	}
	parts := make([]string, 0, len(errs))
	for _, fieldError := range errs {
		if fieldError.Field == "" {
			parts = append(parts, fieldError.Message)
			continue
		}
		parts = append(parts, fieldError.Field+": "+fieldError.Message)
	}
	return strings.Join(parts, "; ")
}

// optional devuelve la dirección del texto solo si no está vacío, para pasar
// baseUrl como NULL cuando el archivo no la trae.
func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
