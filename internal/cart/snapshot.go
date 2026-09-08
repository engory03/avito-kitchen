package cart

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/engory03/avito-kitchen/internal/domain"
)

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *Repo) Snapshot(ctx context.Context, userID string) (Snapshot, error) {
	return loadSnapshot(ctx, r.pool, userID, false)
}

// LoadSnapshotTx блокирует блюда, потом корзину — тот же порядок, что при добавлении в корзину.
func LoadSnapshotTx(ctx context.Context, tx pgx.Tx, userID string) (Snapshot, error) {
	return loadSnapshot(ctx, tx, userID, true)
}

func loadSnapshot(ctx context.Context, q querier, userID string, lock bool) (Snapshot, error) {
	if lock {
		rows, err := q.Query(ctx, `
			SELECT d.id
			FROM dishes d
			JOIN cart_items ci ON ci.dish_id = d.id
			JOIN carts c ON c.id = ci.cart_id
			WHERE c.user_id = $1
			ORDER BY d.id
			FOR UPDATE OF d`, userID)
		if err != nil {
			return Snapshot{}, err
		}
		for rows.Next() {
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return Snapshot{}, err
		}
	}

	var snap Snapshot
	err := q.QueryRow(ctx, `
		SELECT c.restaurant_id, r.is_open, r.min_order_amount_cents, r.partner_base_url
		FROM carts c
		JOIN restaurants r ON r.id = c.restaurant_id
		WHERE c.user_id = $1`+lockCartSQL(lock), userID).
		Scan(&snap.RestaurantID, &snap.IsOpen, &snap.MinOrderAmountCents, &snap.PartnerBaseURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{HasCart: false, Lines: []Line{}}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	snap.HasCart = true

	rows, err := q.Query(ctx, `
		SELECT ci.id, ci.dish_id, d.external_id, ci.qty, d.name, d.price_cents, d.is_active,
		       h.expires_at, h.status,
		       GREATEST(d.stock_qty - COALESCE((
		           SELECT SUM(h2.qty)
		           FROM dish_reservations h2
		           WHERE h2.dish_id = d.id
		             AND h2.status = $2
		             AND h2.expires_at > NOW()
		             AND h2.cart_item_id <> ci.id
		       ), 0), 0)
		FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		JOIN dishes d ON d.id = ci.dish_id
		LEFT JOIN LATERAL (
			SELECT expires_at, status
			FROM dish_reservations
			WHERE cart_item_id = ci.id AND status = $2
			ORDER BY created_at DESC
			LIMIT 1
		) h ON true
		WHERE c.user_id = $1
		ORDER BY d.id`, userID, domain.ReservationActive)
	if err != nil {
		return Snapshot{}, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	snap.Lines = []Line{}
	for rows.Next() {
		var line Line
		var expires *time.Time
		var status *string
		if err := rows.Scan(&line.CartItemID, &line.DishID, &line.ExternalID, &line.Qty, &line.Name, &line.PriceCents, &line.IsActive,
			&expires, &status, &line.AvailableForUser); err != nil {
			return Snapshot{}, err
		}
		if expires != nil && status != nil && *status == domain.ReservationActive && expires.After(now) {
			line.HoldOK = true
			line.HoldExpires = *expires
		}
		snap.Lines = append(snap.Lines, line)
	}
	return snap, rows.Err()
}

func lockCartSQL(lock bool) string {
	if lock {
		return " FOR UPDATE OF c"
	}
	return ""
}
