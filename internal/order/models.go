// Package order — создание заказа, статусы и partner-переходы.
package order

import (
	"time"

	"github.com/google/uuid"
)

// Item — позиция заказа (снимок на момент оформления).
type Item struct {
	DishID     uuid.UUID `json:"dish_id" swaggertype:"string" format:"uuid"`
	Name       string    `json:"name"`
	PriceCents int       `json:"price_cents"`
	Qty        int       `json:"qty"`
}

// HistoryItem — одна смена статуса в GET /orders/{id}.
type HistoryItem struct {
	FromStatus *string   `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ChangedBy  string    `json:"changed_by" enums:"user,restaurant,system"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Order — заказ площадки.
type Order struct {
	ID              uuid.UUID     `json:"id" swaggertype:"string" format:"uuid"`
	UserID          string        `json:"-"`
	RestaurantID    uuid.UUID     `json:"restaurant_id" swaggertype:"string" format:"uuid"`
	Status          string        `json:"status" enums:"created,pending_confirmation,confirmed,cooking,delivering,completed,cancelled,rejected"`
	TotalCents      int           `json:"total_cents"`
	DeliveryAddress string        `json:"delivery_address"`
	ConfirmUntil    *time.Time    `json:"confirm_until,omitempty"`
	Items           []Item        `json:"items"`
	StatusHistory   []HistoryItem `json:"status_history,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
}

// ListResponse — список заказов пользователя или заведения.
type ListResponse struct {
	Items []Order `json:"items"`
}

// CreateRequest — POST /orders.
type CreateRequest struct {
	AcceptPartial   bool   `json:"accept_partial"`
	DeliveryAddress string `json:"delivery_address"`
}

// SetStatusRequest — POST /partner/orders/{id}/status.
type SetStatusRequest struct {
	Status string `json:"status" enums:"confirmed,rejected,cooking,delivering,completed"`
	Reason string `json:"reason"`
}
