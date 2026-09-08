package order

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

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

// Create оформляет заказ. Повтор с тем же Idempotency-Key возвращает тот же заказ.
//
//	@Summary		Создать заказ
//	@Description	Ключ идемпотентности обязателен. Создание ещё раз проверяет бронь и остатки. Смотри поле error (400/401/500) или status+invalid_items+blockers (409/422).
//	@Tags			orders
//	@Accept			json
//	@Produce		json
//	@Param			Idempotency-Key		header		string			true	"ключ идемпотентности"
//	@Param			body				body		CreateRequest	true	"адрес и accept_partial"
//	@Success		201					{object}	Order	"новый заказ"
//	@Success		200					{object}	Order	"тот же заказ по Idempotency-Key (replay)"
//	@Failure		400					{object}	httpserver.ErrorBody
//	@Failure		401					{object}	httpserver.ErrorBody
//	@Failure		409					{object}	cart.PartialResult
//	@Failure		422					{object}	cart.InvalidResult
//	@Failure		500					{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/orders [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	key := r.Header.Get(httpserver.HeaderIdempotency)
	var body CreateRequest
	dec := json.NewDecoder(r.Body)
	_ = dec.Decode(&body)

	o, replay, check, err := h.svc.Create(r.Context(), user, key, body.DeliveryAddress, body.AcceptPartial)
	if errors.Is(err, ErrIdempotencyKey) {
		httpserver.WriteError(w, http.StatusBadRequest, "Idempotency-Key required")
		return
	}
	if errors.Is(err, ErrCannotCreate) {
		httpserver.WriteJSON(w, http.StatusUnprocessableEntity, check)
		return
	}
	if errors.Is(err, ErrPartial) {
		httpserver.WriteJSON(w, http.StatusConflict, check)
		return
	}
	if errors.Is(err, ErrBadRequest) {
		httpserver.WriteError(w, http.StatusBadRequest, "idempotency key already used")
		return
	}
	if err != nil {
		httpserver.WriteError(w, http.StatusInternalServerError, "cannot create order")
		return
	}
	code := http.StatusCreated
	if replay {
		code = http.StatusOK
	}
	httpserver.WriteJSON(w, code, o)
}

// Get возвращает заказ; если вышел confirm_until — уже cancelled.
//
//	@Summary		Статус, состав и история заказа
//	@Tags			orders
//	@Produce		json
//	@Param			id			path		string	true	"ID заказа"	format(uuid)
//	@Success		200			{object}	Order	"если вышел confirm_until — status уже cancelled"
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/orders/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}
	o, err := h.svc.Get(r.Context(), user, id)
	if writeOrderError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, o)
}

// Cancel отменяет заказ из created или pending_confirmation.
//
//	@Summary		Отменить заказ
//	@Description	После confirmed отмена недоступна (409). Остаток возвращается.
//	@Tags			orders
//	@Produce		json
//	@Param			id			path		string	true	"ID заказа"	format(uuid)
//	@Success		200			{object}	Order	"заказ cancelled, остаток возвращён"
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Failure		409			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/orders/{id}/cancel [post]
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}
	o, err := h.svc.Cancel(r.Context(), user, id)
	if writeOrderError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, o)
}

// ListUser возвращает историю заказов пользователя.
//
//	@Summary		История заказов пользователя
//	@Description	Пустой список — нормальный 200. X-User-Id должен совпадать с userId.
//	@Tags			orders
//	@Produce		json
//	@Param			userId		path		string	true	"тот же user id"
//	@Success		200			{object}	ListResponse
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Failure		500			{object}	httpserver.ErrorBody
//	@Security		UserID
//	@Router			/api/v1/users/{userId}/orders [get]
func (h *Handler) ListUser(w http.ResponseWriter, r *http.Request) {
	user, ok := httpserver.UserFrom(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListUser(r.Context(), user, chi.URLParam(r, "userId"))
	if writeOrderError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, ListResponse{Items: list})
}

// PartnerList возвращает заказы заведения.
//
//	@Summary		Список заказов для заведения
//	@Tags			partner
//	@Produce		json
//	@Param			since		query		string	false	"RFC3339"	format(date-time)
//	@Param			status		query		string	false	"фильтр статуса"
//	@Success		200			{object}	ListResponse
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Security		ApiKey
//	@Router			/api/v1/partner/orders [get]
func (h *Handler) PartnerList(w http.ResponseWriter, r *http.Request) {
	hash, ok := httpserver.APIKeyHashFrom(w, r)
	if !ok {
		return
	}
	var since *time.Time
	if raw := r.URL.Query().Get("since"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			httpserver.WriteError(w, http.StatusBadRequest, "since must be RFC3339")
			return
		}
		since = &t
	}
	list, err := h.svc.PartnerList(r.Context(), hash, r.URL.Query().Get("status"), since)
	if writeOrderError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, ListResponse{Items: list})
}

// PartnerSetStatus меняет статус заказа со стороны заведения.
//
//	@Summary		Сменить статус заказа
//	@Tags			partner
//	@Accept			json
//	@Produce		json
//	@Param			id			path		string				true	"ID заказа"	format(uuid)
//	@Param			body		body		SetStatusRequest	true	"новый статус"
//	@Success		200			{object}	Order
//	@Failure		400			{object}	httpserver.ErrorBody
//	@Failure		401			{object}	httpserver.ErrorBody
//	@Failure		404			{object}	httpserver.ErrorBody
//	@Failure		409			{object}	httpserver.ErrorBody
//	@Security		ApiKey
//	@Router			/api/v1/partner/orders/{id}/status [post]
func (h *Handler) PartnerSetStatus(w http.ResponseWriter, r *http.Request) {
	hash, ok := httpserver.APIKeyHashFrom(w, r)
	if !ok {
		return
	}
	id, ok := parseOrderID(w, r)
	if !ok {
		return
	}
	var body SetStatusRequest
	if err := httpserver.DecodeJSON(r, &body); err != nil || body.Status == "" {
		httpserver.WriteError(w, http.StatusBadRequest, "status required")
		return
	}
	o, err := h.svc.PartnerSetStatus(r.Context(), hash, id, body.Status, body.Reason)
	if writeOrderError(w, err) {
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, o)
}

func parseOrderID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.WriteError(w, http.StatusBadRequest, "invalid order id")
		return uuid.Nil, false
	}
	return id, true
}

func writeOrderError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrUnauthorized):
		httpserver.WriteError(w, http.StatusUnauthorized, "X-User-Id required")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteError(w, http.StatusNotFound, "order not found")
	case errors.Is(err, ErrCannotCancel):
		httpserver.WriteError(w, http.StatusConflict, "cancellation is not available after confirmed")
	case errors.Is(err, ErrBadTransition):
		httpserver.WriteError(w, http.StatusConflict, "cannot change status from current")
	case errors.Is(err, ErrBadRequest):
		httpserver.WriteError(w, http.StatusBadRequest, "bad request")
	default:
		httpserver.WriteError(w, http.StatusInternalServerError, "order error")
	}
	return true
}
