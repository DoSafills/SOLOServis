package services

import (
	"context"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/apierror"
	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// resourceName is used in 404 messages.
const resourceName = "service"

// Handler exposes the services endpoints over HTTP.
type Handler struct {
	repository Repository
}

// NewHandler builds a Handler backed by the given repository.
func NewHandler(repository Repository) *Handler {
	return &Handler{repository: repository}
}

type listOutput struct {
	Body []dto.ServiceDetail
}

type detailInput struct {
	PublicID string `path:"publicID" format:"uuid" doc:"Public identifier of the service." example:"1b7f1f2e-3a4b-4c5d-8e9f-0a1b2c3d4e5f"`
}

type detailOutput struct {
	Body dto.ServiceDetail
}

// Register attaches both services operations to the API.
func (h *Handler) Register(api huma.API, basePath string) {
	huma.Register(api, huma.Operation{
		OperationID: "list-services",
		Method:      http.MethodGet,
		Path:        basePath,
		Summary:     "List services",
		Description: `Returns every active service with its provider offers, specifications and price history.

Offers are ordered by ascending price, and the monthlyPrice, installationCost,
contractMonths, provider and coverage fields describe the cheapest one.`,
		Tags: []string{"Services"},
	}, h.list)

	huma.Register(api, huma.Operation{
		OperationID: "get-service",
		Method:      http.MethodGet,
		Path:        basePath + "/{publicID}",
		Summary:     "Get a service",
		Description: "Returns a single service with its provider offers, specifications and price history.",
		Tags:        []string{"Services"},
		Responses: map[string]*huma.Response{
			"404": {Description: "No service exists with the given public identifier."},
		},
	}, h.getByPublicID)
}

func (h *Handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	rows, err := h.repository.List(ctx)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.List")
	}

	serviceIDs := make([]int32, 0, len(rows))
	for _, row := range rows {
		serviceIDs = append(serviceIDs, row.ID)
	}

	// Three batched queries replace the three queries per service the previous
	// implementation issued.
	offers, err := h.repository.ListOffersByServiceIDs(ctx, serviceIDs)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.ListOffersByServiceIDs")
	}

	specifications, err := h.repository.ListSpecificationsByServiceIDs(ctx, serviceIDs)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.ListSpecificationsByServiceIDs")
	}

	history, err := h.repository.ListPriceHistoryByServiceIDs(ctx, serviceIDs)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.ListPriceHistoryByServiceIDs")
	}

	relations := dto.NewRelations(offers, specifications, history)

	return &listOutput{Body: dto.ListServices(rows, relations)}, nil
}

func (h *Handler) getByPublicID(ctx context.Context, input *detailInput) (*detailOutput, error) {
	var publicID pgtype.UUID

	if err := publicID.Scan(input.PublicID); err != nil {
		return nil, apierror.Unprocessable("the public identifier is not a valid UUID", &huma.ErrorDetail{
			Location: "path.publicID",
			Value:    input.PublicID,
		})
	}

	service, err := h.repository.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.GetByPublicID")
	}

	// A single-element batch keeps one mapping path for both endpoints.
	ids := []int32{service.ID}

	offers, err := h.repository.ListOffersByServiceIDs(ctx, ids)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.ListOffersByServiceIDs")
	}

	specifications, err := h.repository.ListSpecificationsByServiceIDs(ctx, ids)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.ListSpecificationsByServiceIDs")
	}

	history, err := h.repository.ListPriceHistoryByServiceIDs(ctx, ids)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "services.ListPriceHistoryByServiceIDs")
	}

	relations := dto.NewRelations(offers, specifications, history)[service.ID]

	return &detailOutput{Body: dto.FromService(service, relations)}, nil
}
