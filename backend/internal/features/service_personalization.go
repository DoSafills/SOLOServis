package features

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/DoSafills/SOLOServis/backend/internal/apiutil"
	"github.com/DoSafills/SOLOServis/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

// maxComparedServices es el máximo de servicios en una comparación.
const maxComparedServices = 3

func (h *Handler) ListServiceFavorites(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	h.rows(w, r, `SELECT jsonb_build_object('serviceId',s.public_id,'name',s.name,'addedAt',i.updated_at)
		FROM service_user_interest i JOIN service s ON s.id=i.service_id
		WHERE i.user_id=$1 AND i.is_favorite ORDER BY i.updated_at DESC`, userID)
}

func (h *Handler) AddServiceFavorite(w http.ResponseWriter, r *http.Request) {
	var input favoriteInput
	if !decode(w, r, &input) {
		return
	}
	userID, err := h.service.UserID(r.Context(), sessionUserID(r))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Usuario no encontrado.")
		return
	}
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	_, err = h.db.Exec(r.Context(), `INSERT INTO service_user_interest (user_id,service_id,is_favorite)
		VALUES ($1,$2,TRUE) ON CONFLICT (user_id,service_id)
		DO UPDATE SET is_favorite=TRUE,updated_at=NOW()`, userID, serviceID)
	if err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo guardar el servicio favorito.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
		"serviceId": chi.URLParam(r, "serviceId"),
		"favorite":  true,
	})
}

func (h *Handler) DeleteServiceFavorite(w http.ResponseWriter, r *http.Request) {
	userID := sessionUserID(r)
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	result, err := h.db.Exec(r.Context(), `UPDATE service_user_interest
		SET is_favorite=FALSE,updated_at=NOW()
		WHERE user_id=$1 AND service_id=$2 AND is_favorite`, userID, serviceID)
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio favorito no encontrado.")
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
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "Comparación de servicios"
	}
	if input.ServiceID != "" {
		input.Services = append(input.Services, input.ServiceID)
	}
	var v httpx.Validator
	v.Check(len(input.Services) > 0, "serviceIds", "required", "serviceIds debe tener al menos un servicio")
	if !valid(w, r, &v) {
		return
	}
	userID, err := h.service.UserID(r.Context(), sessionUserID(r))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Usuario no encontrado.")
		return
	}

	serviceIDs := make([]int32, 0, len(input.Services))
	for i, publicID := range input.Services {
		serviceID, err := h.service.ServiceID(r.Context(), publicID)
		if err != nil {
			h.referenceError(w, r, err, fmt.Sprintf("serviceIds.%d", i))
			return
		}
		serviceIDs = append(serviceIDs, serviceID)
	}

	var categoryID int32
	if err := h.db.QueryRow(r.Context(), `SELECT category_id FROM service WHERE id=$1`, serviceIDs[0]).Scan(&categoryID); err != nil {
		h.referenceError(w, r, err, "serviceIds.0")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	defer tx.Rollback(r.Context())

	var comparisonID int32
	if err := tx.QueryRow(r.Context(), `INSERT INTO service_comparison (user_id,category_id,name)
		VALUES ($1,$2,$3) RETURNING id`, userID, categoryID, input.Name).Scan(&comparisonID); err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo crear la comparación de servicios.")
		return
	}
	for position, serviceID := range serviceIDs {
		if _, err := tx.Exec(r.Context(), `INSERT INTO service_comparison_item
			(comparison_id,service_id,position) VALUES ($1,$2,$3)`, comparisonID, serviceID, position); err != nil {
			h.dbError(w, r, err, http.StatusConflict, "Un servicio está repetido en la comparación.")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo guardar la comparación de servicios.")
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
		badRequest(w, r, "El id de la comparación no es válido.")
		return
	}
	result, err := h.repo.QueryJSON(r.Context(), `SELECT jsonb_build_object(
		'id',c.id,'userId',c.user_id,'name',c.name,'createdAt',c.created_at,'updatedAt',c.updated_at,
		'services',COALESCE(jsonb_agg(jsonb_build_object('id',s.public_id,'name',s.name)
			ORDER BY ci.position) FILTER (WHERE s.id IS NOT NULL),'[]'::jsonb))
		FROM service_comparison c
		LEFT JOIN service_comparison_item ci ON ci.comparison_id=c.id
		LEFT JOIN service s ON s.id=ci.service_id
		WHERE c.id=$1 AND c.user_id=$2 GROUP BY c.id`, id, sessionUserID(r))
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	if len(result) == 0 {
		writeError(w, r, http.StatusNotFound, "Comparación de servicios no encontrada.")
		return
	}
	var comparison map[string]any
	if err := json.Unmarshal(result[0], &comparison); err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	apiutil.WriteJSON(w, http.StatusOK, comparison)
}

