package ingestion

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/jackc/pgx/v5"
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
	var storeID, otherStoreID, userID, lgBrandID int32

	// Limpieza registrada antes de crear datos.
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, source)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE sku LIKE $1 OR name LIKE $2`, tag+"%", "%"+tag)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = ANY($1)`, []int32{storeID, otherStoreID})
		_, _ = pool.Exec(ctx, `DELETE FROM brand WHERE name = $1`, brand)
		_, _ = pool.Exec(ctx, `DELETE FROM user_account WHERE id = $1`, userID)
		// Sin productos del seed, EnsureBrand puede crear LG: se borra si quedó
		// sin productos, para no dejar una marca vacía en la base.
		_, _ = pool.Exec(ctx, `DELETE FROM brand b WHERE b.id = $1
			AND NOT EXISTS (SELECT 1 FROM product p WHERE p.brand_id = b.id)`, lgBrandID)
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
	brandID, err := runs.EnsureBrand(ctx, "LG")
	must(err, "buscar marca LG")
	lgBrandID = brandID
	for _, variant := range []string{"lg", "L.G."} {
		variantID, err := runs.EnsureBrand(ctx, variant)
		must(err, "buscar variante de marca "+variant)
		if variantID != brandID {
			t.Fatalf("EnsureBrand(%q) = %d, quiero el id compartido %d", variant, variantID, brandID)
		}
	}
	var storedBrandName string
	must(pool.QueryRow(ctx, `SELECT name FROM brand WHERE id = $1`, brandID).Scan(&storedBrandName),
		"leer nombre original de LG")
	if storedBrandName != "LG" {
		t.Fatalf("EnsureBrand reescribió el nombre de marca: %q", storedBrandName)
	}

	h := NewHandler(runs, NewProcessor(runs, products.NewRepository(pool), pricing.NewRepository(pool), notifier, logger), nil, logger)
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
	// Un producto ya catalogado, de marca LG, creado por el test (TAL-241: el
	// seed ya no trae productos). El nombre termina en el tag para la limpieza.
	var seedPublicID string
	seedModel := strings.ToUpper("GM53-" + tag)
	must(pool.QueryRow(ctx, `
		INSERT INTO product (category_id, category_ref, brand_id, name, model)
		SELECT c.legacy_product_category_id, c.id, $1, $2, $3 FROM categories c WHERE c.slug = 'refrigeradores'
		RETURNING public_id::text`, brandID, "Refri catalogado "+tag, seedModel).Scan(&seedPublicID), "producto catalogado")

	result := items(run.ID,
		map[string]any{"externalId": "e-1", "product": map[string]any{"sku": tag + "-1", "name": "Refri " + tag,
			"brand": brand, "model": "M1", "categorySlug": "refrigeradores", "attributes": map[string]any{"capacidad_litros": 350}},
			"offer": map[string]any{"price": 300000, "productUrl": "https://tienda.example.test/p/1"}},
		map[string]any{"externalId": "e-2", "product": map[string]any{"sku": tag + "-sku-de-la-tienda",
			"name": "lo que diga la tienda", "brand": "lg", "model": strings.ToLower(seedModel)},
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
		t.Fatal("un SKU desconocido con marca + modelo de un producto catalogado debe encontrar ese producto, no duplicarlo")
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
	// El producto catalogado no se modificó (el nombre del ítem se ignora).
	var seedName string
	must(pool.QueryRow(ctx, `SELECT name FROM product WHERE public_id = $1::uuid`, seedPublicID).Scan(&seedName), "seed")
	if seedName == "lo que diga la tienda" {
		t.Fatal("Ingestion no debe modificar productos existentes")
	}

	// 2b. Una hoja sembrada por 040 se ingiere por slug sin atributos.
	hoja := items(run.ID, map[string]any{"externalId": "e-lav",
		"product": map[string]any{"sku": tag + "-lav", "name": "Lavadora " + tag, "brand": brand,
			"categorySlug": "lavadoras"},
		"offer": map[string]any{"price": 450000, "productUrl": "https://tienda.example.test/p/lav"}})
	if hoja.Results[0].Status != ItemCreated {
		t.Fatalf("hoja nueva = %+v", hoja.Results[0])
	}
	var hojaSlug string
	must(pool.QueryRow(ctx, `
		SELECT c.slug FROM product p
		JOIN categories c ON c.id = p.category_ref
		JOIN product_category pc ON pc.id = p.category_id
		WHERE p.sku = $1 AND c.legacy_product_category_id = pc.id`, tag+"-lav").Scan(&hojaSlug), "hoja nueva")
	if hojaSlug != "lavadoras" {
		t.Fatalf("hoja nueva quedó en %q, quiero lavadoras", hojaSlug)
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

	// 3b. TAL-169: un ítem sin SKU ni modelo, como los del scraper. Repetirlo
	// con la misma URL (aunque cambien los parámetros de seguimiento o el
	// fragmento) no duplica el producto ni la oferta.
	sinIdentidad := func(price int, productURL string) map[string]any {
		return map[string]any{"product": map[string]any{"name": "Sin identidad " + tag, "brand": brand,
			"categorySlug": "refrigeradores", "attributes": map[string]any{"capacidad_litros": 262}},
			"offer": map[string]any{"price": price, "productUrl": productURL}}
	}
	first := items(run.ID, sinIdentidad(199990, "https://tienda.example.test/p/sin-identidad?pos=4&catPos=4"))
	again := items(run.ID, sinIdentidad(189990, "https://tienda.example.test/p/sin-identidad?pos=25&catPos=25#opiniones"))
	if first.Results[0].Status != ItemCreated || again.Results[0].Status != ItemUpdated ||
		again.Results[0].ProductID != first.Results[0].ProductID {
		t.Fatalf("misma tienda + URL: primero %+v, repetido %+v", first.Results[0], again.Results[0])
	}
	var copies, offerCount int
	must(pool.QueryRow(ctx, `SELECT count(DISTINCT p.id), count(o.id) FROM product p
		LEFT JOIN product_offer o ON o.product_id = p.id WHERE p.name = $1`, "Sin identidad "+tag).Scan(&copies, &offerCount), "copias")
	if copies != 1 || offerCount != 1 {
		t.Fatalf("productos = %d, ofertas = %d; quiero 1 y 1", copies, offerCount)
	}
	var priceNow string
	var priceRows int
	must(pool.QueryRow(ctx, `SELECT o.price::text, (SELECT count(*) FROM product_price_history h WHERE h.product_offer_id = o.id)
		FROM product_offer o JOIN product p ON p.id = o.product_id WHERE p.name = $1`, "Sin identidad "+tag).Scan(&priceNow, &priceRows), "oferta")
	if priceNow != "189990.00" || priceRows != 2 {
		t.Fatalf("precio = %s, historial = %d; quiero 189990.00 y 2 registros en la misma oferta", priceNow, priceRows)
	}

	// 4. Contadores, auditoría y cierre.
	rec = do(http.MethodGet, fmt.Sprintf("/ingestion/runs/%d", run.ID), nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &run)
	if run.RecordsFound != 8 || run.RecordsProcessed != 6 || run.RecordsFailed != 2 {
		t.Fatalf("contadores = %d/%d/%d", run.RecordsFound, run.RecordsProcessed, run.RecordsFailed)
	}
	var scraped int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM scraped_data WHERE scrape_run_id = $1`, run.ID).Scan(&scraped), "scraped_data")
	if scraped != 8 {
		t.Fatalf("scraped_data = %d, quiero 8", scraped)
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

// Integración TAL-243: un producto existente sin imagen activa toma la imagen
// que envía la tienda, sin duplicarla ni pisar una imagen existente, y sin
// reactivar una imagen que el admin desactivó. Se omite sin TEST_DATABASE_URL.
func TestIngestionIntegration_ImagenDeProductoExistente(t *testing.T) {
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

	tag := "tal243-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	source := "Scraper " + tag
	brand := "Marca " + tag
	name := "Refri sin imagen " + tag
	var storeID int32
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, source)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE name = $1`, name) // product_image cae en cascada
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = $1`, storeID)
		_, _ = pool.Exec(ctx, `DELETE FROM brand WHERE name = $1`, brand)
	})
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID), "tienda")

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	runs := NewRepository(pool)
	h := NewHandler(runs, NewProcessor(runs, products.NewRepository(pool), pricing.NewRepository(pool), &fakeNotifier{}, logger), nil, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs", h.OpenRun)
	r.Post("/ingestion/runs/{id}/items", h.AddItems)
	do := func(path string, body any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(encoded)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	rec := do("/ingestion/runs", map[string]any{
		"source": map[string]any{"name": source, "baseUrl": "https://tienda.example.test"}, "storeId": storeID})
	var run Run
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil || rec.Code != http.StatusCreated {
		t.Fatalf("abrir = %d %s", rec.Code, rec.Body.String())
	}

	// Ítem como los del scraper: sin SKU ni modelo, identificado por tienda + URL.
	send := func(imageURL string) ItemResult {
		t.Helper()
		product := map[string]any{"name": name, "brand": brand, "categorySlug": "refrigeradores",
			"attributes": map[string]any{"capacidad_litros": 300}}
		if imageURL != "" {
			product["imageUrl"] = imageURL
		}
		rec := do(fmt.Sprintf("/ingestion/runs/%d/items", run.ID), map[string]any{"items": []any{map[string]any{
			"product": product, "offer": map[string]any{"price": 299990, "productUrl": "https://tienda.example.test/p/refri?pos=3"}}}})
		var result BatchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusOK || len(result.Results) != 1 {
			t.Fatalf("lote = %d %s", rec.Code, rec.Body.String())
		}
		return result.Results[0]
	}
	// images devuelve las imágenes del producto como "url|activa|orden|alt".
	images := func() []string {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT i.image_url, i.active, i.sort_order, COALESCE(i.alt_text, '')
			FROM product_image i JOIN product p ON p.id = i.product_id WHERE p.name = $1 ORDER BY i.id`, name)
		must(err, "imágenes")
		defer rows.Close()
		out := []string{}
		for rows.Next() {
			var imageURL, alt string
			var active bool
			var order int
			must(rows.Scan(&imageURL, &active, &order, &alt), "imagen")
			out = append(out, fmt.Sprintf("%s|%t|%d|%s", imageURL, active, order, alt))
		}
		return out
	}

	// 1. Se crea sin imagen (como los 18 productos de Falabella).
	if r := send(""); r.Status != ItemCreated || len(images()) != 0 {
		t.Fatalf("crear = %+v, imágenes = %v", r, images())
	}
	// 2. El mismo producto llega con imagen: se guarda una sola vez.
	first := "https://img.example.test/refri.jpg"
	for range 2 {
		if r := send(first); r.Status != ItemUpdated || len(r.Errors) != 0 {
			t.Fatalf("actualizar = %+v", r)
		}
	}
	if got, want := images(), []string{first + "|true|0|" + name}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("imágenes = %v, quiero %v", got, want)
	}
	// 3. Con imagen activa, otra URL no la reemplaza ni se agrega.
	if r := send("https://img.example.test/otra.jpg"); r.Status != ItemUpdated {
		t.Fatalf("otra imagen = %+v", r)
	}
	if got := images(); len(got) != 1 || !strings.HasPrefix(got[0], first+"|true") {
		t.Fatalf("una imagen existente no se cambia: %v", got)
	}
	// 4. El admin la desactiva: la misma URL no se reactiva ni se duplica.
	_, err = pool.Exec(ctx, `UPDATE product_image SET active = FALSE
		WHERE product_id = (SELECT id FROM product WHERE name = $1)`, name)
	must(err, "desactivar imagen")
	send(first)
	if got := images(); len(got) != 1 || !strings.HasPrefix(got[0], first+"|false") {
		t.Fatalf("una imagen desactivada por el admin no vuelve: %v", got)
	}
	// 5. Sin imagen activa, una URL distinta sí se guarda.
	second := "https://img.example.test/nueva.jpg"
	send(second)
	if got := images(); len(got) != 2 || got[1] != second+"|true|0|"+name {
		t.Fatalf("imágenes = %v, quiero la nueva activa", got)
	}
	var copies int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM product WHERE name = $1`, name).Scan(&copies), "copias")
	if copies != 1 {
		t.Fatalf("productos = %d, quiero 1 (sin duplicados)", copies)
	}
}

// Integración TAL-205: SaveScrapedBatch guarda todas las filas del lote con
// un solo INSERT, en el orden recibido y con NULL donde no hay identificador.
// Si una fila no es jsonb válido (\u0000), no guarda ninguna: el procesador
// cae al guardado ítem por ítem. Se omite sin TEST_DATABASE_URL.
func TestIngestionIntegration_ScrapedDataPorLote(t *testing.T) {
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

	tag := "tal205-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	var storeID int32
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, "Scraper "+tag)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = $1`, storeID)
	})
	if err := pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	runID, err := repo.OpenRun(ctx, "Scraper "+tag, nil, storeID, "perf", TargetProduct)
	if err != nil {
		t.Fatal(err)
	}

	rows := []ScrapedItem{
		{ExternalID: strp("a"), ExternalURL: strp("https://t.test/a"), Raw: json.RawMessage(`{"n":1}`)},
		{Raw: json.RawMessage(`{"n":2}`)},
		{ExternalID: strp("c"), Raw: json.RawMessage(`{"n":3}`)},
	}
	if err := repo.SaveScrapedBatch(ctx, runID, rows); err != nil {
		t.Fatalf("guardar lote: %v", err)
	}
	saved := func() []string {
		t.Helper()
		result, err := pool.Query(ctx, `
			SELECT COALESCE(external_id, '-') || '|' || COALESCE(external_url, '-') || '|' || (raw_data->>'n')
			FROM scraped_data WHERE scrape_run_id = $1 ORDER BY id`, runID)
		if err != nil {
			t.Fatal(err)
		}
		out, err := pgx.CollectRows(result, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got, want := saved(), []string{"a|https://t.test/a|1", "-|-|2", "c|-|3"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("scraped_data = %v, quiero %v", got, want)
	}

	bad := []ScrapedItem{{Raw: json.RawMessage(`{"n":4}`)}, {Raw: json.RawMessage(`{"n":"\u0000"}`)}}
	if err := repo.SaveScrapedBatch(ctx, runID, bad); err == nil {
		t.Fatal("un \\u0000 debe hacer fallar el lote")
	}
	if got := saved(); len(got) != 3 {
		t.Fatalf("un lote fallido no debe dejar filas: %v", got)
	}
}

// offersQueFalla envuelve al repositorio real de Pricing y, con fail activo,
// rechaza la oferta después de que el producto ya se insertó en la misma
// transacción: simula un corte entre producto y oferta (TAL-191).
type offersQueFalla struct {
	*pricing.Repository
	fail bool
}

// UpsertOfferTx falla a pedido o delega en el repositorio real dentro de la
// transacción del ítem.
func (o *offersQueFalla) UpsertOfferTx(ctx context.Context, tx pgx.Tx, w pricing.OfferWrite) (int32, bool, *string, error) {
	if o.fail {
		return 0, false, nil, errors.New("corte simulado entre producto y oferta")
	}
	return o.Repository.UpsertOfferTx(ctx, tx, w)
}

// Integración TAL-191: repetir el mismo lote, en la misma ejecución y en una
// nueva, no duplica producto, oferta ni historial. Y si el proceso se corta
// entre el alta del producto y la oferta, la transacción revierte completa: no
// queda un producto huérfano y el reintento lo crea una sola vez. Se omite sin
// TEST_DATABASE_URL.
func TestIngestionIntegration_ReintentoYCorteNoDuplican(t *testing.T) {
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

	tag := "tal191-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	source := "Scraper " + tag
	brand := "Marca " + tag
	name := "Refri reintento " + tag
	cortada := "Refri cortada " + tag
	var storeID int32
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, source)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE name = ANY($1)`, []string{name, cortada})
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = $1`, storeID)
		_, _ = pool.Exec(ctx, `DELETE FROM brand WHERE name = $1`, brand)
	})
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID), "tienda")

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	runs := NewRepository(pool)
	offers := &offersQueFalla{Repository: pricing.NewRepository(pool)}
	h := NewHandler(runs, NewProcessor(runs, products.NewRepository(pool), offers, &fakeNotifier{}, logger), nil, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs", h.OpenRun)
	r.Post("/ingestion/runs/{id}/items", h.AddItems)
	do := func(path string, body any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(encoded)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	open := func() Run {
		t.Helper()
		rec := do("/ingestion/runs", map[string]any{
			"source": map[string]any{"name": source, "baseUrl": "https://tienda.example.test"}, "storeId": storeID})
		var run Run
		if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil || rec.Code != http.StatusCreated {
			t.Fatalf("abrir = %d %s", rec.Code, rec.Body.String())
		}
		return run
	}
	// item arma un ítem como los del scraper: sin SKU ni modelo, identificado
	// por tienda + URL, como los productos que se duplicaban.
	item := func(productName, productURL string) map[string]any {
		return map[string]any{"product": map[string]any{"name": productName, "brand": brand,
			"categorySlug": "refrigeradores", "attributes": map[string]any{"capacidad_litros": 320}},
			"offer": map[string]any{"price": 299990, "productUrl": productURL}}
	}
	send := func(runID int32, items ...map[string]any) BatchResult {
		t.Helper()
		rec := do(fmt.Sprintf("/ingestion/runs/%d/items", runID), map[string]any{"items": items})
		if rec.Code != http.StatusOK {
			t.Fatalf("lote = %d %s", rec.Code, rec.Body.String())
		}
		var result BatchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	// counts devuelve productos, ofertas e historial del producto indicado.
	counts := func(productName string) (int, int, int) {
		t.Helper()
		var p, o, h int
		must(pool.QueryRow(ctx, `SELECT count(DISTINCT p.id), count(DISTINCT o.id), count(DISTINCT h.id)
			FROM product p
			LEFT JOIN product_offer o ON o.product_id = p.id
			LEFT JOIN product_price_history h ON h.product_offer_id = o.id
			WHERE p.name = $1`, productName).Scan(&p, &o, &h), "contar "+productName)
		return p, o, h
	}

	// 1. Doble corrida: el mismo producto dos veces en el mismo lote y luego
	// en una nueva ejecución. Queda un solo producto y una sola oferta.
	first := open()
	result := send(first.ID, item(name, "https://tienda.example.test/p/reintento?pos=1"),
		item(name, "https://tienda.example.test/p/reintento?pos=2"))
	if result.Results[0].Status != ItemCreated || result.Results[1].Status != ItemUpdated ||
		result.Results[0].ProductID != result.Results[1].ProductID {
		t.Fatalf("mismo lote = %+v", result.Results)
	}
	second := open()
	if r := send(second.ID, item(name, "https://tienda.example.test/p/reintento?pos=3")); r.Results[0].Status != ItemUpdated {
		t.Fatalf("segunda corrida = %+v", r.Results[0])
	}
	if p, o, h := counts(name); p != 1 || o != 1 || h != 1 {
		t.Fatalf("tras dos corridas: productos = %d, ofertas = %d, historial = %d; quiero 1/1/1", p, o, h)
	}

	// 2. Corte simulado entre el producto y la oferta: el ítem falla y no deja
	// un producto huérfano. El reintento lo crea una sola vez.
	third := open()
	offers.fail = true
	result = send(third.ID, item(cortada, "https://tienda.example.test/p/cortada"))
	if result.Results[0].Status != ItemFailed || len(result.Results[0].Errors) != 1 ||
		result.Results[0].Errors[0].Code != codeInternalError {
		t.Fatalf("corte = %+v", result.Results[0])
	}
	if p, o, h := counts(cortada); p != 0 || o != 0 || h != 0 {
		t.Fatalf("el corte dejó filas: productos = %d, ofertas = %d, historial = %d", p, o, h)
	}
	offers.fail = false
	if r := send(third.ID, item(cortada, "https://tienda.example.test/p/cortada")); r.Results[0].Status != ItemCreated {
		t.Fatalf("reintento = %+v", r.Results[0])
	}
	if r := send(third.ID, item(cortada, "https://tienda.example.test/p/cortada?pos=2")); r.Results[0].Status != ItemUpdated {
		t.Fatalf("reintento repetido = %+v", r.Results[0])
	}
	if p, o, h := counts(cortada); p != 1 || o != 1 || h != 1 {
		t.Fatalf("tras el reintento: productos = %d, ofertas = %d, historial = %d; quiero 1/1/1", p, o, h)
	}
}

// Integración TAL-191: sin SKU ni modelo, la tienda publica variantes de color
// del mismo producto con otra URL y el mismo nombre; deben quedar como ofertas
// de un solo producto. Dos productos distintos de la misma marca y categoría
// tienen nombres distintos y no se unen. Se omite sin TEST_DATABASE_URL.
func TestIngestionIntegration_VariantesDeColorYFalsosPositivos(t *testing.T) {
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

	tag := "tal191v-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	source := "Scraper " + tag
	brand := "Marca color " + tag
	baseName := "Refri Color " + tag
	otherName := "Refri Distinto " + tag
	var storeID int32
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, source)
		_, _ = pool.Exec(ctx, `DELETE FROM product WHERE name ILIKE $1`, "%"+tag)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = $1`, storeID)
		_, _ = pool.Exec(ctx, `DELETE FROM brand WHERE name = $1`, brand)
	})
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	must(pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID), "tienda")

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	runs := NewRepository(pool)
	h := NewHandler(runs, NewProcessor(runs, products.NewRepository(pool), pricing.NewRepository(pool), &fakeNotifier{}, logger), nil, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs", h.OpenRun)
	r.Post("/ingestion/runs/{id}/items", h.AddItems)
	do := func(path string, body any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(encoded)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	rec := do("/ingestion/runs", map[string]any{
		"source": map[string]any{"name": source, "baseUrl": "https://tienda.example.test"}, "storeId": storeID})
	var run Run
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil || rec.Code != http.StatusCreated {
		t.Fatalf("abrir = %d %s", rec.Code, rec.Body.String())
	}
	send := func(productName, productURL string) ItemResult {
		t.Helper()
		rec := do(fmt.Sprintf("/ingestion/runs/%d/items", run.ID), map[string]any{"items": []any{map[string]any{
			"product": map[string]any{"name": productName, "brand": brand, "categorySlug": "refrigeradores",
				"attributes": map[string]any{"capacidad_litros": 340}},
			"offer": map[string]any{"price": 349990, "productUrl": productURL}}}})
		var result BatchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusOK || len(result.Results) != 1 {
			t.Fatalf("lote = %d %s", rec.Code, rec.Body.String())
		}
		return result.Results[0]
	}
	offers := func(publicID string) int {
		t.Helper()
		var n int
		must(pool.QueryRow(ctx, `SELECT count(*) FROM product_offer o
			JOIN product p ON p.id = o.product_id WHERE p.public_id = $1::uuid`, publicID).Scan(&n), "ofertas")
		return n
	}

	// 1. Dos variantes de color: mismo nombre (con mayúsculas y espacios de más)
	// y URL distinta. Quedan como dos ofertas del mismo producto.
	first := send(baseName, "https://tienda.example.test/p/color-blanco")
	second := send("  "+strings.ToUpper("refri color "+tag)+"  ", "https://tienda.example.test/p/color-negro")
	if first.Status != ItemCreated || second.Status != ItemUpdated || second.ProductID != first.ProductID {
		t.Fatalf("variantes = %+v / %+v", first, second)
	}
	if got := offers(first.ProductID); got != 2 {
		t.Fatalf("ofertas de la variante = %d, quiero 2", got)
	}

	// 2. Falso positivo: otro producto de la misma marca y categoría, con otro
	// nombre, no se une al de las variantes.
	other := send(otherName, "https://tienda.example.test/p/distinto")
	if other.Status != ItemCreated || other.ProductID == first.ProductID {
		t.Fatalf("producto distinto = %+v", other)
	}
	var total int
	must(pool.QueryRow(ctx, `SELECT count(*) FROM product p
		JOIN brand b ON b.id = p.brand_id
		JOIN categories c ON c.id = p.category_ref
		WHERE lower(b.name) = lower($1) AND c.slug = 'refrigeradores'`, brand).Scan(&total), "productos")
	if total != 2 {
		t.Fatalf("productos de la marca = %d, quiero 2 (variantes juntas, distinto aparte)", total)
	}
}

// Integración TAL-250: una corrida que el scraper cierra como success pero en
// la que ingestion rechazó todos los ítems queda failed en scrape_run, con el
// motivo en error_message, y no entra en la mediana de RecentFound. Una con
// algún ítem procesado se cierra como success. Se omite sin TEST_DATABASE_URL.
func TestIngestionIntegration_EstadoRealDeLaCorrida(t *testing.T) {
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

	tag := "tal250-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	var storeID int32
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, "Scraper "+tag)
		_, _ = pool.Exec(ctx, `DELETE FROM store WHERE id = $1`, storeID)
	})
	if err := pool.QueryRow(ctx, `INSERT INTO store (name) VALUES ($1) RETURNING id`, "Tienda "+tag).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs/{id}/finish", NewHandler(repo, nil, nil, logger).FinishRun)

	tests := []struct {
		name       string
		processed  int
		failed     int
		wantStatus string
		wantMsg    string
	}{
		{"todo rechazado", 0, 26, StatusFailed, "Ingestion rechazó los 26 ítems de la corrida."},
		{"algo procesado", 20, 6, StatusSuccess, ""},
	}
	var ids []int32
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := repo.OpenRun(ctx, "Scraper "+tag, nil, storeID, "http", TargetProduct)
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
			if err := repo.AddCounters(ctx, id, tc.processed+tc.failed, tc.processed, tc.failed); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/finish", id),
				strings.NewReader(`{"status":"success"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("finish = %d %s", rec.Code, rec.Body.String())
			}
			var status string
			var msg *string
			if err := pool.QueryRow(ctx, `SELECT status, error_message FROM scrape_run WHERE id = $1`, id).
				Scan(&status, &msg); err != nil {
				t.Fatal(err)
			}
			got := ""
			if msg != nil {
				got = *msg
			}
			if status != tc.wantStatus || got != tc.wantMsg {
				t.Fatalf("scrape_run = (%q, %q), quiero (%q, %q)", status, got, tc.wantStatus, tc.wantMsg)
			}
		})
	}

	found, err := repo.RecentFound(ctx, storeID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(found) != "[26]" {
		t.Fatalf("RecentFound = %v; solo la corrida exitosa (26 encontrados) cuenta para la mediana", found)
	}
}

