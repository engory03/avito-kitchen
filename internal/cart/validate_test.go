package cart

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/engory03/avito-kitchen/internal/domain"
)

func TestEvaluateEmptyCart(t *testing.T) {
	got := Evaluate(time.Now(), Snapshot{HasCart: false, Lines: nil})
	if got.Status != domain.ValidateInvalid {
		t.Fatalf("status=%s", got.Status)
	}
	if len(got.Blockers) != 1 || got.Blockers[0] != domain.BlockerEmptyCart {
		t.Fatalf("blockers=%v", got.Blockers)
	}
}

func TestEvaluateOK(t *testing.T) {
	now := time.Now().UTC()
	dish := uuid.New()
	got := Evaluate(now, Snapshot{
		HasCart: true, IsOpen: true, MinOrderAmountCents: 50000,
		Lines: []Line{{
			CartItemID: uuid.New(), DishID: dish, Qty: 2, PriceCents: 35000,
			IsActive: true, HoldOK: true, HoldExpires: now.Add(time.Minute), AvailableForUser: 10,
		}},
	})
	if got.Status != domain.ValidateOK {
		t.Fatalf("status=%s blockers=%v invalid=%v", got.Status, got.Blockers, got.InvalidItems)
	}
	if len(got.ValidItems) != 1 {
		t.Fatalf("valid=%d", len(got.ValidItems))
	}
}

func TestEvaluateReservationExpired(t *testing.T) {
	dish := uuid.New()
	got := Evaluate(time.Now(), Snapshot{
		HasCart: true, IsOpen: true, MinOrderAmountCents: 1,
		Lines: []Line{{
			CartItemID: uuid.New(), DishID: dish, Qty: 2, PriceCents: 35000,
			IsActive: true, HoldOK: false, AvailableForUser: 10,
		}},
	})
	if got.Status != domain.ValidateInvalid {
		t.Fatalf("status=%s", got.Status)
	}
	if len(got.InvalidItems) != 1 || got.InvalidItems[0].Reason != domain.ReasonReservationExpired {
		t.Fatalf("invalid=%v", got.InvalidItems)
	}
}

func TestEvaluateDishInactiveBeatsExpired(t *testing.T) {
	dish := uuid.New()
	got := Evaluate(time.Now(), Snapshot{
		HasCart: true, IsOpen: true, MinOrderAmountCents: 1,
		Lines: []Line{{
			CartItemID: uuid.New(), DishID: dish, Qty: 1, PriceCents: 100,
			IsActive: false, HoldOK: false,
		}},
	})
	if len(got.InvalidItems) != 1 || got.InvalidItems[0].Reason != domain.ReasonDishInactive {
		t.Fatalf("invalid=%v", got.InvalidItems)
	}
}

func TestEvaluateOutOfStock(t *testing.T) {
	dish := uuid.New()
	got := Evaluate(time.Now(), Snapshot{
		HasCart: true, IsOpen: true, MinOrderAmountCents: 1,
		Lines: []Line{{
			CartItemID: uuid.New(), DishID: dish, Qty: 5, PriceCents: 100,
			IsActive: true, HoldOK: true, HoldExpires: time.Now().Add(time.Minute), AvailableForUser: 2,
		}},
	})
	if len(got.InvalidItems) != 1 || got.InvalidItems[0].Reason != domain.ReasonOutOfStock {
		t.Fatalf("invalid=%v", got.InvalidItems)
	}
}

func TestEvaluatePartial(t *testing.T) {
	okDish := uuid.New()
	badDish := uuid.New()
	got := Evaluate(time.Now(), Snapshot{
		HasCart: true, IsOpen: true, MinOrderAmountCents: 100,
		Lines: []Line{
			{CartItemID: uuid.New(), DishID: okDish, Qty: 2, PriceCents: 100, IsActive: true, HoldOK: true, HoldExpires: time.Now().Add(time.Minute), AvailableForUser: 5},
			{CartItemID: uuid.New(), DishID: badDish, Qty: 1, PriceCents: 100, IsActive: true, HoldOK: false, AvailableForUser: 5},
		},
	})
	if got.Status != domain.ValidatePartial {
		t.Fatalf("status=%s blockers=%v", got.Status, got.Blockers)
	}
	if len(got.ValidItems) != 1 || len(got.InvalidItems) != 1 {
		t.Fatalf("valid=%d invalid=%d", len(got.ValidItems), len(got.InvalidItems))
	}
}

func TestEvaluateRestaurantClosed(t *testing.T) {
	got := Evaluate(time.Now(), Snapshot{
		HasCart: true, IsOpen: false, MinOrderAmountCents: 1,
		Lines: []Line{{
			CartItemID: uuid.New(), DishID: uuid.New(), Qty: 1, PriceCents: 100,
			IsActive: true, HoldOK: true, HoldExpires: time.Now().Add(time.Minute), AvailableForUser: 5,
		}},
	})
	if got.Status != domain.ValidateInvalid {
		t.Fatalf("status=%s", got.Status)
	}
	found := false
	for _, b := range got.Blockers {
		if b == domain.BlockerRestaurantClosed {
			found = true
		}
	}
	if !found {
		t.Fatalf("blockers=%v", got.Blockers)
	}
}

func TestEvaluateBelowMinOrder(t *testing.T) {
	got := Evaluate(time.Now(), Snapshot{
		HasCart: true, IsOpen: true, MinOrderAmountCents: 50000,
		Lines: []Line{{
			CartItemID: uuid.New(), DishID: uuid.New(), Qty: 1, PriceCents: 12000,
			IsActive: true, HoldOK: true, HoldExpires: time.Now().Add(time.Minute), AvailableForUser: 5,
		}},
	})
	if got.Status != domain.ValidateInvalid {
		t.Fatalf("status=%s", got.Status)
	}
	found := false
	for _, b := range got.Blockers {
		if b == domain.BlockerBelowMinOrder {
			found = true
		}
	}
	if !found {
		t.Fatalf("blockers=%v", got.Blockers)
	}
}