func (h *Handler) DeleteServiceComparison(w http.ResponseWriter, r *http.Request) {
	id, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id de la comparación no es válido.")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM service_comparison WHERE id=$1 AND user_id=$2`, id, sessionUserID(r))
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "Comparación de servicios no encontrada.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddServiceComparisonItem(w http.ResponseWriter, r *http.Request) {
	comparisonID, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id de la comparación no es válido.")
		return
	}
	var input serviceComparisonInput
	if !decode(w, r, &input) {
		return
	}
	var v httpx.Validator
	v.Required("serviceId", input.ServiceID)
	if !valid(w, r, &v) {
		return
	}
	serviceID, err := h.service.ServiceID(r.Context(), input.ServiceID)
	if err != nil {
		h.referenceError(w, r, err, "serviceId")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		httpx.WriteInternal(w, r, h.logger, err)
		return
	}
	defer tx.Rollback(r.Context())

	var itemCount int32
	if err := tx.QueryRow(r.Context(), `SELECT COUNT(ci.id)::int
		FROM service_comparison c
		LEFT JOIN service_comparison_item ci ON ci.comparison_id=c.id
		WHERE c.id=$1 AND c.user_id=$2 GROUP BY c.id`, comparisonID, sessionUserID(r)).Scan(&itemCount); err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Comparación de servicios no encontrada.")
		return
	}
	if itemCount >= maxComparedServices {
		httpx.WriteValidation(w, r, []httpx.FieldError{{Field: "serviceId", Code: "max",
			Message: fmt.Sprintf("Una comparación no puede tener más de %d servicios", maxComparedServices)}})
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO service_comparison_item
		(comparison_id,service_id,position)
		VALUES ($1,$2,COALESCE((SELECT MAX(position)+1 FROM service_comparison_item
			WHERE comparison_id=$1),0))`, comparisonID, serviceID); err != nil {
		h.dbError(w, r, err, http.StatusConflict, "El servicio ya está en la comparación.")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.dbError(w, r, err, http.StatusInternalServerError, "No se pudo guardar el servicio en la comparación.")
		return
	}
	apiutil.WriteJSON(w, http.StatusCreated, map[string]any{
		"comparisonId": comparisonID,
		"serviceId":    input.ServiceID,
	})
}

func (h *Handler) DeleteServiceComparisonItem(w http.ResponseWriter, r *http.Request) {
	comparisonID, ok := intParam(r, "id")
	if !ok {
		badRequest(w, r, "El id de la comparación no es válido.")
		return
	}
	serviceID, err := h.service.ServiceID(r.Context(), chi.URLParam(r, "serviceId"))
	if err != nil {
		h.dbError(w, r, err, http.StatusNotFound, "Servicio no encontrado.")
		return
	}
	result, err := h.db.Exec(r.Context(), `DELETE FROM service_comparison_item ci
		USING service_comparison c
		WHERE ci.comparison_id=c.id AND c.id=$1 AND c.user_id=$3 AND ci.service_id=$2`,
		comparisonID, serviceID, sessionUserID(r))
	if err != nil || result.RowsAffected() == 0 {
		h.dbError(w, r, err, http.StatusNotFound, "El servicio no está en la comparación.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
