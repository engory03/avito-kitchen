// Package partnerdemo — состояние демо-заведения в памяти процесса (без своей БД).
package partnerdemo

import (
	"fmt"
	"sync"

	"github.com/engory03/avito-kitchen/internal/domain"
)

// Dish — позиция кухни заведения; stock_qty здесь не то же самое, что в Postgres площадки.
type Dish struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	StockQty   int    `json:"stock_qty"`
	IsActive   bool   `json:"is_active"`
}

// Category — раздел локального меню.
type Category struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Dishes    []Dish `json:"dishes"`
}

// Restaurant — карточка, которую демо отдаёт площадке.
type Restaurant struct {
	Name                string `json:"name"`
	Cuisine             string `json:"cuisine"`
	MinOrderAmountCents int    `json:"min_order_amount_cents"`
	IsOpen              bool   `json:"is_open"`
	PartnerBaseURL      string `json:"partner_base_url"`
}

// Menu — GET /internal/menu и тело PUT /partner/menu.
type Menu struct {
	Restaurant Restaurant `json:"restaurant"`
	Categories []Category `json:"categories"`
}

// OrderItem — позиция из notify площадки.
type OrderItem struct {
	ExternalID string `json:"external_id"`
	Qty        int    `json:"qty"`
}

// Decision — confirm или reject по своим остаткам.
type Decision struct {
	Status string
	Reason string
}

type category struct {
	name string
	sort int
	ids  []string
}

// Store держит меню, локальный сток и уже принятые заказы.
type Store struct {
	mu     sync.Mutex
	rest   Restaurant
	cats   []category
	dishes map[string]*Dish
	seen   map[string]Decision
}

// NewStore заполняет демо-меню. publicURL уходит в partner_base_url на площадке.
func NewStore(publicURL string) *Store {
	s := &Store{
		rest: Restaurant{
			Name:                "Пельменная у Авито",
			Cuisine:             "russian",
			MinOrderAmountCents: 50000,
			IsOpen:              true,
			PartnerBaseURL:      publicURL,
		},
		dishes: map[string]*Dish{},
		seen:   map[string]Decision{},
	}
	s.addCat("Горячее", 1)
	s.addDish("Горячее", Dish{ExternalID: "pelmeni", Name: "Пельмени домашние", PriceCents: 35000, StockQty: 20, IsActive: true})
	s.addDish("Горячее", Dish{ExternalID: "borscht", Name: "Борщ", PriceCents: 28000, StockQty: 15, IsActive: true})
	s.addCat("Напитки", 2)
	s.addDish("Напитки", Dish{ExternalID: "kompot", Name: "Компот", PriceCents: 12000, StockQty: 30, IsActive: true})
	return s
}

func (s *Store) addCat(name string, sort int) {
	s.cats = append(s.cats, category{name: name, sort: sort})
}

func (s *Store) addDish(catName string, d Dish) {
	cp := d
	s.dishes[d.ExternalID] = &cp
	for i := range s.cats {
		if s.cats[i].name == catName {
			s.cats[i].ids = append(s.cats[i].ids, d.ExternalID)
			return
		}
	}
}

// Menu — снимок для HTTP и для синка на площадку.
func (s *Store) Menu() Menu {
	s.mu.Lock()
	defer s.mu.Unlock()
	cats := make([]Category, 0, len(s.cats))
	for _, c := range s.cats {
		dishes := make([]Dish, 0, len(c.ids))
		for _, id := range c.ids {
			if d := s.dishes[id]; d != nil {
				dishes = append(dishes, *d)
			}
		}
		cats = append(cats, Category{Name: c.name, SortOrder: c.sort, Dishes: dishes})
	}
	return Menu{Restaurant: s.rest, Categories: cats}
}

// Decide сверяет заказ со своим стоком. Повтор того же order_id не списывает повторно.
func (s *Store) Decide(orderID string, items []OrderItem) Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if orderID == "" {
		return Decision{Status: domain.OrderRejected, Reason: "empty order_id"}
	}
	if prev, ok := s.seen[orderID]; ok {
		return prev
	}
	if len(items) == 0 {
		d := Decision{Status: domain.OrderRejected, Reason: "empty items"}
		s.seen[orderID] = d
		return d
	}
	for _, it := range items {
		if it.Qty <= 0 || it.ExternalID == "" {
			d := Decision{Status: domain.OrderRejected, Reason: "invalid item"}
			s.seen[orderID] = d
			return d
		}
		dish, ok := s.dishes[it.ExternalID]
		if !ok || !dish.IsActive {
			d := Decision{Status: domain.OrderRejected, Reason: "unknown dish " + it.ExternalID}
			s.seen[orderID] = d
			return d
		}
		if dish.StockQty < it.Qty {
			d := Decision{Status: domain.OrderRejected, Reason: "out of local stock: " + it.ExternalID}
			s.seen[orderID] = d
			return d
		}
	}
	for _, it := range items {
		s.dishes[it.ExternalID].StockQty -= it.Qty
	}
	d := Decision{Status: domain.OrderConfirmed}
	s.seen[orderID] = d
	return d
}

// SetStock меняет только локальный остаток (площадку не трогает).
func (s *Store) SetStock(externalID string, qty int) error {
	if qty < 0 {
		return fmt.Errorf("stock_qty must be >= 0")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dishes[externalID]
	if !ok {
		return fmt.Errorf("dish not found")
	}
	d.StockQty = qty
	return nil
}
