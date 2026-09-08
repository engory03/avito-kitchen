// Package cart — корзина и бронь порций в Postgres.
package cart

import (
	"time"

	"github.com/google/uuid"
)

// Item — позиция корзины с бронью.
type Item struct {
	CartItemID           uuid.UUID `json:"cart_item_id" swaggertype:"string" format:"uuid"`
	DishID               uuid.UUID `json:"dish_id" swaggertype:"string" format:"uuid"`
	Qty                  int       `json:"qty"`
	ReservationExpiresAt time.Time `json:"reservation_expires_at"`
}

// Cart — текущая корзина пользователя (одно заведение).
type Cart struct {
	RestaurantID *uuid.UUID `json:"restaurant_id,omitempty" swaggertype:"string" format:"uuid"`
	Items        []Item     `json:"items"`
}

// AddItemRequest — POST /cart/items.
type AddItemRequest struct {
	DishID uuid.UUID `json:"dish_id" swaggertype:"string" format:"uuid"`
	Qty    int       `json:"qty"`
}

// PatchItemRequest — PATCH /cart/items/{itemId}.
type PatchItemRequest struct {
	Qty int `json:"qty"`
}

type addDish struct {
	ID           uuid.UUID
	RestaurantID uuid.UUID
	StockQty     int
	IsActive     bool
	IsOpen       bool
}
