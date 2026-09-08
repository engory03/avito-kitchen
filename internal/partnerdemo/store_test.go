package partnerdemo

import "testing"

func TestDecideConfirmsAndDecrements(t *testing.T) {
	s := NewStore("http://partner")
	d := s.Decide("ord-1", []OrderItem{{ExternalID: "pelmeni", Qty: 2}})
	if d.Status != "confirmed" {
		t.Fatalf("status=%s reason=%s", d.Status, d.Reason)
	}
	stock := stockOf(s, "pelmeni")
	if stock != 18 {
		t.Fatalf("local stock=%d want 18", stock)
	}
}

func TestDecideRejectsWhenLocalStockZero(t *testing.T) {
	s := NewStore("http://partner")
	if err := s.SetStock("pelmeni", 0); err != nil {
		t.Fatal(err)
	}
	d := s.Decide("ord-2", []OrderItem{{ExternalID: "pelmeni", Qty: 1}})
	if d.Status != "rejected" {
		t.Fatalf("status=%s", d.Status)
	}
	if stockOf(s, "pelmeni") != 0 {
		t.Fatalf("reject must not decrement")
	}
}

func TestDecideIsIdempotent(t *testing.T) {
	s := NewStore("http://partner")
	first := s.Decide("ord-3", []OrderItem{{ExternalID: "kompot", Qty: 5}})
	second := s.Decide("ord-3", []OrderItem{{ExternalID: "kompot", Qty: 5}})
	if first.Status != "confirmed" || second.Status != "confirmed" {
		t.Fatalf("first=%v second=%v", first, second)
	}
	if stockOf(s, "kompot") != 25 {
		t.Fatalf("double notify must not double-decrement, stock=%d", stockOf(s, "kompot"))
	}
}

func TestMenuExposesLiveStock(t *testing.T) {
	s := NewStore("http://partner")
	_ = s.SetStock("borscht", 1)
	menu := s.Menu()
	if menu.Restaurant.PartnerBaseURL != "http://partner" {
		t.Fatalf("url=%s", menu.Restaurant.PartnerBaseURL)
	}
	got := 0
	for _, c := range menu.Categories {
		for _, d := range c.Dishes {
			if d.ExternalID == "borscht" {
				got = d.StockQty
			}
		}
	}
	if got != 1 {
		t.Fatalf("menu stock=%d", got)
	}
}

func stockOf(s *Store, id string) int {
	for _, c := range s.Menu().Categories {
		for _, d := range c.Dishes {
			if d.ExternalID == id {
				return d.StockQty
			}
		}
	}
	return -1
}
