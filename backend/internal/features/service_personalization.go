package features

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListServiceFavorites(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	if !ok {
		badRequest(w, "userId is required")
		return
	}
	h.rows(w, r, `SELECT jsonb_build_object('serviceId',s.public_id,'name',s.name,'addedAt',i.updated_at)
		FROM service_user_interest i JOIN service s ON s.id=i.service_id
		WHERE i.user_id=$1 AND i.is_favorite ORDER BY i.updated_at DESC`, userID)
}

func (h *Handler) AddServiceFavorite(w http.ResponseWriter, r *http.Request) {
	var input favoriteInput
	if !decode(r, &input) {
		badRequest(w, "invalid request body")
		return
	}
	input.UserID = normalizeUserID(r, input.UserID)
	userID, err := h.service.UserID(r.Context(), input.UserID)
	serviceID, serviceErr := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil || serviceErr != nil {
		apiutil.WriteError(w, http.StatusNotFound, "user or service not found")
		return
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO service_user_interest (user_id,service_id,is_favorite)
		VALUES ($1,$2,TRUE) ON CONFLICT (user_id,service_id)
		DO UPDATE SET is_favorite=TRUE,updated_at=NOW()`, userID, serviceID)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not save service favorite")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
		"serviceId": chi.URLParam(r, "serviceId"),
		"favorite":  true,
	})
}

func (h *Handler) DeleteServiceFavorite(w http.ResponseWriter, r *http.Request) {
	userID, ok := requestUserID(r)
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if !ok || err != nil {
		badRequest(w, "valid userId and serviceId are required")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE service_user_interest
		SET is_favorite=FALSE,updated_at=NOW()
		WHERE user_id=$1 AND service_id=$2 AND is_favorite`, userID, serviceID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "service favorite not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type serviceComparisonInput struct {
	UserID    int32    `json:"userId"`
	Name      string   `json:"name"`
	ServiceID string   `json:"serviceId"`
	Services  []string `json:"serviceIds"`
}

func (h *Handler) CreateServiceComparison(w http.ResponseWriter, r *http.Request) {
	var input serviceComparisonInput
	if !decode(r, &input) || input.UserID <= 0 {
		badRequest(w, "userId is required")
		return
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = "Comparación de servicios"
	}
	if input.ServiceID != "" {
		input.Services = append(input.Services, input.ServiceID)
	}
	if len(input.Services) == 0 {
		badRequest(w, "at least one serviceId is required")
		return
	}
	userID, err := h.service.UserID(r.Context(), input.UserID)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	serviceIDs := make([]int32, 0, len(input.Services))
	for _, publicID := range input.Services {
		serviceID, err := h.service.ServiceID(r.Context(), publicID)
		if err != nil {
			apiutil.WriteError(w, http.StatusNotFound, "service not found")
			return
		}
		serviceIDs = append(serviceIDs, serviceID)
	}

	var categoryID int32
	if err := h.db.QueryRow(r.Context(), `SELECT category_id FROM service WHERE id=$1`, serviceIDs[0]).Scan(&categoryID); err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "service not found")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not start service comparison")
		return
	}
	defer tx.Rollback(r.Context())

	var comparisonID int32
	if err := tx.QueryRow(r.Context(), `INSERT INTO service_comparison (user_id,category_id,name)
		VALUES ($1,$2,$3) RETURNING id`, userID, categoryID, input.Name).Scan(&comparisonID); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not create service comparison")
		return
	}
	for position, serviceID := range serviceIDs {
		if _, err := tx.Exec(r.Context(), `INSERT INTO service_comparison_item
			(comparison_id,service_id,position) VALUES ($1,$2,$3)`, comparisonID, serviceID, position); err != nil {
			apiutil.WriteError(w, http.StatusConflict, "service is already compared or cannot be added")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not save service comparison")
		return
	}

	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":       comparisonID,
		"userId":   userID,
		"name":     input.Name,
		"services": input.Services,
	})
}

func (h *Handler) GetServiceComparison(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "invalid service comparison id")
		return
	}
	result, err := h.repo.QueryJSON(r.Context(), `SELECT jsonb_build_object(
		'id',c.id,'userId',c.user_id,'name',c.name,'createdAt',c.created_at,'updatedAt',c.updated_at,
		'services',COALESCE(jsonb_agg(jsonb_build_object('id',s.public_id,'name',s.name)
			ORDER BY ci.position) FILTER (WHERE s.id IS NOT NULL),'[]'::jsonb))
		FROM service_comparison c
		LEFT JOIN service_comparison_item ci ON ci.comparison_id=c.id
		LEFT JOIN service s ON s.id=ci.service_id
		WHERE c.id=$1 GROUP BY c.id`, id)
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not load service comparison")
		return
	}
	if len(result) == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "service comparison not found")
		return
	}
	var comparison map[string]any
	if err := json.Unmarshal(result[0], &comparison); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "invalid service comparison data")
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, comparison)
}

func (h *Handler) DeleteServiceComparison(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, "invalid service comparison id")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM service_comparison WHERE id=$1`, id)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "service comparison not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddServiceComparisonItem(w http.ResponseWriter, r *http.Request) {
	comparisonID, ok := intParam(r, "id")
	var input serviceComparisonInput
	if !ok || !decode(r, &input) || input.ServiceID == "" {
		badRequest(w, "valid comparison id and serviceId are required")
		return
	}
	serviceID, err := h.service.ServiceID(r.Context(), input.ServiceID)
	if err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "service not found")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not start service comparison update")
		return
	}
	defer tx.Rollback(r.Context())

	var itemCount int32
	if err := tx.QueryRow(r.Context(), `SELECT COUNT(ci.id)::int
		FROM service_comparison c
		LEFT JOIN service_comparison_item ci ON ci.comparison_id=c.id
		WHERE c.id=$1 GROUP BY c.id`, comparisonID).Scan(&itemCount); err != nil {
		apiutil.WriteError(w, http.StatusNotFound, "service comparison not found")
		return
	}
	if itemCount >= 3 {
		badRequest(w, "a service comparison cannot contain more than 3 services")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO service_comparison_item
		(comparison_id,service_id,position)
		VALUES ($1,$2,COALESCE((SELECT MAX(position)+1 FROM service_comparison_item
			WHERE comparison_id=$1),0))`, comparisonID, serviceID); err != nil {
		apiutil.WriteError(w, http.StatusConflict, "service is already compared or cannot be added")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		apiutil.WriteError(w, http.StatusInternalServerError, "could not save service comparison item")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
		"comparisonId": comparisonID,
		"serviceId":    input.ServiceID,
	})
}

func (h *Handler) DeleteServiceComparisonItem(w http.ResponseWriter, r *http.Request) {
	comparisonID, ok := intParam(r, "id")
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if !ok || err != nil {
		badRequest(w, "invalid comparison or service id")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM service_comparison_item
		WHERE comparison_id=$1 AND service_id=$2`, comparisonID, serviceID)
	if err != nil || result.RowsAffected() == 0 {
		apiutil.WriteError(w, http.StatusNotFound, "service comparison item not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
