package cart

import (
	"errors"
	"net/http"

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

// Get возвращает текущую корзину пользователя.
//
//	@Summary		Текущая корзина
//	@Tags			cart
//	@Produce		json
//	@Success		200			{object}	Cart	"корзина; пустые items — норма"
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/cart [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	c, err := h.svc.Get(r.Context(), user)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot get cart")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, c)
}

// AddItem добавляет блюдо и создаёт бронь на 10 минут.
//
//	@Summary		Добавить блюдо
//	@Description	qty > 0. Чужое заведение — сначала DELETE /cart. Нет остатка — 409.
//	@Tags			cart
//	@Accept			json
//	@Produce		json
//	@Param			body		body		AddItemRequest	true	"блюдо и количество"
//	@Success		201			{object}	Item	"позиция с бронью на 10 минут"
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		409			{object}	httpserver.ErrorBody
//	@Failure		422			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/cart/items [post]
func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	var body AddItemRequest
	if err := httpserver.DecodeJSON(r, &body); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := h.svc.Add(r.Context(), user, body.DishID, body.Qty)
	if writeCartError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusCreated, item)
}

// PatchItem меняет количество: уменьшить — отпустить бронь; увеличить — добрать остаток.
//
//	@Summary		Изменить количество
//	@Tags			cart
//	@Accept			json
//	@Produce		json
//	@Param			itemId		path		string				true	"ID позиции корзины"	format(uuid)
//	@Param			body		body		PatchItemRequest	true	"новое qty > 0"
//	@Success		200			{object}	Item
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Failure		409			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/cart/items/{itemId} [patch]
func (h *Handler) PatchItem(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	itemID, ok := parseItemID(w, r)
	if !ok {
		return
	}
	var body PatchItemRequest
	if err := httpserver.DecodeJSON(r, &body); err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	item, err := h.svc.Patch(r.Context(), user, itemID, body.Qty)
	if writeCartError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, item)
}

// DeleteItem убирает позицию и снимает бронь.
//
//	@Summary		Убрать из корзины
//	@Tags			cart
//	@Param			itemId		path	string	true	"ID позиции корзины"	format(uuid)
//	@Success		204
//	@Failure		400	{object}	httpserver.ErrorBody
//	@Failure		401	{object}	httpserver.ErrorBody
//	@Failure		404	{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/cart/items/{itemId} [delete]
func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	itemID, ok := parseItemID(w, r)
	if !ok {
		return
	}
	err := h.svc.DeleteItem(r.Context(), user, itemID)
	if writeCartError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Clear очищает корзину и снимает все брони.
//
//	@Summary		Очистить корзину
//	@Tags			cart
//	@Success		204
//	@Failure		401	{object}	httpserver.ErrorBody
//	@Failure		500	{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/cart [delete]
func (h *Handler) Clear(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	if err := h.svc.Clear(r.Context(), user); err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot clear cart")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Validate проверяет корзину без создания заказа.
//
//	@Summary		Проверить корзину
//	@Description	status: ok, partial или invalid. Заказ не создаётся.
//	@Tags			cart
//	@Produce		json
//	@Success		200			{object}	ValidateResult	"смотри status, invalid_items.reason, blockers"
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/cart/validate [post]
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	res, err := h.svc.Validate(r.Context(), user)
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot validate cart")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, res)
}

func parseItemID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "itemId"))
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid cart item id")
		return uuid.Nil, false
	}
	return id, true
}

func writeCartError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrUnauthorized):
		httpserver.WriteError(w, http.StatusUnauthorized, "X-User-Id required")
	case errors.Is(err, ErrBadRequest):
		httpserver.WriteError(w, http.StatusBadRequest, "invalid dish_id or qty")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteError(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrNoStock):
		httpserver.WriteError(w, http.StatusConflict, "not enough stock")
	case errors.Is(err, ErrOtherRestaurant):
		httpserver.WriteError(w, http.StatusConflict, "cart has another restaurant, DELETE /cart first")
	case errors.Is(err, ErrRestaurantClosed):
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "restaurant is closed")
	case errors.Is(err, ErrDishInactive):
		httpserver.WriteError(w, http.StatusUnprocessableEntity, "dish is not on the menu")
	default:
		httpserver.WriteError(w, http.StatusInternalServerError, "cart error")
	}
	return true
}
