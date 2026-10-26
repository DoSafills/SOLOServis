package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/pricing"
	"github.com/DoSafills/SOLOServis/backend/internal/products"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Integración TAL-77 contra PostgreSQL con todas las migraciones (incluida la
// 015). Usa los repositorios reales de Ingestion, Product y Pricing; solo el
// envío de avisos es falso. Se omite sin TEST_DATABASE_URL.
func TestIngestionIntegration_FlujoCompleto(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no definida: se omite el test de integración")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	tag := "tal77-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	source := "Scraper " + tag
	brand := "Marca " + tag
	var storeID, otherStoreID, userID int32

	// Limpieza registrada antes de crear datos.
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, source)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE sku LIKE $1`, tag+"%")
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = ANY($1)`, []int32{storeID, otherStoreID})
		_, _ = pool.Exec(ctx, `DELETE FROM brand WHERE name = $1`, brand)
		_, _ = pool.Exec(ctx, `DELETE FROM user_account WHERE id = $1`, userID)
	})
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID), "tienda")
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Otra "+tag).Scan(&otherStoreID), "otra tienda")
	must(pool.QueryRow(ctx, `INSERT INTO user_account (name, email, password_hash) VALUES ($1, $2, 'x') RETURNING id`,
		"Watcher "+tag, tag+"@example.test").Scan(&userID), "usuario")

	notifier := &fakeNotifier{}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	runs := NewRepository(pool)
	h := NewHandler(runs, NewProcessor(runs, products.NewRepository(pool), pricing.NewRepository(pool), notifier, logger), logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs", h.OpenRun)
	r.Get("/ingestion/runs/{id}", h.GetRun)
	r.Post("/ingestion/runs/{id}/items", h.AddItems)
	r.Post("/ingestion/runs/{id}/finish", h.FinishRun)

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, strings.NewReader(string(encoded)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	open := func(store int32) (*httptest.ResponseRecorder, Run) {
		rec := do(http.MethodPost, "/ingestion/runs", map[string]any{
			"source": map[string]any{"name": source, "baseUrl": "https://tienda.example.test"}, "storeId": store})
		var run Run
		_ = json.Unmarshal(rec.Body.Bytes(), &run)
		return rec, run
	}
	items := func(runID int32, list ...map[string]any) BatchResult {
		t.Helper()
		rec := do(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/items", runID), map[string]any{"items": list})
		if rec.Code != http.StatusOK {
			t.Fatalf("lote = %d %s", rec.Code, rec.Body.String())
		}
		var result BatchResult
		_ = json.Unmarshal(rec.Body.Bytes(), &result)
		return result
	}

	// 1. Abrir la ejecución: crea la fuente enlazada a la tienda.
	rec, run := open(storeID)
	if rec.Code != http.StatusCreated || run.Status != StatusRunning || *run.StoreID != storeID || run.Source != source {
		t.Fatalf("abrir = %d %+v", rec.Code, run)
	}

	// 2. Lote mixto.
	var seedPublicID string
	must(pool.QueryRow(ctx, `SELECT p.public_id::text FROM product p JOIN brand b ON b.id = p.brand_id
		WHERE b.name = 'LG' AND p.model = 'GM53'`).Scan(&seedPublicID), "producto del seed")

	result := items(run.ID,
		map[string]any{"externalId": "e-1", "product": map[string]any{"sku": tag + "-1", "name": "Refri " + tag,
			"brand": brand, "model": "M1", "categorySlug": "refrigeradores", "attributes": map[string]any{"capacidad_litros": 350}},
			"offer": map[string]any{"price": 300000, "productUrl": "https://tienda.example.test/p/1"}},
		map[string]any{"externalId": "e-2", "product": map[string]any{"name": "lo que diga la tienda", "brand": "lg", "model": "gm53"},
			"offer": map[string]any{"price": 888888}},
		map[string]any{"externalId": "e-3", "product": map[string]any{"sku": tag + "-3", "name": "Sin atributo",
			"categorySlug": "refrigeradores"}, "offer": map[string]any{"price": 1}},
		map[string]any{"externalId": "e-4", "product": map[string]any{"name": "Precio malo"}, "offer": map[string]any{"price": -5}},
	)
	statuses := []string{}
	for _, item := range result.Results {
		statuses = append(statuses, item.Status+":"+fmt.Sprint(codes(item)))
	}
	want := []string{"created:[]", "updated:[]", "failed:[product.attributes.capacidad_litros/required]", "failed:[offer.price/min]"}
	if fmt.Sprint(statuses) != fmt.Sprint(want) {
		t.Fatalf("resultados = %v, quiero %v", statuses, want)
	}
	if result.Results[1].ProductID != seedPublicID {
		t.Fatal("marca + modelo (sin distinguir mayúsculas) debe encontrar el producto del seed")
	}

	// El producto nuevo quedó con categoría, atributos y la marca creada.
	var categorySlug, brandName, capacity string
	must(pool.QueryRow(ctx, `
		SELECT c.slug, b.name, v.value FROM product p
		JOIN categories c ON c.id = p.category_ref JOIN brand b ON b.id = p.brand_id
		JOIN product_attribute_value v ON v.product_id = p.id
		WHERE p.sku = $1`, tag+"-1").Scan(&categorySlug, &brandName, &capacity), "producto creado")
	if categorySlug != "refrigeradores" || brandName != brand || capacity != "350" {
		t.Fatalf("producto creado = %s %s %s", categorySlug, brandName, capacity)
	}
	// El producto del seed no se modificó (el nombre del ítem se ignora).
	var seedName string
	must(pool.QueryRow(ctx, `SELECT name FROM product WHERE public_id = $1::uuid`, seedPublicID).Scan(&seedName), "seed")
	if seedName == "lo que diga la tienda" {
		t.Fatal("Ingestion no debe modificar productos existentes")
	}

	// 3. Segundo lote: bajada de precio del producto creado, con aviso.
	var productID int32
	must(pool.QueryRow(ctx, `SELECT id FROM product WHERE sku = $1`, tag+"-1").Scan(&productID), "id")
	_, err = pool.Exec(ctx, `INSERT INTO product_user_interest (user_id, product_id, target_price, notify_price_drop)
		VALUES ($1, $2, 280000, TRUE)`, userID, productID)
	must(err, "interés")
	result = items(run.ID, map[string]any{"externalId": "e-1", "product": map[string]any{"sku": tag + "-1", "name": "x"},
		"offer": map[string]any{"price": 250000, "productUrl": "https://tienda.example.test/p/1"}})
	if result.Results[0].Status != ItemUpdated || len(notifier.sent) != 1 || notifier.sent[0].UserID != userID {
		t.Fatalf("bajada = %+v avisos = %+v", result.Results[0], notifier.sent)
	}
	var history []string
	rows, err := pool.Query(ctx, `SELECT h.price::text FROM product_price_history h
		JOIN product_offer o ON o.id = h.product_offer_id
		WHERE o.product_id = $1 AND o.store_id = $2 ORDER BY h.recorded_at`, productID, storeID)
	must(err, "historial")
	for rows.Next() {
		var p string
		_ = rows.Scan(&p)
		history = append(history, p)
	}
	rows.Close()
	if fmt.Sprint(history) != "[300000.00 250000.00]" {
		t.Fatalf("historial = %v (una sola oferta: misma URL)", history)
	}

	// 4. Contadores, auditoría y cierre.
	rec = do(http.MethodGet, fmt.Sprintf("/ingestion/runs/%d", run.ID), nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &run)
	if run.RecordsFound != 5 || run.RecordsProcessed != 3 || run.RecordsFailed != 2 {
		t.Fatalf("contadores = %d/%d/%d", run.RecordsFound, run.RecordsProcessed, run.RecordsFailed)
	}
	var scraped int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM scraped_data WHERE scrape_run_id = $1`, run.ID).Scan(&scraped), "scraped_data")
	if scraped != 5 {
		t.Fatalf("scraped_data = %d, quiero 5", scraped)
	}

	if rec := do(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/finish", run.ID), map[string]any{"status": "success"}); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"status":"success"`) || strings.Contains(rec.Body.String(), `"finishedAt":null`) {
		t.Fatalf("cerrar = %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/items", run.ID), map[string]any{"items": []any{
		map[string]any{"product": map[string]any{"name": "x"}, "offer": map[string]any{"price": 1}}}}); rec.Code != http.StatusConflict {
		t.Fatalf("ítems en una ejecución cerrada = %d", rec.Code)
	}
	if rec := do(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/finish", run.ID), map[string]any{"status": "failed"}); rec.Code != http.StatusConflict {
		t.Fatalf("cerrar dos veces = %d", rec.Code)
	}

	// 5. Reabrir: misma tienda reutiliza fuente y configuración; otra tienda, 409.
	if rec, _ := open(otherStoreID); rec.Code != http.StatusConflict {
		t.Fatalf("fuente con otra tienda = %d", rec.Code)
	}
	if rec, second := open(storeID); rec.Code != http.StatusCreated || second.ID == run.ID {
		t.Fatalf("segunda ejecución = %d %+v", rec.Code, second)
	}
	var configs int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM scraper_config sc JOIN data_source ds ON ds.id = sc.data_source_id
		WHERE ds.name = $1`, source).Scan(&configs), "configs")
	if configs != 1 {
		t.Fatalf("configuraciones = %d, quiero 1 reutilizada", configs)
	}
}
