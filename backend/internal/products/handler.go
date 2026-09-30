package products

import (
	"context"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apierror"
	"github.com/DoSafills/SOLOServis/backend/internal/products/dto"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// resourceName is used in 404 messages.
const resourceName = "product"

// Handler exposes the products endpoints over HTTP.
type Handler struct {
	repository Repository
}

// NewHandler builds a Handler backed by the given repository.
func NewHandler(repository Repository) *Handler {
	return &Handler{repository: repository}
}

// listInput carries the optional filters of GET /products. Query parameters are
// optional by default, so omitting them reproduces the unfiltered listing.
type listInput struct {
	Search   string `query:"search" maxLength:"150" doc:"Case-insensitive match against product name, model or SKU." example:"refrigerador"`
	Category string `query:"category" maxLength:"150" doc:"Case-insensitive match against the category name." example:"Refrigeradores"`
	Brand    string `query:"brand" maxLength:"150" doc:"Case-insensitive match against the brand name." example:"Samsung"`
	Limit    int32  `query:"limit" minimum:"1" maximum:"100" doc:"Maximum number of products to return. Omit for no limit." example:"20"`
	Offset   int32  `query:"offset" minimum:"0" doc:"Number of products to skip before returning results." example:"0"`
}

type listOutput struct {
	Body []dto.ProductListItem
}

// detailInput identifies a single product.
type detailInput struct {
	PublicID string `path:"publicID" format:"uuid" doc:"Public identifier of the product." example:"6f1c1f7e-0a1a-4a0a-9a0a-1a2b3c4d5e6f"`
}

type detailOutput struct {
	Body dto.ProductDetail
}

// createInput is the payload of POST /products.
type createInput struct {
	Body dto.CreateProductRequest
}

type createOutput struct {
	// Location points at the newly created resource, as required by
	// RFC 9110 for a 201 response.
	Location string `header:"Location" doc:"URL of the created product."`
	Body     dto.CreatedProduct
}

// updateInput is the payload of PUT /products/{publicID}.
type updateInput struct {
	PublicID string `path:"publicID" format:"uuid" doc:"Public identifier of the product to update." example:"6f1c1f7e-0a1a-4a0a-9a0a-1a2b3c4d5e6f"`
	Body     dto.UpdateProductRequest
}

type updateOutput struct {
	Body dto.CreatedProduct
}

// deactivateInput identifies the product to soft delete.
type deactivateInput struct {
	PublicID string `path:"publicID" format:"uuid" doc:"Public identifier of the product to deactivate." example:"6f1c1f7e-0a1a-4a0a-9a0a-1a2b3c4d5e6f"`
}

type deactivateOutput struct {
	Body dto.CreatedProduct
}

// Register attaches every products operation to the API. Each operation
// declares its own summary, tags and error responses, which is what populates
// the generated OpenAPI document.
func (h *Handler) Register(api huma.API, basePath string) {
	huma.Register(api, huma.Operation{
		OperationID: "list-products",
		Method:      http.MethodGet,
		Path:        basePath,
		Summary:     "List products",
		Description: `Returns the active products, ordered by identifier.

Supports optional text, category and brand filters plus limit/offset pagination.
When ` + "`limit`" + ` is omitted every matching product is returned.`,
		Tags: []string{"Products"},
	}, h.list)

	huma.Register(api, huma.Operation{
		OperationID: "get-product",
		Method:      http.MethodGet,
		Path:        basePath + "/{publicID}",
		Summary:     "Get a product",
		Description: "Returns a single product with its images, offers, specifications and price history.",
		Tags:        []string{"Products"},
		Responses: map[string]*huma.Response{
			"404": {Description: "No product exists with the given public identifier."},
		},
	}, h.getByPublicID)

	huma.Register(api, huma.Operation{
		OperationID:   "create-product",
		Method:        http.MethodPost,
		Path:          basePath,
		Summary:       "Create a product",
		Description:   "Creates a product together with its first image and returns the new resource.",
		Tags:          []string{"Products"},
		DefaultStatus: http.StatusCreated,
		Responses: map[string]*huma.Response{
			"409": {Description: "Another product already uses the supplied SKU."},
			"422": {Description: "The payload failed validation, or references an unknown category or brand."},
		},
	}, h.create)

	huma.Register(api, huma.Operation{
		OperationID: "update-product",
		Method:      http.MethodPut,
		Path:        basePath + "/{publicID}",
		Summary:     "Update a product",
		Description: `Replaces the editable fields of an active product. The product is identified by
the path, not by the body. Inactive products cannot be updated.`,
		Tags: []string{"Products"},
		Responses: map[string]*huma.Response{
			"404": {Description: "No active product exists with the given public identifier."},
			"409": {Description: "Another product already uses the supplied SKU."},
			"422": {Description: "The payload failed validation, or references an unknown category or brand."},
		},
	}, h.update)

	huma.Register(api, huma.Operation{
		OperationID: "deactivate-product",
		Method:      http.MethodDelete,
		Path:        basePath + "/{publicID}",
		Summary:     "Deactivate a product",
		Description: "Soft deletes a product by clearing its active flag. The row is retained.",
		Tags:        []string{"Products"},
		Responses: map[string]*huma.Response{
			"404": {Description: "No active product exists with the given public identifier."},
		},
	}, h.deactivate)
}

func (h *Handler) list(ctx context.Context, input *listInput) (*listOutput, error) {
	filter := ListFilter{
		Search:   strings.TrimSpace(input.Search),
		Category: strings.TrimSpace(input.Category),
		Brand:    strings.TrimSpace(input.Brand),
		Limit:    input.Limit,
		Offset:   input.Offset,
	}

	rows, err := h.repository.List(ctx, filter)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.List")
	}

	ids := make([]int32, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	// Two batched queries replace the two queries per product the previous
	// implementation issued.
	images, err := h.repository.ListImages(ctx, ids)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.ListImages")
	}

	offers, err := h.repository.ListOffers(ctx, ids)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.ListOffers")
	}

	relations := dto.NewProductRelations(images, offers)

	return &listOutput{Body: dto.ListProducts(rows, relations)}, nil
}

