package domain

import "testing"

func TestUserMayCancel(t *testing.T) {
	if !UserMayCancel(OrderCreated) || !UserMayCancel(OrderPendingConfirmation) {
		t.Fatal("cancel must work before confirmed")
	}
	for _, s := range []string{OrderConfirmed, OrderCooking, OrderDelivering, OrderCompleted, OrderCancelled, OrderRejected} {
		if UserMayCancel(s) {
			t.Fatalf("cancel must be blocked from %s", s)
		}
	}
}

func TestPartnerMaySet(t *testing.T) {
	ok := [][2]string{
		{OrderCreated, OrderConfirmed},
		{OrderCreated, OrderRejected},
		{OrderPendingConfirmation, OrderConfirmed},
		{OrderPendingConfirmation, OrderRejected},
		{OrderConfirmed, OrderCooking},
		{OrderCooking, OrderDelivering},
		{OrderDelivering, OrderCompleted},
	}
	for _, p := range ok {
		if !PartnerMaySet(p[0], p[1]) {
			t.Fatalf("expected allow %s → %s", p[0], p[1])
		}
	}
	bad := [][2]string{
		{OrderCreated, OrderCooking},
		{OrderConfirmed, OrderDelivering},
		{OrderConfirmed, OrderCancelled},
		{OrderCooking, OrderConfirmed},
		{OrderCompleted, OrderCooking},
		{OrderCancelled, OrderConfirmed},
		{OrderRejected, OrderConfirmed},
	}
	for _, p := range bad {
		if PartnerMaySet(p[0], p[1]) {
			t.Fatalf("expected deny %s → %s", p[0], p[1])
		}
	}
}

func TestConfirmDeadlineApplies(t *testing.T) {
	if !ConfirmDeadlineApplies(OrderCreated) || !ConfirmDeadlineApplies(OrderPendingConfirmation) {
		t.Fatal("deadline must apply while waiting for restaurant")
	}
	if ConfirmDeadlineApplies(OrderConfirmed) {
		t.Fatal("deadline must not cancel after confirmed")
	}
}
