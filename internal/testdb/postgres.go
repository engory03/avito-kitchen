package testdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/engory03/avito-kitchen/internal/catalog"
	"github.com/engory03/avito-kitchen/internal/config"
	"github.com/engory03/avito-kitchen/internal/httpserver"
	"github.com/engory03/avito-kitchen/internal/migrate"
)

func TestConfig() config.Config {
	return config.Config{
		ReservationTTL:     10 * time.Minute,
		ConfirmTimeout:     15 * time.Minute,
		PartnerHTTPTimeout: 80 * time.Millisecond,
		CatalogCacheTTL:    time.Second,
	}
}

func Start(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("kitchen"),
		postgres.WithUsername("kitchen"),
		postgres.WithPassword("kitchen"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("postgres testcontainer: %v (Docker required)", err)
	}
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := migrate.Up(ctx, pool, migrationsDir()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return pool
}

func SeedMenu(t *testing.T, pool *pgxpool.Pool, apiKey string, stock int) catalog.Menu {
	t.Helper()
	open := true
	in := catalog.PartnerMenuIn{
		Restaurant: catalog.PartnerRestaurantIn{
			Name:                "Test Pelmennaya",
			Cuisine:             "russian",
			MinOrderAmountCents: 50000,
			IsOpen:              &open,
		},
		Categories: []catalog.PartnerCategoryIn{{
			Name:      "Hot",
			SortOrder: 1,
			Dishes: []catalog.PartnerDishIn{{
				ExternalID: "pelmeni",
				Name:       "Pelmeni",
				PriceCents: 35000,
				StockQty:   stock,
			}},
		}},
	}
	svc := catalog.NewService(catalog.NewRepo(pool), nil)
	if err := svc.UpsertMenu(context.Background(), httpserver.HashAPIKey(apiKey), in); err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	list, err := svc.List(context.Background(), "", false)
	if err != nil || len(list) == 0 {
		t.Fatalf("list after seed: %v n=%d", err, len(list))
	}
	menu, err := svc.Menu(context.Background(), list[0].ID)
	if err != nil {
		t.Fatalf("menu after seed: %v", err)
	}
	return menu
}

func migrationsDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "migrations"
	}
	for i := 0; i < 8; i++ {
		p := filepath.Join(dir, "migrations")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		dir = filepath.Dir(dir)
	}
	return "migrations"
}
