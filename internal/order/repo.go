package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/engory03/avito-kitchen/internal/cart"
	"github.com/engory03/avito-kitchen/internal/domain"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *Repo) OrderIDByKey(ctx context.Context, key, userID string) (uuid.UUID, bool, error) {
	var orderID uuid.UUID
	var owner string
	err := r.pool.QueryRow(ctx, `SELECT order_id, user_id FROM idempotency_keys WHERE key = $1`, key).
		Scan(&orderID, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	if owner != userID {
		return uuid.Nil, false, ErrBadRequest
	}
	return orderID, true, nil
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (Order, error) {
	return r.get(ctx, r.pool, id, true)
}

func (r *Repo) ListByUser(ctx context.Context, userID string) ([]Order, error) {
	if err := r.expireDue(ctx, `user_id = $1`, userID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id FROM orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []Order{}
	for _, id := range ids {
		o, err := r.get(ctx, r.pool, id, false)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func (r *Repo) ListByRestaurant(ctx context.Context, restaurantID uuid.UUID, status string, since *time.Time) ([]Order, error) {
	if err := r.expireDue(ctx, `restaurant_id = $1`, restaurantID); err != nil {
		return nil, err
	}
	q := `SELECT id FROM orders WHERE restaurant_id = $1`
	args := []any{restaurantID}
	if status != "" {
		args = append(args, status)
		q += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if since != nil {
		args = append(args, *since)
		q += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}
	q += ` ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []Order{}
	for _, id := range ids {
		o, err := r.get(ctx, r.pool, id, false)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func (r *Repo) get(ctx context.Context, q querier, id uuid.UUID, withHistory bool) (Order, error) {
	var o Order
	var confirm *time.Time
	err := q.QueryRow(ctx, `
		SELECT id, user_id, restaurant_id, status, total_cents, delivery_address, confirm_until, created_at
		FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.UserID, &o.RestaurantID, &o.Status, &o.TotalCents, &o.DeliveryAddress, &confirm, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	o.ConfirmUntil = confirm

	itemRows, err := q.Query(ctx, `
		SELECT dish_id, name_snapshot, price_cents_snapshot, qty
		FROM order_items WHERE order_id = $1 ORDER BY id`, id)
	if err != nil {
		return Order{}, err
	}
	defer itemRows.Close()
	o.Items = []Item{}
	for itemRows.Next() {
		var it Item
		var dishID *uuid.UUID
		if err := itemRows.Scan(&dishID, &it.Name, &it.PriceCents, &it.Qty); err != nil {
			return Order{}, err
		}
		if dishID != nil {
			it.DishID = *dishID
		}
		o.Items = append(o.Items, it)
	}
	if err := itemRows.Err(); err != nil {
		return Order{}, err
	}
	if !withHistory {
		return o, nil
	}
	histRows, err := q.Query(ctx, `
		SELECT from_status, to_status, changed_by, COALESCE(reason, ''), created_at
		FROM order_status_history WHERE order_id = $1 ORDER BY created_at, id`, id)
	if err != nil {
		return Order{}, err
	}
	defer histRows.Close()
	o.StatusHistory = []HistoryItem{}
	for histRows.Next() {
		var h HistoryItem
		if err := histRows.Scan(&h.FromStatus, &h.ToStatus, &h.ChangedBy, &h.Reason, &h.CreatedAt); err != nil {
			return Order{}, err
		}
		o.StatusHistory = append(o.StatusHistory, h)
	}
	return o, histRows.Err()
}

func (r *Repo) PlaceTx(ctx context.Context, tx pgx.Tx, userID, key, address string, confirmUntil time.Time, snap cart.Snapshot, valid []cart.Item) (Order, error) {
	total := 0
	var chosen []cart.Line
	for _, v := range valid {
		for _, l := range snap.Lines {
			if l.CartItemID == v.CartItemID {
				chosen = append(chosen, l)
				total += l.PriceCents * l.Qty
				break
			}
		}
	}
	if len(chosen) == 0 {
		return Order{}, ErrCannotCreate
	}

	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, restaurant_id, status, total_cents, delivery_address, confirm_until)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id`,
		userID, snap.RestaurantID, domain.OrderCreated, total, address, confirmUntil).Scan(&id)
	if err != nil {
		return Order{}, err
	}

	for _, l := range chosen {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, dish_id, name_snapshot, price_cents_snapshot, qty)
			VALUES ($1,$2,$3,$4,$5)`, id, l.DishID, l.Name, l.PriceCents, l.Qty); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE dish_reservations SET status = $2
			WHERE cart_item_id = $1 AND status = $3`,
			l.CartItemID, domain.ReservationConsumed, domain.ReservationActive); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE dishes SET stock_qty = stock_qty - $2, updated_at = NOW() WHERE id = $1`,
			l.DishID, l.Qty); err != nil {
			return Order{}, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE id = $1`, l.CartItemID); err != nil {
			return Order{}, err
		}
	}

	var left int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		WHERE c.user_id = $1`, userID).Scan(&left); err != nil {
		return Order{}, err
	}
	if left == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM carts WHERE user_id = $1`, userID); err != nil {
			return Order{}, err
		}
	}

	if err := insertHistory(ctx, tx, id, nil, domain.OrderCreated, domain.ActorUser, ""); err != nil {
		return Order{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_keys (key, user_id, order_id) VALUES ($1,$2,$3)`, key, userID, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Order{}, errIdempotencyRace
		}
		return Order{}, err
	}

	return r.get(ctx, tx, id, true)
}

var errIdempotencyRace = errors.New("idempotency race")

func (r *Repo) MarkPending(ctx context.Context, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var from string
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, id).Scan(&from)
	if err != nil {
		return err
	}
	if from != domain.OrderCreated {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, id, domain.OrderPendingConfirmation); err != nil {
		return err
	}
	if err := insertHistory(ctx, tx, id, &from, domain.OrderPendingConfirmation, domain.ActorSystem, domain.ReasonPartnerTimeout2s); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) CancelByUser(ctx context.Context, id uuid.UUID, userID string) (Order, error) {
	if err := r.ExpireIfNeeded(ctx, id); err != nil {
		return Order{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var from string
	var owner string
	err = tx.QueryRow(ctx, `SELECT status, user_id FROM orders WHERE id = $1 FOR UPDATE`, id).Scan(&from, &owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if owner != userID {
		return Order{}, ErrNotFound
	}
	if !domain.UserMayCancel(from) {
		return Order{}, ErrCannotCancel
	}
	if err := restoreStock(ctx, tx, id); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, id, domain.OrderCancelled); err != nil {
		return Order{}, err
	}
	if err := insertHistory(ctx, tx, id, &from, domain.OrderCancelled, domain.ActorUser, ""); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return r.Get(ctx, id)
}

func (r *Repo) PartnerSetStatus(ctx context.Context, id, restaurantID uuid.UUID, to, reason string) (Order, error) {
	if err := r.ExpireIfNeeded(ctx, id); err != nil {
		return Order{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var from string
	var rest uuid.UUID
	err = tx.QueryRow(ctx, `SELECT status, restaurant_id FROM orders WHERE id = $1 FOR UPDATE`, id).Scan(&from, &rest)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if rest != restaurantID {
		return Order{}, ErrNotFound
	}
	if !domain.PartnerMaySet(from, to) {
		return Order{}, ErrBadTransition
	}
	if to == domain.OrderRejected {
		if err := restoreStock(ctx, tx, id); err != nil {
			return Order{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, id, to); err != nil {
		return Order{}, err
	}
	if err := insertHistory(ctx, tx, id, &from, to, domain.ActorRestaurant, reason); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return r.Get(ctx, id)
}

func (r *Repo) ExpireIfNeeded(ctx context.Context, id uuid.UUID) error {
	return r.expireDue(ctx, `id = $1`, id)
}

func (r *Repo) expireDue(ctx context.Context, cond string, arg any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id, status FROM orders
		WHERE `+cond+`
		  AND status IN ($2, $3)
		  AND confirm_until IS NOT NULL
		  AND confirm_until < NOW()
		FOR UPDATE`, arg, domain.OrderCreated, domain.OrderPendingConfirmation)
	if err != nil {
		return err
	}
	type due struct {
		id     uuid.UUID
		status string
	}
	var list []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.status); err != nil {
			rows.Close()
			return err
		}
		list = append(list, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, d := range list {
		if err := restoreStock(ctx, tx, d.id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, d.id, domain.OrderCancelled); err != nil {
			return err
		}
		from := d.status
		if err := insertHistory(ctx, tx, d.id, &from, domain.OrderCancelled, domain.ActorSystem, domain.ReasonConfirmTimeout); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repo) RestaurantIDByKeyHash(ctx context.Context, hash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE api_key_hash = $1`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

func restoreStock(ctx context.Context, tx pgx.Tx, orderID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE dishes d
		SET stock_qty = d.stock_qty + oi.qty, updated_at = NOW()
		FROM order_items oi
		WHERE oi.order_id = $1 AND oi.dish_id = d.id`, orderID)
	return err
}

func insertHistory(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, from *string, to, actor, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, from_status, to_status, changed_by, reason)
		VALUES ($1,$2,$3,$4, NULLIF($5, ''))`, orderID, from, to, actor, reason)
	return err
}
