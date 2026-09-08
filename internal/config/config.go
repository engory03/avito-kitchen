package config

import (
	"os"
	"strconv"
	"time"
)

// Config — настройки kitchen-api.
type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	MigrationsDir      string
	PartnerAPIKey      string
	PartnerBaseURL     string
	ReservationTTL     time.Duration // бронь в корзине, по умолчанию 10 минут
	ConfirmTimeout     time.Duration // ожидание подтверждения заведения, 15 минут
	PartnerHTTPTimeout time.Duration // HTTP notify, 2 секунды
	RedisAddr          string
	CatalogCacheTTL    time.Duration // копия витрины в Redis, не бронь
}

func LoadKitchen() Config {
	ttlMin := envInt("RESERVATION_TTL_MINUTES", 10)
	confirmMin := envInt("CONFIRM_TIMEOUT_MINUTES", 15)
	timeoutMS := envInt("PARTNER_HTTP_TIMEOUT_MS", 2000)
	cacheSec := envInt("CATALOG_CACHE_TTL_SECONDS", 120)

	return Config{
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		DatabaseURL:        env("DATABASE_URL", "postgres://kitchen:kitchen@localhost:5432/kitchen?sslmode=disable"),
		MigrationsDir:      env("MIGRATIONS_DIR", "migrations"),
		PartnerAPIKey:      env("PARTNER_API_KEY", "demo-partner-key"),
		PartnerBaseURL:     env("PARTNER_BASE_URL", "http://localhost:8081"),
		ReservationTTL:     time.Duration(ttlMin) * time.Minute,
		ConfirmTimeout:     time.Duration(confirmMin) * time.Minute,
		PartnerHTTPTimeout: time.Duration(timeoutMS) * time.Millisecond,
		RedisAddr:          env("REDIS_ADDR", "localhost:6379"),
		CatalogCacheTTL:    time.Duration(cacheSec) * time.Second,
	}
}

// PartnerConfig — настройки демо-заведения (partner-demo).
type PartnerConfig struct {
	HTTPAddr       string
	KitchenBaseURL string
	APIKey         string
	PublicURL      string
}

func LoadPartner() PartnerConfig {
	return PartnerConfig{
		HTTPAddr:       env("HTTP_ADDR", ":8081"),
		KitchenBaseURL: env("KITCHEN_BASE_URL", "http://localhost:8080"),
		APIKey:         env("PARTNER_API_KEY", "demo-partner-key"),
		PublicURL:      env("PARTNER_PUBLIC_URL", "http://localhost:8081"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
