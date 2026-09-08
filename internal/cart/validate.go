package cart

import (
	"time"

	"github.com/google/uuid"

	"github.com/engory03/avito-kitchen/internal/domain"
)

// InvalidItem — позиция, которую нельзя оформить.
type InvalidItem struct {
	DishID uuid.UUID `json:"dish_id" swaggertype:"string" format:"uuid"`
	Qty    int       `json:"qty" example:"1"`
	Reason string    `json:"reason" example:"out_of_stock" enums:"reservation_expired,out_of_stock,dish_inactive"`
}

// ValidateResult — POST /cart/validate: ok / partial / invalid без создания заказа.
type ValidateResult struct {
	Status       string        `json:"status" example:"ok" enums:"ok,partial,invalid"`
	ValidItems   []Item        `json:"valid_items"`
	InvalidItems []InvalidItem `json:"invalid_items"`
	Blockers     []string      `json:"blockers" enums:"restaurant_closed,below_min_order,empty_cart"`
}

// PartialResult — тело 409 POST /orders при accept_partial=false (часть позиций нельзя).
type PartialResult struct {
	Status       string        `json:"status" example:"partial" enums:"partial"`
	ValidItems   []Item        `json:"valid_items"`
	InvalidItems []InvalidItem `json:"invalid_items"`
	Blockers     []string      `json:"blockers"`
}

// InvalidResult — тело 422 POST /orders, когда заказ нельзя создать.
type InvalidResult struct {
	Status       string        `json:"status" example:"invalid" enums:"invalid"`
	ValidItems   []Item        `json:"valid_items"`
	InvalidItems []InvalidItem `json:"invalid_items"`
	Blockers     []string      `json:"blockers" enums:"restaurant_closed,below_min_order,empty_cart"`
}

type Snapshot struct {
	HasCart             bool
	RestaurantID        uuid.UUID
	IsOpen              bool
	MinOrderAmountCents int
	PartnerBaseURL      string
	Lines               []Line
}

type Line struct {
	CartItemID       uuid.UUID
	DishID           uuid.UUID
	ExternalID       string
	Qty              int
	Name             string
	PriceCents       int
	IsActive         bool
	HoldOK           bool
	HoldExpires      time.Time
	AvailableForUser int
}

func Evaluate(now time.Time, snap Snapshot) ValidateResult {
	out := ValidateResult{
		ValidItems:   []Item{},
		InvalidItems: []InvalidItem{},
		Blockers:     []string{},
	}
	if !snap.HasCart || len(snap.Lines) == 0 {
		out.Status = domain.ValidateInvalid
		out.Blockers = append(out.Blockers, domain.BlockerEmptyCart)
		return out
	}

	for _, line := range snap.Lines {
		if !line.IsActive {
			out.InvalidItems = append(out.InvalidItems, InvalidItem{DishID: line.DishID, Qty: line.Qty, Reason: domain.ReasonDishInactive})
			continue
		}
		if !line.HoldOK {
			out.InvalidItems = append(out.InvalidItems, InvalidItem{DishID: line.DishID, Qty: line.Qty, Reason: domain.ReasonReservationExpired})
			continue
		}
		if line.AvailableForUser < line.Qty {
			out.InvalidItems = append(out.InvalidItems, InvalidItem{DishID: line.DishID, Qty: line.Qty, Reason: domain.ReasonOutOfStock})
			continue
		}
		out.ValidItems = append(out.ValidItems, Item{
			CartItemID:           line.CartItemID,
			DishID:               line.DishID,
			Qty:                  line.Qty,
			ReservationExpiresAt: line.HoldExpires,
		})
	}

	total := 0
	for _, v := range out.ValidItems {
		for _, line := range snap.Lines {
			if line.CartItemID == v.CartItemID {
				total += line.PriceCents * v.Qty
				break
			}
		}
	}

	if !snap.IsOpen {
		out.Blockers = append(out.Blockers, domain.BlockerRestaurantClosed)
	}
	if total < snap.MinOrderAmountCents {
		out.Blockers = append(out.Blockers, domain.BlockerBelowMinOrder)
	}

	switch {
	case len(out.Blockers) > 0:
		out.Status = domain.ValidateInvalid
	case len(out.InvalidItems) == 0:
		out.Status = domain.ValidateOK
	case len(out.ValidItems) > 0:
		out.Status = domain.ValidatePartial
	default:
		out.Status = domain.ValidateInvalid
	}
	return out
}
