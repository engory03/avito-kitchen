package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/engory03/avito-kitchen/internal/cache"
)

var ErrBadRequest = errors.New("bad request")

type Service struct {
	repo  *Repo
	cache *cache.CatalogCache
}

func NewService(repo *Repo, c *cache.CatalogCache) *Service {
	return &Service{repo: repo, cache: c}
}

func (s *Service) List(ctx context.Context, cuisine string, openOnly bool) ([]Restaurant, error) {
	all, err := s.cachedRestaurants(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Restaurant, 0, len(all))
	want := strings.ToLower(strings.TrimSpace(cuisine))
	for _, r := range all {
		if want != "" && strings.ToLower(r.Cuisine) != want {
			continue
		}
		if openOnly && !r.IsOpen {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Restaurant, error) {
	key := cache.KeyRestaurant + id.String()
	if b, ok := s.cache.Get(ctx, key); ok {
		var r Restaurant
		if json.Unmarshal(b, &r) == nil {
			return r, nil
		}
	}
	r, err := s.repo.GetRestaurant(ctx, id)
	if err != nil {
		return Restaurant{}, err
	}
	if b, err := json.Marshal(r); err == nil {
		s.cache.Set(ctx, key, b)
	}
	return r, nil
}

func (s *Service) Menu(ctx context.Context, id uuid.UUID) (Menu, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return Menu{}, err
	}

	showcase, err := s.cachedShowcase(ctx, id)
	if err != nil {
		return Menu{}, err
	}

	qty, err := s.repo.AvailableQtyByRestaurant(ctx, id)
	if err != nil {
		return Menu{}, err
	}

	menu := Menu{RestaurantID: id, Categories: make([]Category, 0, len(showcase.Categories))}
	for _, c := range showcase.Categories {
		cat := Category{ID: c.ID, Name: c.Name, SortOrder: c.SortOrder, Dishes: []Dish{}}
		for _, d := range c.Dishes {
			cat.Dishes = append(cat.Dishes, Dish{
				ID: d.ID, ExternalID: d.ExternalID, Name: d.Name,
				PriceCents: d.PriceCents, IsActive: d.IsActive,
				AvailableQty: qty[d.ID],
			})
		}
		menu.Categories = append(menu.Categories, cat)
	}
	return menu, nil
}

func (s *Service) UpsertMenu(ctx context.Context, apiKey string, in PartnerMenuIn) error {
	if strings.TrimSpace(apiKey) == "" {
		return ErrBadRequest
	}
	if strings.TrimSpace(in.Restaurant.Name) == "" {
		return ErrBadRequest
	}
	id, err := s.repo.UpsertMenu(ctx, apiKey, in)
	if err != nil {
		return err
	}
	s.bust(ctx, id)
	return nil
}

func (s *Service) PatchOpen(ctx context.Context, restaurantID uuid.UUID, open bool) error {
	if err := s.repo.PatchOpen(ctx, restaurantID, open); err != nil {
		return err
	}
	s.bust(ctx, restaurantID)
	return nil
}

func (s *Service) PatchDish(ctx context.Context, restaurantID uuid.UUID, externalID string, stock *int, active *bool) error {
	if strings.TrimSpace(externalID) == "" {
		return ErrBadRequest
	}
	if stock == nil && active == nil {
		return ErrBadRequest
	}
	if err := s.repo.PatchDish(ctx, restaurantID, externalID, stock, active); err != nil {
		return err
	}
	s.bust(ctx, restaurantID)
	return nil
}

func (s *Service) RestaurantIDByKey(ctx context.Context, keyHash string) (uuid.UUID, error) {
	return s.repo.GetByKeyHash(ctx, keyHash)
}

func (s *Service) cachedRestaurants(ctx context.Context) ([]Restaurant, error) {
	if b, ok := s.cache.Get(ctx, cache.KeyRestaurants); ok {
		var list []Restaurant
		if json.Unmarshal(b, &list) == nil {
			return list, nil
		}
	}
	list, err := s.repo.ListRestaurants(ctx)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(list); err == nil {
		s.cache.Set(ctx, cache.KeyRestaurants, b)
	}
	return list, nil
}

func (s *Service) cachedShowcase(ctx context.Context, id uuid.UUID) (ShowcaseMenu, error) {
	key := cache.KeyMenu + id.String()
	if b, ok := s.cache.Get(ctx, key); ok {
		var m ShowcaseMenu
		if json.Unmarshal(b, &m) == nil {
			return m, nil
		}
	}
	m, err := s.repo.LoadShowcaseMenu(ctx, id)
	if err != nil {
		return ShowcaseMenu{}, err
	}
	if b, err := json.Marshal(m); err == nil {
		s.cache.Set(ctx, key, b)
	}
	return m, nil
}

func (s *Service) bust(ctx context.Context, restaurantID uuid.UUID) {
	s.cache.Del(ctx,
		cache.KeyRestaurants,
		cache.KeyRestaurant+restaurantID.String(),
		cache.KeyMenu+restaurantID.String(),
	)
}
