// Command kitchen-api — HTTP API площадки Авито.Кухня.
//
//go:generate go tool swag init -g main.go --dir ../.. -o ../../docs/swagger --parseInternal --exclude e2e
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/engory03/avito-kitchen/internal/app"
	"github.com/engory03/avito-kitchen/internal/config"
	"github.com/engory03/avito-kitchen/internal/migrate"
)

// @title			Авито.Кухня API
// @version		0.1.0
// @description	API площадки доставки. Пользователь: X-User-Id. Заведение: X-Api-Key. Каталог открыт; корзина, заказы и partner без заголовка — 401.
// @host			localhost:8080
// @BasePath		/
//
// @securityDefinitions.apikey	UserID
// @in							header
// @name						X-User-Id
//
// @securityDefinitions.apikey	ApiKey
// @in							header
// @name						X-Api-Key
func main() {
	_ = godotenv.Load()

	cfg := config.LoadKitchen()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := migrate.Up(ctx, pool, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations ok")
	log.Printf("reservation TTL=%s confirm timeout=%s partner HTTP timeout=%s redis=%s catalog cache TTL=%s",
		cfg.ReservationTTL, cfg.ConfirmTimeout, cfg.PartnerHTTPTimeout, cfg.RedisAddr, cfg.CatalogCacheTTL)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.NewMux(cfg, pool),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("kitchen-api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
