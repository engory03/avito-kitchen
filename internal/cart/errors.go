package cart

import "errors"

var (
	ErrBadRequest       = errors.New("bad request")
	ErrNotFound         = errors.New("not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrNoStock          = errors.New("no stock")
	ErrOtherRestaurant  = errors.New("other restaurant")
	ErrRestaurantClosed = errors.New("restaurant closed")
	ErrDishInactive     = errors.New("dish inactive")
)
