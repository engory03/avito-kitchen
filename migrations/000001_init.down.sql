-- +migrate Down
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS order_status_history;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS dish_reservations;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
DROP TABLE IF EXISTS dishes;
DROP TABLE IF EXISTS menu_categories;
DROP TABLE IF EXISTS restaurants;
