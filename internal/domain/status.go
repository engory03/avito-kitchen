package domain

// Статусы заказа. Отмена пользователем — только created и pending_confirmation.
const (
	OrderCreated             = "created"
	OrderPendingConfirmation = "pending_confirmation"
	OrderConfirmed           = "confirmed"
	OrderCooking             = "cooking"
	OrderDelivering          = "delivering"
	OrderCompleted           = "completed"
	OrderCancelled           = "cancelled"
	OrderRejected            = "rejected"
)

// UserMayCancel — граница из design-decisions: после confirmed отмена недоступна.
func UserMayCancel(status string) bool {
	return status == OrderCreated || status == OrderPendingConfirmation
}

// ConfirmDeadlineApplies — заказ ещё ждёт заведение, можно снять по confirm_until.
func ConfirmDeadlineApplies(status string) bool {
	return status == OrderCreated || status == OrderPendingConfirmation
}

// Статусы брони блюда.
const (
	ReservationActive   = "active"
	ReservationExpired  = "expired"
	ReservationConsumed = "consumed"
	ReservationReleased = "released"
)

// Кто меняет статус заказа (поле changed_by в истории).
const (
	ActorUser       = "user"
	ActorRestaurant = "restaurant"
	ActorSystem     = "system"
)

// Причины по позициям корзины (invalid_items.reason в OpenAPI).
const (
	ReasonReservationExpired = "reservation_expired"
	ReasonOutOfStock         = "out_of_stock"
	ReasonDishInactive       = "dish_inactive"
)

// Блокеры на весь заказ (blockers в OpenAPI).
const (
	BlockerRestaurantClosed = "restaurant_closed"
	BlockerBelowMinOrder    = "below_min_order"
	BlockerEmptyCart        = "empty_cart"
)

// Результат POST /cart/validate.
const (
	ValidateOK      = "ok"
	ValidatePartial = "partial"
	ValidateInvalid = "invalid"
)

const (
	ReasonPartnerTimeout2s = "partner_timeout_2s"
	ReasonConfirmTimeout   = "confirm_timeout"
)

// PartnerMaySet — заведение двигает статус только вперёд по цепочке.
func PartnerMaySet(from, to string) bool {
	switch from {
	case OrderCreated, OrderPendingConfirmation:
		return to == OrderConfirmed || to == OrderRejected
	case OrderConfirmed:
		return to == OrderCooking
	case OrderCooking:
		return to == OrderDelivering
	case OrderDelivering:
		return to == OrderCompleted
	default:
		return false
	}
}
