package cart

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/engory03/avito-kitchen/internal/domain"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) GetCart(ctx context.Context, userID string) (Cart, error) {
	var restaurantID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT restaurant_id FROM carts WHERE user_id = $1`, userID).Scan(&restaurantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Cart{Items: []Item{}}, nil
	}
	if err != nil {
		return Cart{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT ci.id, ci.dish_id, ci.qty, COALESCE(h.expires_at, NOW())
		FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		LEFT JOIN LATERAL (
			SELECT expires_at
			FROM dish_reservations
			WHERE cart_item_id = ci.id
			ORDER BY CASE WHEN status = 'active' THEN 0 ELSE 1 END, created_at DESC
			LIMIT 1
		) h ON true
		WHERE c.user_id = $1
		ORDER BY ci.id`, userID)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.CartItemID, &it.DishID, &it.Qty, &it.ReservationExpiresAt); err != nil {
			return Cart{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return Cart{}, err
	}
	return Cart{RestaurantID: &restaurantID, Items: items}, nil
}

func (r *Repo) AddItem(ctx context.Context, userID string, dishID uuid.UUID, qty int, ttl time.Duration) (Item, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	dish, err := lockDish(ctx, tx, dishID)
	if err != nil {
		return Item{}, err
	}
	if err := checkDishForHold(dish); err != nil {
		return Item{}, err
	}

	cartID, cartRestaurant, err := lockOrCreateCart(ctx, tx, userID, dish.RestaurantID)
	if err != nil {
		return Item{}, err
	}
	if cartRestaurant != dish.RestaurantID {
		return Item{}, ErrOtherRestaurant
	}

	itemID, oldQty, err := cartItemForDish(ctx, tx, cartID, dishID)
	if err != nil {
		return Item{}, err
	}
	newQty := oldQty + qty

	expires, err := ensureHold(ctx, tx, dish, userID, itemID, newQty, ttl)
	if err != nil {
		return Item{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Item{}, err
	}
	return Item{
		CartItemID:           itemID,
		DishID:               dishID,
		Qty:                  newQty,
		ReservationExpiresAt: expires,
	}, nil
}

func (r *Repo) PatchItem(ctx context.Context, userID string, itemID uuid.UUID, qty int, ttl time.Duration) (Item, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Сначала блюдо, потом корзина — тот же порядок, что в AddItem.
	dishID, _, err := peekCartItem(ctx, tx, userID, itemID)
	if err != nil {
		return Item{}, err
	}
	dish, err := lockDish(ctx, tx, dishID)
	if err != nil {
		return Item{}, err
	}
	_, _, currentQty, err := lockCartItem(ctx, tx, userID, itemID)
	if err != nil {
		return Item{}, err
	}
	if qty > currentQty {
		if err := checkDishForHold(dish); err != nil {
			return Item{}, err
		}
	}

	expires, err := ensureHold(ctx, tx, dish, userID, itemID, qty, ttl)
	if err != nil {
		return Item{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Item{}, err
	}
	return Item{
		CartItemID:           itemID,
		DishID:               dishID,
		Qty:                  qty,
		ReservationExpiresAt: expires,
	}, nil
}

func (r *Repo) DeleteItem(ctx context.Context, userID string, itemID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cartID, _, _, err := lockCartItem(ctx, tx, userID, itemID)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE dish_reservations SET status = $2
		WHERE cart_item_id = $1 AND status = $3`,
		itemID, domain.ReservationReleased, domain.ReservationActive); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM cart_items WHERE id = $1`, itemID); err != nil {
		return err
	}
	if err := deleteCartIfEmpty(ctx, tx, cartID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repo) Clear(ctx context.Context, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var cartID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM carts WHERE user_id = $1 FOR UPDATE`, userID).Scan(&cartID)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE dish_reservations h
		SET status = $2
		FROM cart_items ci
		WHERE ci.cart_id = $1
		  AND h.cart_item_id = ci.id
		  AND h.status = $3`,
		cartID, domain.ReservationReleased, domain.ReservationActive); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM carts WHERE id = $1`, cartID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lockDish(ctx context.Context, tx pgx.Tx, dishID uuid.UUID) (addDish, error) {
	var d addDish
	err := tx.QueryRow(ctx, `
		SELECT d.id, d.restaurant_id, d.stock_qty, d.is_active, r.is_open
		FROM dishes d
		JOIN restaurants r ON r.id = d.restaurant_id
		WHERE d.id = $1
		FOR UPDATE OF d`, dishID).Scan(&d.ID, &d.RestaurantID, &d.StockQty, &d.IsActive, &d.IsOpen)
	if errors.Is(err, pgx.ErrNoRows) {
		return addDish{}, ErrNotFound
	}
	return d, err
}

func checkDishForHold(d addDish) error {
	if !d.IsOpen {
		return ErrRestaurantClosed
	}
	if !d.IsActive {
		return ErrDishInactive
	}
	return nil
}

func lockOrCreateCart(ctx context.Context, tx pgx.Tx, userID string, restaurantID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	var cartID, cartRestaurant uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id, restaurant_id FROM carts WHERE user_id = $1 FOR UPDATE`, userID).
		Scan(&cartID, &cartRestaurant)
	if err == nil {
		return cartID, cartRestaurant, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, err
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO carts (user_id, restaurant_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING id, restaurant_id`, userID, restaurantID).Scan(&cartID, &cartRestaurant)
	if err == nil {
		return cartID, cartRestaurant, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, err
	}
	err = tx.QueryRow(ctx, `
		SELECT id, restaurant_id FROM carts WHERE user_id = $1 FOR UPDATE`, userID).
		Scan(&cartID, &cartRestaurant)
	return cartID, cartRestaurant, err
}

func cartItemForDish(ctx context.Context, tx pgx.Tx, cartID, dishID uuid.UUID) (uuid.UUID, int, error) {
	var itemID uuid.UUID
	var qty int
	err := tx.QueryRow(ctx, `
		SELECT id, qty FROM cart_items WHERE cart_id = $1 AND dish_id = $2 FOR UPDATE`,
		cartID, dishID).Scan(&itemID, &qty)
	if err == nil {
		return itemID, qty, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, err
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO cart_items (cart_id, dish_id, qty)
		VALUES ($1, $2, 1)
		RETURNING id`, cartID, dishID).Scan(&itemID)
	return itemID, 0, err
}

func peekCartItem(ctx context.Context, tx pgx.Tx, userID string, itemID uuid.UUID) (dishID uuid.UUID, qty int, err error) {
	err = tx.QueryRow(ctx, `
		SELECT ci.dish_id, ci.qty
		FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		WHERE ci.id = $1 AND c.user_id = $2`, itemID, userID).Scan(&dishID, &qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, ErrNotFound
	}
	return dishID, qty, err
}

func lockCartItem(ctx context.Context, tx pgx.Tx, userID string, itemID uuid.UUID) (cartID, dishID uuid.UUID, qty int, err error) {
	err = tx.QueryRow(ctx, `
		SELECT c.id, ci.dish_id, ci.qty
		FROM cart_items ci
		JOIN carts c ON c.id = ci.cart_id
		WHERE ci.id = $1 AND c.user_id = $2
		FOR UPDATE OF c, ci`, itemID, userID).Scan(&cartID, &dishID, &qty)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, 0, ErrNotFound
	}
	return cartID, dishID, qty, err
}

