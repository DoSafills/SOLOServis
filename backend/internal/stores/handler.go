package stores

import (
	"context"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/apierror"
	"github.com/DoSafills/SOLOServis/backend/internal/stores/dto"
	"github.com/danielgtaylor/huma/v2"
)

// resourceName is used in error messages.
const resourceName = "store"

// Handler exposes the stores endpoints over HTTP.
type Handler struct {
	repository Repository
}

// NewHandler builds a Handler backed by the given repository.
func NewHandler(repository Repository) *Handler {
	return &Handler{repository: repository}
}

type listOutput struct {
	Body []dto.Store
}

// Register attaches the stores operation to the API.
func (h *Handler) Register(api huma.API, basePath string) {
	huma.Register(api, huma.Operation{
		OperationID: "list-stores",
		Method:      http.MethodGet,
		Path:        basePath,
		Summary:     "List stores",
		Description: "Returns every active store together with its rating and the number of products it offers.",
		Tags:        []string{"Stores"},
	}, h.list)
}

func (h *Handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	rows, err := h.repository.List(ctx)
	if err != nil {
		return nil, apierror.FromRepository(err, resourceName, "stores.List")
	}

	return &listOutput{Body: dto.ListStores(rows)}, nil
}
