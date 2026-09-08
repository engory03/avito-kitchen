-- +migrate Up
-- Таблицы Авито.Кухня

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE restaurants (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                   TEXT NOT NULL,
    cuisine                TEXT NOT NULL DEFAULT '',
    is_open                BOOLEAN NOT NULL DEFAULT TRUE,
    min_order_amount_cents INTEGER NOT NULL DEFAULT 0 CHECK (min_order_amount_cents >= 0),
    partner_base_url       TEXT NOT NULL DEFAULT '',
    api_key_hash           TEXT NOT NULL DEFAULT '',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE menu_categories (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    sort_order    INT NOT NULL DEFAULT 0,
    UNIQUE (restaurant_id, name)
);

CREATE TABLE dishes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    category_id   UUID NOT NULL REFERENCES menu_categories(id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,
    name          TEXT NOT NULL,
    price_cents   INT NOT NULL CHECK (price_cents >= 0),
    stock_qty     INT NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (restaurant_id, external_id)
);

-- Одна корзина на пользователя и сразу одно заведение
CREATE TABLE carts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       TEXT NOT NULL,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id)
);

CREATE TABLE cart_items (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    dish_id UUID NOT NULL REFERENCES dishes(id) ON DELETE CASCADE,
    qty     INT NOT NULL CHECK (qty > 0),
    UNIQUE (cart_id, dish_id)
);

-- Бронь блюда в корзине на ограниченное время (TTL 10 минут в приложении)
CREATE TABLE dish_reservations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dish_id      UUID NOT NULL REFERENCES dishes(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL,
    cart_item_id UUID NOT NULL REFERENCES cart_items(id) ON DELETE CASCADE,
    qty          INT NOT NULL CHECK (qty > 0),
    expires_at   TIMESTAMPTZ NOT NULL,
    status       TEXT NOT NULL CHECK (status IN ('active', 'expired', 'consumed', 'released')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX dish_reservations_active_idx
    ON dish_reservations (dish_id, status, expires_at)
    WHERE status = 'active';

CREATE INDEX dish_reservations_cart_item_idx
    ON dish_reservations (cart_item_id)
    WHERE status = 'active';

CREATE TABLE orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           TEXT NOT NULL,
    restaurant_id     UUID NOT NULL REFERENCES restaurants(id),
    status            TEXT NOT NULL CHECK (status IN (
                          'created',
                          'pending_confirmation',
                          'confirmed',
                          'cooking',
                          'delivering',
                          'completed',
                          'cancelled',
                          'rejected'
                      )),
    total_cents       INT NOT NULL CHECK (total_cents >= 0),
    -- строка для отображения, не логистика курьера
    delivery_address  TEXT NOT NULL DEFAULT '',
    -- до какого момента ждём подтверждение заведения (15 минут в приложении)
    confirm_until     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX orders_user_id_idx ON orders (user_id);
CREATE INDEX orders_restaurant_status_idx ON orders (restaurant_id, status, created_at);
CREATE INDEX orders_waiting_confirm_idx
    ON orders (confirm_until)
    WHERE status IN ('created', 'pending_confirmation');

CREATE TABLE order_items (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id              UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    dish_id               UUID REFERENCES dishes(id) ON DELETE SET NULL,
    name_snapshot         TEXT NOT NULL,
    price_cents_snapshot  INT NOT NULL,
    qty                   INT NOT NULL CHECK (qty > 0)
);

CREATE TABLE order_status_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status   TEXT NOT NULL,
    changed_by  TEXT NOT NULL CHECK (changed_by IN ('user', 'restaurant', 'system')),
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX order_status_history_order_idx
    ON order_status_history (order_id, created_at);

-- Повторный POST /orders с тем же ключом не создаёт второй заказ
CREATE TABLE idempotency_keys (
    key           TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL,
    request_hash  TEXT,
    order_id      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
