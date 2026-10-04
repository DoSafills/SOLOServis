package products

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	repository *Repository
}

var errProductNotFound = errors.New("product not found")

func NewHandler(repository *Repository) *Handler {
	return &Handler{
		repository: repository,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateProductRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid product payload", http.StatusBadRequest)
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	request.ImageURL = strings.TrimSpace(request.ImageURL)

	if request.CategoryID <= 0 || request.Name == "" || !isHTTPURL(request.ImageURL) {
		http.Error(w, "categoryId, name and a valid http(s) imageUrl are required", http.StatusBadRequest)
		return
	}

	product, err := h.repository.Create(r.Context(), request)
	if err != nil {
		http.Error(w, "failed to create product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(product)
}

func isHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)

	return err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != ""
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	listed, err := h.listProducts(r)
	if err != nil {
		if errors.Is(err, errInvalidCategory) {
			http.Error(w, "invalid category id", http.StatusBadRequest)
			return
		}

		http.Error(w, "failed to list products", http.StatusInternalServerError)
		return
	}

	products := make([]dto.ProductListItem, 0, len(listed))

	for _, entry := range listed {
		product := entry.item

		images, err := h.repository.ListImages(r.Context(), entry.id)
		if err != nil {
			http.Error(w, "failed to load product images", http.StatusInternalServerError)
			return
		}

		for _, image := range images {
			product.Images = append(product.Images, dto.FromProductImage(image))
		}

		offers, err := h.repository.ListOffers(r.Context(), entry.id)
		if err != nil {
			http.Error(w, "failed to load product offers", http.StatusInternalServerError)
			return
		}

		for _, offer := range offers {
			product.Offers = append(product.Offers, dto.FromProductOffer(offer))
		}

		specifications, err := h.repository.ListSpecifications(r.Context(), row.ID)
		if err != nil {
			http.Error(w, "failed to load product specifications", http.StatusInternalServerError)
			return
		}

		for _, specification := range specifications {
			product.Specs[specification.Name] = specification.Value
		}

		products = append(products, product)
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(products); err != nil {
		return
	}
}

var errInvalidCategory = errors.New("invalid category id")

// listedProduct conserva el id interno del producto para poder cargar sus
// imágenes y ofertas sin exponerlo en la respuesta.
type listedProduct struct {
	id   int32
	item dto.ProductListItem
}

// listProducts devuelve el catálogo completo o, si viene ?category=<id>,
// los productos de esa categoría y de todas sus subcategorías.
func (h *Handler) listProducts(r *http.Request) ([]listedProduct, error) {
	category := r.URL.Query().Get("category")

	if category == "" {
		rows, err := h.repository.List(r.Context())
		if err != nil {
			return nil, err
		}

		products := make([]listedProduct, 0, len(rows))
		for _, row := range rows {
			products = append(products, listedProduct{id: row.ID, item: dto.FromListProduct(row)})
		}

		return products, nil
	}

	categoryID, err := strconv.ParseInt(category, 10, 32)
	if err != nil || categoryID <= 0 {
		return nil, errInvalidCategory
	}

	rows, err := h.repository.ListByCategory(r.Context(), int32(categoryID))
	if err != nil {
		return nil, err
	}

	products := make([]listedProduct, 0, len(rows))
	for _, row := range rows {
		products = append(products, listedProduct{id: row.ID, item: dto.FromListProductByCategory(row)})
	}

	return products, nil
}

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.ListCategories(r.Context())
	if err != nil {
		http.Error(w, "failed to list categories", http.StatusInternalServerError)
		return
	}

	categories := make([]dto.ProductCategory, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, dto.FromProductCategory(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(categories); err != nil {
		return
	}
}

func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID

	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	product, err := h.repository.GetByPublicID(r.Context(), uuid)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	rows, err := h.repository.ListReviews(r.Context(), product.ID)
	if err != nil {
		http.Error(w, "failed to load product reviews", http.StatusInternalServerError)
		return
	}

	reviews := make([]dto.ProductReview, 0, len(rows))
	for _, row := range rows {
		reviews = append(reviews, dto.FromProductReview(row))
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(reviews); err != nil {
		return
	}
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID

	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	result, err := h.loadProductDetail(r.Context(), uuid)
	if errors.Is(err, errProductNotFound) {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to load product detail", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) < 2 || len(ids) > 3 {
		http.Error(w, "comparison requires between 2 and 3 product ids", http.StatusBadRequest)
		return
	}

	seen := make(map[string]struct{}, len(ids))
	productDetails := make([]dto.ProductDetail, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			http.Error(w, "invalid product id", http.StatusBadRequest)
			return
		}
		if _, exists := seen[id]; exists {
			http.Error(w, "duplicate product ids are not allowed", http.StatusBadRequest)
			return
		}
		seen[id] = struct{}{}

		var publicID pgtype.UUID
		if err := publicID.Scan(id); err != nil {
			http.Error(w, "invalid product id", http.StatusBadRequest)
			return
		}

		product, err := h.loadProductDetail(r.Context(), publicID)
		if errors.Is(err, errProductNotFound) {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "failed to load product comparison", http.StatusInternalServerError)
			return
		}
		productDetails = append(productDetails, product)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildProductComparison(productDetails))
}

func buildProductComparison(products []dto.ProductDetail) dto.ProductComparison {
	candidates := make([]ComparisonProduct, 0, len(products))
	for _, product := range products {
		candidate := ComparisonProduct{
			ID:     product.ID,
			Rating: product.Rating,
			Offers: make([]ComparisonOffer, 0, len(product.Offers)),
		}
		for _, offer := range product.Offers {
			price, _ := strconv.ParseFloat(strings.TrimSpace(offer.Price), 64)
			candidate.Offers = append(candidate.Offers, ComparisonOffer{
				Price:     price,
				Available: offer.Available,
			})
		}
		candidates = append(candidates, candidate)
	}

	result := CompareProducts(candidates)
	scores := make([]dto.ProductComparisonScore, 0, len(result.Scores))
	for _, score := range result.Scores {
		scores = append(scores, dto.ProductComparisonScore{
			ProductID: score.ProductID,
			Score:     score.Score,
			Reasons:   score.Reasons,
		})
	}

	return dto.ProductComparison{
		Products: products,
		Recommendation: dto.ProductRecommendation{
			WinnerIDs:     result.WinnerIDs,
			IsTie:         result.IsTie,
			Score:         result.Score,
			TotalCriteria: result.TotalCriteria,
			Scores:        scores,
		},
	}
}

func (h *Handler) loadProductDetail(ctx context.Context, publicID pgtype.UUID) (dto.ProductDetail, error) {
	product, err := h.repository.GetDetailByPublicID(ctx, publicID)
	if err != nil {
		return dto.ProductDetail{}, errProductNotFound
	}

	images, err := h.repository.ListImages(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	offers, err := h.repository.ListOffers(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	specifications, err := h.repository.ListSpecifications(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}
	priceHistory, err := h.repository.ListPriceHistory(ctx, product.ID)
	if err != nil {
		return dto.ProductDetail{}, err
	}

	result := dto.FromProduct(product)
	result.Images = make([]dto.ProductImage, 0, len(images))
	for _, image := range images {
		result.Images = append(result.Images, dto.FromProductImage(image))
	}
	result.Offers = make([]dto.ProductOffer, 0, len(offers))
	for _, offer := range offers {
		result.Offers = append(result.Offers, dto.FromProductOffer(offer))
	}
	result.Specs = make(map[string]string, len(specifications))
	for _, specification := range specifications {
		result.Specs[specification.Name] = specification.Value
	}
	result.PriceHistory = make([]dto.PricePoint, 0, len(priceHistory))
	result.OfferPriceHistory = make([]dto.PricePoint, 0)
	for _, point := range priceHistory {
		price := ""
		if point.Price.Valid {
			value, err := point.Price.MarshalJSON()
			if err == nil {
				price = string(value)
			}
		}
		date := ""
		if point.RecordedAt.Valid {
			date = point.RecordedAt.Time.Format("2006-01-02")
		}
		pricePoint := dto.PricePoint{Date: date, Price: price}
		result.PriceHistory = append(result.PriceHistory, pricePoint)
		if point.IsPromotional {
			result.OfferPriceHistory = append(result.OfferPriceHistory, pricePoint)
		}
	}

	return result, nil
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID

	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	var request dto.UpdateProductRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	product, err := h.repository.UpdateByPublicID(
		r.Context(),
		request.ToParams(uuid),
	)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(product)
}

func (h *Handler) Deactivate(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID

	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	product, err := h.repository.DeactivateByPublicID(
		r.Context(),
		uuid,
	)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_ = json.NewEncoder(w).Encode(product)
}
