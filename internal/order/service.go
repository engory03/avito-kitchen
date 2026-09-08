package order

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/engory03/avito-kitchen/internal/cart"
	"github.com/engory03/avito-kitchen/internal/domain"
)

type Service struct {
	repo       *Repo
	confirmTTL time.Duration
	notify     *Notifier
}

func NewService(repo *Repo, confirmTTL time.Duration, notify *Notifier) *Service {
	return &Service{repo: repo, confirmTTL: confirmTTL, notify: notify}
}

func (s *Service) Create(ctx context.Context, userID, key, address string, acceptPartial bool) (Order, bool, cart.ValidateResult, error) {
	if strings.TrimSpace(userID) == "" {
		return Order{}, false, cart.ValidateResult{}, ErrUnauthorized
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return Order{}, false, cart.ValidateResult{}, ErrIdempotencyKey
	}

	if id, ok, err := s.repo.OrderIDByKey(ctx, key, userID); err != nil {
		return Order{}, false, cart.ValidateResult{}, err
	} else if ok {
		o, err := s.Get(ctx, userID, id)
		return o, true, cart.ValidateResult{}, err
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return Order{}, false, cart.ValidateResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	snap, err := cart.LoadSnapshotTx(ctx, tx, userID)
	if err != nil {
		return Order{}, false, cart.ValidateResult{}, err
	}
	check := cart.Evaluate(time.Now().UTC(), snap)
	switch check.Status {
	case domain.ValidateInvalid:
		return Order{}, false, check, ErrCannotCreate
	case domain.ValidatePartial:
		if !acceptPartial {
			return Order{}, false, check, ErrPartial
		}
	}

	o, err := s.repo.PlaceTx(ctx, tx, userID, key, address, time.Now().UTC().Add(s.confirmTTL), snap, check.ValidItems)
	if errors.Is(err, errIdempotencyRace) {
		_ = tx.Rollback(ctx)
		id, ok, e2 := s.repo.OrderIDByKey(ctx, key, userID)
		if e2 != nil || !ok {
			return Order{}, false, check, err
		}
		got, e2 := s.Get(ctx, userID, id)
		return got, true, cart.ValidateResult{}, e2
	}
	if err != nil {
		return Order{}, false, check, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, false, check, err
	}

	if err := s.notify.Notify(ctx, snap.PartnerBaseURL, o, snap.Lines); err != nil {
		_ = s.repo.MarkPending(ctx, o.ID)
		o, _ = s.repo.Get(ctx, o.ID)
	}
	return o, false, check, nil
}

func (s *Service) Get(ctx context.Context, userID string, id uuid.UUID) (Order, error) {
	if strings.TrimSpace(userID) == "" {
		return Order{}, ErrUnauthorized
	}
	if err := s.repo.ExpireIfNeeded(ctx, id); err != nil {
		return Order{}, err
	}
	o, err := s.repo.Get(ctx, id)
	if err != nil {
		return Order{}, err
	}
	if o.UserID != userID {
		return Order{}, ErrNotFound
	}
	return o, nil
}

func (s *Service) ListUser(ctx context.Context, headerUser, pathUser string) ([]Order, error) {
	if strings.TrimSpace(headerUser) == "" {
		return nil, ErrUnauthorized
	}
	if headerUser != pathUser {
		return nil, ErrNotFound
	}
	return s.repo.ListByUser(ctx, pathUser)
}

func (s *Service) Cancel(ctx context.Context, userID string, id uuid.UUID) (Order, error) {
	if strings.TrimSpace(userID) == "" {
		return Order{}, ErrUnauthorized
	}
	return s.repo.CancelByUser(ctx, id, userID)
}

func (s *Service) PartnerList(ctx context.Context, keyHash, status string, since *time.Time) ([]Order, error) {
	restID, err := s.repo.RestaurantIDByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}
	return s.repo.ListByRestaurant(ctx, restID, status, since)
}

func (s *Service) PartnerSetStatus(ctx context.Context, keyHash string, id uuid.UUID, to, reason string) (Order, error) {
	restID, err := s.repo.RestaurantIDByKeyHash(ctx, keyHash)
	if err != nil {
		return Order{}, err
	}
	return s.repo.PartnerSetStatus(ctx, id, restID, to, reason)
}
