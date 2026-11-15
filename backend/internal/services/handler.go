package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/DoSafills/SOLOServis/backend/internal/services/dto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	repository *Repository
	logger     *slog.Logger
}

func NewHandler(repository *Repository, logger *slog.Logger) *Handler {
	return &Handler{
		repository: repository,
		logger:     logger,
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, result any) {
	var buffer bytes.Buffer

	if err := json.NewEncoder(&buffer).Encode(result); err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(buffer.Bytes()); err != nil && h.logger != nil {
		h.logger.Error("error writing service response", "error", err)
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repository.List(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	result := make([]dto.ServiceDetail, 0, len(rows))

	for _, row := range rows {
		offers, err := h.repository.ListOffers(r.Context(), row.ID)
		if err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}

		specifications, err := h.repository.ListSpecifications(r.Context(), row.ID)
		if err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}

		history, err := h.repository.ListPriceHistory(r.Context(), row.ID)
		if err != nil {
			httpx.WriteInternal(w, r, h.logger, err)
			return
		}

		result = append(
			result,
			dto.FromServiceList(row, offers, specifications, history),
		)
	}

	h.writeJSON(w, r, result)
}

func (h *Handler) GetByPublicID(w http.ResponseWriter, r *http.Request) {
	publicID := chi.URLParam(r, "publicID")

	var uuid pgtype.UUID

	if err := uuid.Scan(publicID); err != nil {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return
	}

	service, err := h.repository.GetByPublicID(r.Context(), uuid)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	offers, err := h.repository.ListOffers(r.Context(), service.ID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	specifications, err := h.repository.ListSpecifications(r.Context(), service.ID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	history, err := h.repository.ListPriceHistory(r.Context(), service.ID)
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}

	result := h.repository.ToDetail(
		service,
		offers,
		specifications,
		history,
	)

	h.writeJSON(w, r, result)
}