func (h *Handler) getByPublicID(ctx context.Context, input *detailInput) (*detailOutput, error) {
	publicID, err := parseUUID(input.PublicID)
	if err != nil {
		return nil, err
	}

	product, err := h.repository.GetDetailByPublicID(ctx, publicID)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.GetDetailByPublicID")
	}

	images, err := h.repository.ListImages(ctx, []int32{product.ID})
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.ListImages")
	}

	offers, err := h.repository.ListOffers(ctx, []int32{product.ID})
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.ListOffers")
	}

	specifications, err := h.repository.ListSpecifications(ctx, product.ID)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.ListSpecifications")
	}

	priceHistory, err := h.repository.ListPriceHistory(ctx, product.ID)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.ListPriceHistory")
	}

	return &detailOutput{
		Body: dto.BuildProductDetail(product, images, offers, specifications, priceHistory),
	}, nil
}

func (h *Handler) create(ctx context.Context, input *createInput) (*createOutput, error) {
	request := input.Body
	request.Normalize()

	if err := validateCreate(&request); err != nil {
		return nil, err
	}

	created, err := h.repository.Create(ctx, request)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.Create")
	}

	return &createOutput{
		Location: "/products/" + created.ID,
		Body:     created,
	}, nil
}

func (h *Handler) update(ctx context.Context, input *updateInput) (*updateOutput, error) {
	publicID, err := parseUUID(input.PublicID)
	if err != nil {
		return nil, err
	}

	request := input.Body
	request.Normalize()

	if err := validateUpdate(&request); err != nil {
		return nil, err
	}

	product, err := h.repository.Update(ctx, publicID, request)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.Update")
	}

	return &updateOutput{
		Body: dto.CreatedProduct{
			ID:   product.PublicID.String(),
			Name: product.Name,
		},
	}, nil
}

func (h *Handler) deactivate(ctx context.Context, input *deactivateInput) (*deactivateOutput, error) {
	publicID, err := parseUUID(input.PublicID)
	if err != nil {
		return nil, err
	}

	product, err := h.repository.Deactivate(ctx, publicID)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "products.Deactivate")
	}

	return &deactivateOutput{
		Body: dto.CreatedProduct{
			ID:   product.PublicID.String(),
			Name: product.Name,
		},
	}, nil
}

// validateCreate applies the business rules that a JSON Schema cannot express.
// Huma has already rejected payloads that break the declared constraints, so
// these checks only cover semantics.
func validateCreate(request *dto.CreateProductRequest) error {
	if request.Name == "" {
		return apierror.Unprocessable("the product name is required", &huma.ErrorDetail{
			Location: "body.name",
			Message:  "expected a non-empty name after trimming whitespace",
		})
	}

	if err := validateImageURL(request.ImageURL); err != nil {
		return err
	}

	return nil
}

// validateUpdate applies the same business rules to an update payload. The
// previous implementation validated nothing here, so an empty name or a
// non-positive category was accepted and stored.
func validateUpdate(request *dto.UpdateProductRequest) error {
	if request.Name == "" {
		return apierror.Unprocessable("the product name is required", &huma.ErrorDetail{
			Location: "body.name",
			Message:  "expected a non-empty name after trimming whitespace",
		})
	}

	return nil
}

func validateImageURL(value string) error {
	// A relative or non-http scheme is not a usable image location, and the
	// schema's "format: uri" only guarantees that it parses as a URI.
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return apierror.Unprocessable("the image URL must use the http or https scheme", &huma.ErrorDetail{
			Location: "body.imageUrl",
			Value:    value,
		})
	}

	return nil
}

// parseUUID converts a validated path parameter into the pgtype form used by
// the generated queries. Huma rejects malformed identifiers before this point,
// so a failure here means the caller skipped validation.
func parseUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID

	if err := parsed.Scan(value); err != nil {
		return parsed, apierror.Unprocessable("the public identifier is not a valid UUID", &huma.ErrorDetail{
			Location: "path.publicID",
			Value:    value,
		})
	}

	return parsed, nil
}
