package cache

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// CatalogCache — копия витрины. Если Redis нет, все методы просто ничего не делают.
type CatalogCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCatalog(addr string, ttl time.Duration) *CatalogCache {
	c := &CatalogCache{ttl: ttl}
	if addr == "" {
		log.Println("catalog cache: Redis addr empty, working without cache")
		return c
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("catalog cache: Redis ping failed (%v), working without cache", err)
		_ = rdb.Close()
		return c
	}
	c.rdb = rdb
	log.Printf("catalog cache: connected to %s ttl=%s", addr, ttl)
	return c
}

func (c *CatalogCache) Get(ctx context.Context, key string) ([]byte, bool) {
	if c == nil || c.rdb == nil {
		return nil, false
	}
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

func (c *CatalogCache) Set(ctx context.Context, key string, value []byte) {
	if c == nil || c.rdb == nil {
		return
	}
	_ = c.rdb.Set(ctx, key, value, c.ttl).Err()
}

func (c *CatalogCache) Del(ctx context.Context, keys ...string) {
	if c == nil || c.rdb == nil || len(keys) == 0 {
		return
	}
	_ = c.rdb.Del(ctx, keys...).Err()
}

const (
	KeyRestaurants = "catalog:restaurants"
	KeyRestaurant  = "catalog:restaurant:" // + id
	KeyMenu        = "catalog:menu:"       // + restaurant id
)
