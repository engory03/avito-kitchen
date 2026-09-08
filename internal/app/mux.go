// Package app собирает HTTP-роутер kitchen-api.
package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "github.com/engory03/avito-kitchen/docs/swagger"
	"github.com/engory03/avito-kitchen/internal/cache"
	"github.com/engory03/avito-kitchen/internal/cart"
	"github.com/engory03/avito-kitchen/internal/catalog"
	"github.com/engory03/avito-kitchen/internal/config"
	"github.com/engory03/avito-kitchen/internal/httpserver"
	"github.com/engory03/avito-kitchen/internal/order"
)

// NewMux — те же роуты, что слушает kitchen-api. Нужен тестам и main.
func NewMux(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	catCache := cache.NewCatalog(cfg.RedisAddr, cfg.CatalogCacheTTL)
	catH := catalog.NewHandler(catalog.NewService(catalog.NewRepo(pool), catCache))
	cartH := cart.NewHandler(cart.NewService(cart.NewRepo(pool), cfg.ReservationTTL))
	ordH := order.NewHandler(order.NewService(
		order.NewRepo(pool),
		cfg.ConfirmTimeout,
		order.NewNotifier(cfg.PartnerHTTPTimeout),
	))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/health", Health)
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.PersistAuthorization(true)))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/restaurants", catH.ListRestaurants)
		r.Get("/restaurants/{id}", catH.GetRestaurant)
		r.Get("/restaurants/{id}/menu", catH.GetMenu)

		r.Group(func(r chi.Router) {
			r.Use(httpserver.RequireUser)
			r.Get("/cart", cartH.Get)
			r.Delete("/cart", cartH.Clear)
			r.Post("/cart/items", cartH.AddItem)
			r.Patch("/cart/items/{itemId}", cartH.PatchItem)
			r.Delete("/cart/items/{itemId}", cartH.DeleteItem)
			r.Post("/cart/validate", cartH.Validate)

			r.Post("/orders", ordH.Create)
			r.Get("/orders/{id}", ordH.Get)
			r.Post("/orders/{id}/cancel", ordH.Cancel)
			r.Get("/users/{userId}/orders", ordH.ListUser)
		})

		r.Group(func(r chi.Router) {
			r.Use(httpserver.RequireAPIKey)
			r.Put("/partner/menu", catH.PutMenu)
			r.Patch("/partner/restaurant", catH.PatchRestaurant)
			r.Patch("/partner/dishes/{externalId}", catH.PatchDish)
			r.Get("/partner/orders", ordH.PartnerList)
			r.Post("/partner/orders/{id}/status", ordH.PartnerSetStatus)
		})
	})
	return r
}

// Health сообщает, что процесс жив.
//
//	@Summary	Проверка живости сервиса
//	@Tags		health
//	@Produce	json
//	@Success	200	{object}	httpserver.HealthResponse
//	@Router		/health [get]
func Health(w http.ResponseWriter, _ *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, httpserver.HealthResponse{Status: "ok", Service: "kitchen-api"})
}
