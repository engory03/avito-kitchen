package catalog

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/engory03/avito-kitchen/internal/httpserver"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListRestaurants возвращает список заведений.
//
//	@Summary		Список заведений
//	@Description	Пустой список — нормальный 200. Витрина может читаться из Redis.
//	@Tags			catalog
//	@Produce		json
//	@Param			cuisine		query		string	false	"фильтр по кухне"
//	@Param			open_only	query		bool	false	"только открытые"
//	@Success		200			{object}	ListResponse
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Router			/api/v1/restaurants [get]
func (h *Handler) ListRestaurants(w http.ResponseWriter, r *http.Request) {
	cuisine := r.URL.Query().Get("cuisine")
	openOnly := false
	if v := r.URL.Query().Get("open_only"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			httpserver.WriteError(w, http.StatusBadRequest, "open_only must be true or false")
			return
		}
		openOnly = b
	}
	list, err := h.svc.List(r.Context(), cuisine, openOnly)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot list restaurants")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, ListResponse{Items: list})
}

// GetRestaurant возвращает карточку заведения.
//
//	@Summary		Карточка заведения
//	@Tags			catalog
//	@Produce		json
//	@Param			id	path		string	true	"ID заведения"	format(uuid)
//	@Success		200	{object}	Restaurant
//	@Failure		400	{object}	httpserver.ErrorBody
//	@Failure		404	{object}	httpserver.ErrorBody
//	@Failure		500	{object}	httpserver.ErrorBody
//	@Router			/api/v1/restaurants/{id} [get]
func (h *Handler) GetRestaurant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid restaurant id")
		return
	}
	item, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, http.StatusNotFound, "restaurant not found")
		return
	}
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot get restaurant")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, item)
}

// GetMenu возвращает меню; available_qty всегда из Postgres.
//
//	@Summary		Меню заведения
//	@Description	Пустое меню — нормальный 200. available_qty = склад минус живые брони.
//	@Tags			catalog
//	@Produce		json
//	@Param			id	path		string	true	"ID заведения"	format(uuid)
//	@Success		200	{object}	Menu
//	@Failure		400	{object}	httpserver.ErrorBody
//	@Failure		404	{object}	httpserver.ErrorBody
//	@Failure		500	{object}	httpserver.ErrorBody
//	@Router			/api/v1/restaurants/{id}/menu [get]
func (h *Handler) GetMenu(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid restaurant id")
		return
	}
	menu, err := h.svc.Menu(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, http.StatusNotFound, "restaurant not found")
		return
	}
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot get menu")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, menu)
}

// PutMenu загружает меню и остатки заведения.
//
//	@Summary		Загрузить меню и остатки
//	@Tags			partner
//	@Accept			json
//	@Produce		json
//	@Param			body		body		PartnerMenuIn	true	"меню"
//	@Success		200			{object}	httpserver.StatusOK
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Security		ApiKey
//	@Router			/api/v1/partner/menu [put]
func (h *Handler) PutMenu(w http.ResponseWriter, r *http.Request) {
	hash, ok := httpserver.APIKeyHashFrom(w, r)
	if !ok {
		return
	}
	var in PartnerMenuIn
	if err := httpserver.DecodeJSON(r, &in); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	err := h.svc.UpsertMenu(r.Context(), hash, in)
	if errors.Is(err, ErrBadRequest) {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid menu payload")
		return
	}
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot save menu")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, httpserver.StatusOK{Status: "ok"})
}

// PatchRestaurant открывает или закрывает заведение.
//
//	@Summary		Открыть или закрыть заведение
//	@Tags			partner
//	@Accept			json
//	@Produce		json
//	@Param			body		body		PatchRestaurantRequest	true	"is_open"
//	@Success		200			{object}	httpserver.StatusOK
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Security		ApiKey
//	@Router			/api/v1/partner/restaurant [patch]
func (h *Handler) PatchRestaurant(w http.ResponseWriter, r *http.Request) {
	restID, ok := h.partnerRestaurant(w, r)
	if !ok {
		return
	}
	var body PatchRestaurantRequest
	if err := httpserver.DecodeJSON(r, &body); err != nil || body.IsOpen == nil {
		httpserver.WriteError(w, http.StatusBadRequest, "is_open required")
		return
	}
	if err := h.svc.PatchOpen(r.Context(), restID, *body.IsOpen); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot update restaurant")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, httpserver.StatusOK{Status: "ok"})
}

// PatchDish обновляет остаток или снимает блюдо с меню.
//
//	@Summary		Обновить остаток или снять блюдо с меню
//	@Description	Если is_active=false, живые брони на блюдо снимаются.
//	@Tags			partner
//	@Accept			json
//	@Produce		json
//	@Param			externalId	path		string				true	"external_id блюда у заведения"
//	@Param			body		body		PatchDishRequest	true	"stock_qty и/или is_active"
//	@Success		200			{object}	httpserver.StatusOK
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Security		ApiKey
//	@Router			/api/v1/partner/dishes/{externalId} [patch]
func (h *Handler) PatchDish(w http.ResponseWriter, r *http.Request) {
	restID, ok := h.partnerRestaurant(w, r)
	if !ok {
		return
	}
	externalID := chi.URLParam(r, "externalId")
	var body PatchDishRequest
	if err := httpserver.DecodeJSON(r, &body); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	err := h.svc.PatchDish(r.Context(), restID, externalID, body.StockQty, body.IsActive)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, http.StatusNotFound, "dish not found")
		return
	}
	if errors.Is(err, ErrBadRequest) {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid dish patch")
		return
	}
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot update dish")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, httpserver.StatusOK{Status: "ok"})
}

func (h *Handler) partnerRestaurant(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	hash, ok := httpserver.APIKeyHashFrom(w, r)
	if !ok {
		return uuid.Nil, false
	}
	id, err := h.svc.RestaurantIDByKey(r.Context(), hash)
	if errors.Is(err, ErrNotFound) {
		httpserver.WriteError(w, http.StatusNotFound, "restaurant not found")
		return uuid.Nil, false
	}
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot find restaurant")
		return uuid.Nil, false
	}
	return id, true
}