func ensureHold(ctx context.Context, tx pgx.Tx, dish addDish, userID string, itemID uuid.UUID, newQty int, ttl time.Duration) (time.Time, error) {
	if newQty <= 0 {
		return time.Time{}, ErrBadRequest
	}

	var heldOthers int
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(qty), 0)
		FROM dish_reservations
		WHERE dish_id = $1
		  AND status = $2
		  AND expires_at > NOW()
		  AND cart_item_id <> $3`,
		dish.ID, domain.ReservationActive, itemID).Scan(&heldOthers)
	if err != nil {
		return time.Time{}, err
	}
	available := dish.StockQty - heldOthers
	if available < newQty {
		return time.Time{}, ErrNoStock
	}

	expires := time.Now().UTC().Add(ttl)
	tag, err := tx.Exec(ctx, `
		UPDATE dish_reservations
		SET qty = $2, expires_at = $3, status = $4
		WHERE cart_item_id = $1 AND status = $4`,
		itemID, newQty, expires, domain.ReservationActive)
	if err != nil {
		return time.Time{}, err
	}
	if tag.RowsAffected() == 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO dish_reservations (dish_id, user_id, cart_item_id, qty, expires_at, status)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			dish.ID, userID, itemID, newQty, expires, domain.ReservationActive)
		if err != nil {
			return time.Time{}, err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE cart_items SET qty = $2 WHERE id = $1`, itemID, newQty)
	if err != nil {
		return time.Time{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE carts SET updated_at = NOW() WHERE user_id = $1`, userID)
	return expires, err
}

func deleteCartIfEmpty(ctx context.Context, tx pgx.Tx, cartID uuid.UUID) error {
	var n int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM cart_items WHERE cart_id = $1`, cartID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `DELETE FROM carts WHERE id = $1`, cartID)
	return err
}
