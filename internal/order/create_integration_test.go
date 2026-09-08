//go:build integration

package order_test

import (
	"context"
	"testing"
	"time"

	"github.com/engory03/avito-kitchen/internal/cart"
	"github.com/engory03/avito-kitchen/internal/catalog"
	"github.com/engory03/avito-kitchen/internal/order"
	"github.com/engory03/avito-kitchen/internal/testdb"
)

func TestCreateOrderConsumesStockAndIsIdempotent(t *testing.T) {
	pool := testdb.Start(t)
	cfg := testdb.TestConfig()
	menu := testdb.SeedMenu(t, pool, "order-key", 10)
	dishID := menu.Categories[0].Dishes[0].ID

	cartSvc := cart.NewService(cart.NewRepo(pool), cfg.ReservationTTL)
	ordSvc := order.NewService(order.NewRepo(pool), cfg.ConfirmTimeout, order.NewNotifier(cfg.PartnerHTTPTimeout))
	ctx := context.Background()

	if _, err := cartSvc.Add(ctx, "anna", dishID, 2); err != nil {
		t.Fatalf("add cart: %v", err)
	}

	first, replay, _, err := ordSvc.Create(ctx, "anna", "key-1", "street 1", false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if replay {
		t.Fatal("first create must not be a replay")
	}
	if first.TotalCents != 70000 {
		t.Fatalf("total=%d", first.TotalCents)
	}

	second, replay, _, err := ordSvc.Create(ctx, "anna", "key-1", "ignored", false)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !replay || second.ID != first.ID {
		t.Fatalf("expected same order, replay=%v id=%s", replay, second.ID)
	}

	got, err := catalog.NewService(catalog.NewRepo(pool), nil).Menu(ctx, menu.RestaurantID)
	if err != nil {
		t.Fatalf("menu: %v", err)
	}
	qty := got.Categories[0].Dishes[0].AvailableQty
	if qty != 8 {
		t.Fatalf("available after order=%d want 8", qty)
	}

	c, err := cartSvc.Get(ctx, "anna")
	if err != nil {
		t.Fatalf("cart: %v", err)
	}
	if len(c.Items) != 0 {
		t.Fatalf("cart should be empty, got %d items", len(c.Items))
	}
}

func TestCancelAfterPendingConfirmation(t *testing.T) {
	pool := testdb.Start(t)
	cfg := testdb.TestConfig()
	cfg.PartnerHTTPTimeout = 20 * time.Millisecond
	menu := testdb.SeedMenu(t, pool, "cancel-key", 10)
	dishID := menu.Categories[0].Dishes[0].ID

	cartSvc := cart.NewService(cart.NewRepo(pool), cfg.ReservationTTL)
	ordSvc := order.NewService(order.NewRepo(pool), cfg.ConfirmTimeout, order.NewNotifier(cfg.PartnerHTTPTimeout))
	ctx := context.Background()

	if _, err := cartSvc.Add(ctx, "anna", dishID, 2); err != nil {
		t.Fatalf("add: %v", err)
	}
	o, _, _, err := ordSvc.Create(ctx, "anna", "key-cancel", "", false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	cancelled, err := ordSvc.Cancel(ctx, "anna", o.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("status=%s", cancelled.Status)
	}

	got, err := catalog.NewService(catalog.NewRepo(pool), nil).Menu(ctx, menu.RestaurantID)
	if err != nil {
		t.Fatalf("menu: %v", err)
	}
	if got.Categories[0].Dishes[0].AvailableQty != 10 {
		t.Fatalf("stock should return, qty=%d", got.Categories[0].Dishes[0].AvailableQty)
	}
}
