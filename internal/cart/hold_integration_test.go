//go:build integration

package cart_test

import (
	"context"
	"errors"
	"testing"

	"github.com/engory03/avito-kitchen/internal/cart"
	"github.com/engory03/avito-kitchen/internal/testdb"
)

func TestHoldBlocksLastPortion(t *testing.T) {
	pool := testdb.Start(t)
	cfg := testdb.TestConfig()
	menu := testdb.SeedMenu(t, pool, "hold-key", 2)
	if len(menu.Categories) == 0 || len(menu.Categories[0].Dishes) == 0 {
		t.Fatal("seeded menu is empty")
	}
	dishID := menu.Categories[0].Dishes[0].ID
	svc := cart.NewService(cart.NewRepo(pool), cfg.ReservationTTL)
	ctx := context.Background()

	if _, err := svc.Add(ctx, "anna", dishID, 1); err != nil {
		t.Fatalf("anna add: %v", err)
	}
	if _, err := svc.Add(ctx, "boris", dishID, 1); err != nil {
		t.Fatalf("boris add: %v", err)
	}
	_, err := svc.Add(ctx, "clara", dishID, 1)
	if !errors.Is(err, cart.ErrNoStock) {
		t.Fatalf("clara should get no stock, got %v", err)
	}
}