// TestIngestionIntegration_Servicios prueba el camino de servicios de punta a
// punta con PostgreSQL real: corrida sin tienda, alta del proveedor y del
// servicio, atributos unificados, identidad por externalId y historial de
// precios al cambiar el monto (issue #191). Se omite sin TEST_DATABASE_URL.
func TestIngestionIntegration_Servicios(t *testing.T) {
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

	tag := "tal191-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	source := "Proveedor " + tag
	providerSlug := "proveedor-" + tag
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	runs := NewRepository(pool)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM data_source WHERE name = $1`, source)
		_, _ = pool.Exec(ctx, `DELETE FROM provider WHERE slug = $1`, providerSlug)
	})

	processor := NewProcessor(runs, products.NewRepository(pool), pricing.NewRepository(pool), nil, logger)
	h := NewHandler(runs, processor, nil, logger)
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Post("/ingestion/runs", h.OpenRun)
	r.Post("/ingestion/runs/{id}/services", h.AddServices)

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, strings.NewReader(string(encoded)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	rec := do(http.MethodPost, "/ingestion/runs", map[string]any{
		"source":     map[string]any{"name": source, "baseUrl": "https://proveedor.example.test"},
		"targetType": TargetService,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("abrir corrida = %d %s", rec.Code, rec.Body.String())
	}
	var run Run
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.TargetType != TargetService || run.StoreID != nil {
		t.Fatalf("corrida = %+v", run)
	}

	item := func(price int) map[string]any {
		return map[string]any{
			"externalId":  "plan-600",
			"externalUrl": "https://proveedor.example.test/planes",
			"provider":    map[string]any{"slug": providerSlug, "name": "Proveedor"},
			"service": map[string]any{
				"name":         "Fibra 600",
				"categorySlug": "internet-fibra-optica",
				"attributes": map[string]any{
					"velocidad_bajada_mbps": 600,
					"velocidad_subida_mbps": 600,
				},
			},
			"offer": map[string]any{"price": price, "billingPeriod": "monthly"},
		}
	}
	send := func(price int) BatchResult {
		t.Helper()
		rec := do(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/services", run.ID), map[string]any{"items": []any{item(price)}})
		if rec.Code != http.StatusOK {
			t.Fatalf("lote = %d %s", rec.Code, rec.Body.String())
		}
		var result BatchResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Processed != 1 || result.Results[0].ServiceID == "" {
			t.Fatalf("resultado = %+v", result)
		}
		return result
	}

	first := send(29990)
	if first.Results[0].Status != ItemCreated {
		t.Fatalf("primer ítem = %+v", first.Results[0])
	}

	var (
		serviceID  int32
		categoryID int32
		offerID    int32
	)
	if err := pool.QueryRow(ctx, `
		SELECT s.id, s.category_ref, o.id
		FROM service s
		JOIN service_offer o ON o.service_id = s.id
		JOIN provider p ON p.id = o.provider_id
		WHERE p.slug = $1 AND o.external_id = 'plan-600'`,
		providerSlug).Scan(&serviceID, &categoryID, &offerID); err != nil {
		t.Fatal(err)
	}
	if categoryID == 0 {
		t.Fatal("el servicio quedó sin category_ref")
	}
	var attributeCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM service_attribute_value WHERE service_id = $1`, serviceID).
		Scan(&attributeCount); err != nil {
		t.Fatal(err)
	}
	if attributeCount != 2 {
		t.Fatalf("atributos = %d, quiero 2", attributeCount)
	}

	second := send(27990)
	if second.Results[0].Status != ItemUpdated || second.Results[0].ServiceID != first.Results[0].ServiceID {
		t.Fatalf("segundo ítem = %+v", second.Results[0])
	}
	var history int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM service_price_history WHERE service_offer_id = $1`, offerID).
		Scan(&history); err != nil {
		t.Fatal(err)
	}
	if history != 2 {
		t.Fatalf("historial = %d, quiero 2", history)
	}
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM scraped_data WHERE scrape_run_id = $1 AND entity_type = 'service'`, run.ID).
		Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 2 {
		t.Fatalf("filas de scraped_data = %d, quiero 2", stored)
	}

	// Una corrida de producto no acepta el endpoint de servicios.
	productRec := do(http.MethodPost, "/ingestion/runs", map[string]any{
		"source": map[string]any{"name": source + " producto", "baseUrl": "https://tienda.example.test"}, "storeId": 1,
	})
	if productRec.Code != http.StatusCreated {
		t.Fatalf("abrir corrida de producto = %d %s", productRec.Code, productRec.Body.String())
	}
	var productRun Run
	_ = json.Unmarshal(productRec.Body.Bytes(), &productRun)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM data_source WHERE name = $1`, source+" producto")
	})
	rejected := do(http.MethodPost, fmt.Sprintf("/ingestion/runs/%d/services", productRun.ID), map[string]any{"items": []any{item(27990)}})
	if rejected.Code != http.StatusConflict {
		t.Fatalf("servicio en corrida de producto = %d %s", rejected.Code, rejected.Body.String())
	}
}
