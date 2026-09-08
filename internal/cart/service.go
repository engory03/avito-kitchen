package cart

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repo
	ttl  time.Duration
}

func NewService(repo *Repo, ttl time.Duration) *Service {
	return &Service{repo: repo, ttl: ttl}
}

func (s *Service) Get(ctx context.Context, userID string) (Cart, error) {
	if err := requireUserID(userID); err != nil {
		return Cart{}, err
	}
	return s.repo.GetCart(ctx, userID)
}

func (s *Service) Add(ctx context.Context, userID string, dishID uuid.UUID, qty int) (Item, error) {
	if err := requireUserID(userID); err != nil {
		return Item{}, err
	}
	if qty <= 0 {
		return Item{}, ErrBadRequest
	}
	if dishID == uuid.Nil {
		return Item{}, ErrBadRequest
	}
	return s.repo.AddItem(ctx, userID, dishID, qty, s.ttl)
}

func (s *Service) Patch(ctx context.Context, userID string, itemID uuid.UUID, qty int) (Item, error) {
	if err := requireUserID(userID); err != nil {
		return Item{}, err
	}
	if qty <= 0 {
		return Item{}, ErrBadRequest
	}
	return s.repo.PatchItem(ctx, userID, itemID, qty, s.ttl)
}

func (s *Service) DeleteItem(ctx context.Context, userID string, itemID uuid.UUID) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	return s.repo.DeleteItem(ctx, userID, itemID)
}

func (s *Service) Clear(ctx context.Context, userID string) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	return s.repo.Clear(ctx, userID)
}

func (s *Service) Validate(ctx context.Context, userID string) (ValidateResult, error) {
	if err := requireUserID(userID); err != nil {
		return ValidateResult{}, err
	}
	snap, err := s.repo.Snapshot(ctx, userID)
	if err != nil {
		return ValidateResult{}, err
	}
	return Evaluate(time.Now().UTC(), snap), nil
}

func requireUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return ErrUnauthorized
	}
	return nil
}
