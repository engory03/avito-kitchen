// Package catalog — витрина заведений и partner-меню.
package catalog

import "github.com/google/uuid"

// Restaurant — карточка заведения на витрине.
type Restaurant struct {
	ID                  uuid.UUID `json:"id" swaggertype:"string" format:"uuid"`
	Name                string    `json:"name"`
	Cuisine             string    `json:"cuisine"`
	IsOpen              bool      `json:"is_open"`
	MinOrderAmountCents int       `json:"min_order_amount_cents"`
}

// ListResponse — GET /restaurants.
type ListResponse struct {
	Items []Restaurant `json:"items"`
}

// Dish — позиция меню; available_qty считается из Postgres, не из Redis.
type Dish struct {
	ID           uuid.UUID `json:"id" swaggertype:"string" format:"uuid"`
	ExternalID   string    `json:"external_id"`
	Name         string    `json:"name"`
	PriceCents   int       `json:"price_cents"`
	AvailableQty int       `json:"available_qty"`
	IsActive     bool      `json:"is_active"`
}

// ShowcaseDish — витрина для Redis, без свободного остатка.
type ShowcaseDish struct {
	ID         uuid.UUID `json:"id"`
	ExternalID string    `json:"external_id"`
	Name       string    `json:"name"`
	PriceCents int       `json:"price_cents"`
	IsActive   bool      `json:"is_active"`
}

// Category — раздел меню.
type Category struct {
	ID        uuid.UUID `json:"id" swaggertype:"string" format:"uuid"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	Dishes    []Dish    `json:"dishes"`
}

type ShowcaseCategory struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	SortOrder int            `json:"sort_order"`
	Dishes    []ShowcaseDish `json:"dishes"`
}

// Menu — меню заведения с живыми остатками.
type Menu struct {
	RestaurantID uuid.UUID  `json:"restaurant_id" swaggertype:"string" format:"uuid"`
	Categories   []Category `json:"categories"`
}

type ShowcaseMenu struct {
	RestaurantID uuid.UUID          `json:"restaurant_id"`
	Categories   []ShowcaseCategory `json:"categories"`
}

// PartnerMenuIn — PUT /partner/menu: меню и остатки от заведения.
type PartnerMenuIn struct {
	Restaurant PartnerRestaurantIn `json:"restaurant"`
	Categories []PartnerCategoryIn `json:"categories"`
}

// PatchRestaurantRequest — PATCH /partner/restaurant.
type PatchRestaurantRequest struct {
	IsOpen *bool `json:"is_open"`
}

// PatchDishRequest — PATCH /partner/dishes/{externalId}.
type PatchDishRequest struct {
	StockQty *int  `json:"stock_qty"`
	IsActive *bool `json:"is_active"`
}

type PartnerRestaurantIn struct {
	Name                string `json:"name"`
	Cuisine             string `json:"cuisine"`
	MinOrderAmountCents int    `json:"min_order_amount_cents"`
	IsOpen              *bool  `json:"is_open"`
	PartnerBaseURL      string `json:"partner_base_url"`
}

type PartnerCategoryIn struct {
	Name      string          `json:"name"`
	SortOrder int             `json:"sort_order"`
	Dishes    []PartnerDishIn `json:"dishes"`
}

type PartnerDishIn struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	StockQty   int    `json:"stock_qty"`
	IsActive   *bool  `json:"is_active"`
}

func (d PartnerDishIn) Active() bool {
	if d.IsActive == nil {
		return true
	}
	return *d.IsActive
}

func (r PartnerRestaurantIn) Open() bool {
	if r.IsOpen == nil {
		return true
	}
	return *r.IsOpen
}
