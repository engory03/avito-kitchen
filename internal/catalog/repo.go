package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) ListRestaurants(ctx context.Context) ([]Restaurant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, cuisine, is_open, min_order_amount_cents
		FROM restaurants
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Restaurant
	for rows.Next() {
		var x Restaurant
		if err := rows.Scan(&x.ID, &x.Name, &x.Cuisine, &x.IsOpen, &x.MinOrderAmountCents); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if out == nil {
		out = []Restaurant{}
	}
	return out, rows.Err()
}

func (r *Repo) GetRestaurant(ctx context.Context, id uuid.UUID) (Restaurant, error) {
	var x Restaurant
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, cuisine, is_open, min_order_amount_cents
		FROM restaurants WHERE id = $1`, id).
		Scan(&x.ID, &x.Name, &x.Cuisine, &x.IsOpen, &x.MinOrderAmountCents)
	if errors.Is(err, pgx.ErrNoRows) {
		return Restaurant{}, ErrNotFound
	}
	return x, err
}

func (r *Repo) GetByKeyHash(ctx context.Context, hash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT id FROM restaurants WHERE api_key_hash = $1`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

func (r *Repo) LoadShowcaseMenu(ctx context.Context, restaurantID uuid.UUID) (ShowcaseMenu, error) {
	if _, err := r.GetRestaurant(ctx, restaurantID); err != nil {
		return ShowcaseMenu{}, err
	}

	catRows, err := r.pool.Query(ctx, `
		SELECT id, name, sort_order
		FROM menu_categories
		WHERE restaurant_id = $1
		ORDER BY sort_order, name`, restaurantID)
	if err != nil {
		return ShowcaseMenu{}, err
	}
	defer catRows.Close()

	type cat struct {
		id   uuid.UUID
		name string
		sort int
	}
	var cats []cat
	for catRows.Next() {
		var c cat
		if err := catRows.Scan(&c.id, &c.name, &c.sort); err != nil {
			return ShowcaseMenu{}, err
		}
		cats = append(cats, c)
	}
	if err := catRows.Err(); err != nil {
		return ShowcaseMenu{}, err
	}

	dishRows, err := r.pool.Query(ctx, `
		SELECT id, category_id, external_id, name, price_cents, is_active
		FROM dishes
		WHERE restaurant_id = $1
		ORDER BY name`, restaurantID)
	if err != nil {
		return ShowcaseMenu{}, err
	}
	defer dishRows.Close()

	byCat := map[uuid.UUID][]ShowcaseDish{}
	for dishRows.Next() {
		var d ShowcaseDish
		var catID uuid.UUID
		if err := dishRows.Scan(&d.ID, &catID, &d.ExternalID, &d.Name, &d.PriceCents, &d.IsActive); err != nil {
			return ShowcaseMenu{}, err
		}
		byCat[catID] = append(byCat[catID], d)
	}
	if err := dishRows.Err(); err != nil {
		return ShowcaseMenu{}, err
	}

	menu := ShowcaseMenu{RestaurantID: restaurantID, Categories: []ShowcaseCategory{}}
	for _, c := range cats {
		dishes := byCat[c.id]
		if dishes == nil {
			dishes = []ShowcaseDish{}
		}
		menu.Categories = append(menu.Categories, ShowcaseCategory{
			ID: c.id, Name: c.name, SortOrder: c.sort, Dishes: dishes,
		})
	}
	return menu, nil
}

func (r *Repo) AvailableQtyByRestaurant(ctx context.Context, restaurantID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id,
		       GREATEST(d.stock_qty - COALESCE((
		           SELECT SUM(h.qty)
		           FROM dish_reservations h
		           WHERE h.dish_id = d.id
		             AND h.status = 'active'
		             AND h.expires_at > NOW()
		       ), 0), 0) AS available_qty
		FROM dishes d
		WHERE d.restaurant_id = $1`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[uuid.UUID]int{}
	for rows.Next() {
		var id uuid.UUID
		var qty int
		if err := rows.Scan(&id, &qty); err != nil {
			return nil, err
		}
		out[id] = qty
	}
	return out, rows.Err()
}

func (r *Repo) UpsertMenu(ctx context.Context, keyHash string, in PartnerMenuIn) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM restaurants WHERE api_key_hash = $1`, keyHash).Scan(&id)
	open := in.Restaurant.Open()
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO restaurants (name, cuisine, is_open, min_order_amount_cents, partner_base_url, api_key_hash)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id`,
			in.Restaurant.Name, in.Restaurant.Cuisine, open,
			in.Restaurant.MinOrderAmountCents, in.Restaurant.PartnerBaseURL, keyHash,
		).Scan(&id)
		if err != nil {
			return uuid.Nil, err
		}
	} else if err != nil {
		return uuid.Nil, err
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE restaurants
			SET name=$2, cuisine=$3, is_open=$4, min_order_amount_cents=$5, partner_base_url=$6
			WHERE id=$1`,
			id, in.Restaurant.Name, in.Restaurant.Cuisine, open,
			in.Restaurant.MinOrderAmountCents, in.Restaurant.PartnerBaseURL,
		)
		if err != nil {
			return uuid.Nil, err
		}
	}

	var keptExternal []string
	for _, cat := range in.Categories {
		var catID uuid.UUID
		err = tx.QueryRow(ctx, `
			INSERT INTO menu_categories (restaurant_id, name, sort_order)
			VALUES ($1,$2,$3)
			ON CONFLICT (restaurant_id, name) DO UPDATE SET sort_order = EXCLUDED.sort_order
			RETURNING id`, id, cat.Name, cat.SortOrder).Scan(&catID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("category %s: %w", cat.Name, err)
		}
		for _, d := range cat.Dishes {
			if d.ExternalID == "" {
				return uuid.Nil, ErrBadRequest
			}
			if d.StockQty < 0 || d.PriceCents < 0 {
				return uuid.Nil, ErrBadRequest
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO dishes (restaurant_id, category_id, external_id, name, price_cents, stock_qty, is_active)
				VALUES ($1,$2,$3,$4,$5,$6,$7)
				ON CONFLICT (restaurant_id, external_id) DO UPDATE SET
					category_id = EXCLUDED.category_id,
					name = EXCLUDED.name,
					price_cents = EXCLUDED.price_cents,
					stock_qty = EXCLUDED.stock_qty,
					is_active = EXCLUDED.is_active,
					updated_at = NOW()`,
				id, catID, d.ExternalID, d.Name, d.PriceCents, d.StockQty, d.Active(),
			)
			if err != nil {
				return uuid.Nil, err
			}
			keptExternal = append(keptExternal, d.ExternalID)
		}
	}

	if len(keptExternal) == 0 {
		_, err = tx.Exec(ctx, `UPDATE dishes SET is_active=false, updated_at=NOW() WHERE restaurant_id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE dishes SET is_active=false, updated_at=NOW()
			WHERE restaurant_id=$1 AND NOT (external_id = ANY($2))`, id, keptExternal)
	}
	if err != nil {
		return uuid.Nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE dish_reservations h
		SET status = 'released'
		FROM dishes d
		WHERE h.dish_id = d.id
		  AND d.restaurant_id = $1
		  AND d.is_active = false
		  AND h.status = 'active'`, id)
	if err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (r *Repo) PatchOpen(ctx context.Context, restaurantID uuid.UUID, open bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE restaurants SET is_open=$2 WHERE id=$1`, restaurantID, open)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) PatchDish(ctx context.Context, restaurantID uuid.UUID, externalID string, stock *int, active *bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var dishID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM dishes WHERE restaurant_id=$1 AND external_id=$2`, restaurantID, externalID).Scan(&dishID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if stock != nil {
		if *stock < 0 {
			return ErrBadRequest
		}
		_, err = tx.Exec(ctx, `UPDATE dishes SET stock_qty=$2, updated_at=NOW() WHERE id=$1`, dishID, *stock)
		if err != nil {
			return err
		}
	}
	if active != nil {
		_, err = tx.Exec(ctx, `UPDATE dishes SET is_active=$2, updated_at=NOW() WHERE id=$1`, dishID, *active)
		if err != nil {
			return err
		}
		if !*active {
			_, err = tx.Exec(ctx, `
				UPDATE dish_reservations SET status='released'
				WHERE dish_id=$1 AND status='active'`, dishID)
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
