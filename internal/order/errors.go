package order

import "errors"

var (
	ErrUnauthorized   = errors.New("unauthorized")
	ErrBadRequest     = errors.New("bad request")
	ErrNotFound       = errors.New("not found")
	ErrCannotCreate   = errors.New("cannot create order")
	ErrPartial        = errors.New("partial cart")
	ErrCannotCancel   = errors.New("cannot cancel")
	ErrBadTransition  = errors.New("bad status transition")
	ErrIdempotencyKey = errors.New("idempotency key required")
)
