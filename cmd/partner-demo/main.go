package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/engory03/avito-kitchen/internal/config"
	"github.com/engory03/avito-kitchen/internal/httpserver"
	"github.com/engory03/avito-kitchen/internal/partnerdemo"
)

func main() {
	_ = godotenv.Load()

	cfg := config.LoadPartner()
	store := partnerdemo.NewStore(cfg.PublicURL)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "partner-demo"})
	})

	r.Get("/internal/menu", func(w http.ResponseWriter, _ *http.Request) {
		httpserver.WriteJSON(w, http.StatusOK, store.Menu())
	})

	r.Patch("/internal/dishes/{externalId}", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			StockQty *int `json:"stock_qty"`
		}
		if err := httpserver.DecodeJSON(req, &body); err != nil || body.StockQty == nil {
			httpserver.WriteError(w, http.StatusBadRequest, "stock_qty required")
			return
		}
		if err := store.SetStock(chi.URLParam(req, "externalId"), *body.StockQty); err != nil {
			httpserver.WriteError(w, http.StatusNotFound, err.Error())
			return
		}
		httpserver.WriteJSON(w, http.StatusOK, httpserver.StatusOK{Status: "ok"})
	})

	r.Post("/internal/orders/notify", func(w http.ResponseWriter, req *http.Request) {
		var payload struct {
			OrderID string                  `json:"order_id"`
			Items   []partnerdemo.OrderItem `json:"items"`
		}
		_ = json.NewDecoder(req.Body).Decode(&payload)
		httpserver.WriteJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
		go applyDecision(cfg, store, payload.OrderID, payload.Items)
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("partner-demo listening on %s (kitchen=%s)", cfg.HTTPAddr, cfg.KitchenBaseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	go syncMenuToKitchen(cfg, store)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func syncMenuToKitchen(cfg config.PartnerConfig, store *partnerdemo.Store) {
	body, err := json.Marshal(store.Menu())
	if err != nil {
		log.Printf("menu json: %v", err)
		return
	}
	url := cfg.KitchenBaseURL + "/api/v1/partner/menu"
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 1; i <= 20; i++ {
		req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			log.Printf("sync menu: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", cfg.APIKey)

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode < 300 {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			log.Println("menu synced to kitchen-api")
			return
		}
		if resp != nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			log.Printf("sync menu attempt %d: http %d %s", i, resp.StatusCode, string(b))
		} else {
			log.Printf("sync menu attempt %d: %v", i, err)
		}
		time.Sleep(time.Second)
	}
	log.Println("sync menu: kitchen-api did not accept menu, catalog stays empty until retry")
}

func applyDecision(cfg config.PartnerConfig, store *partnerdemo.Store, orderID string, items []partnerdemo.OrderItem) {
	d := store.Decide(orderID, items)
	if orderID == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{"status": d.Status, "reason": d.Reason})
	url := cfg.KitchenBaseURL + "/api/v1/partner/orders/" + orderID + "/status"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Printf("partner status: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", cfg.APIKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("partner status: %v", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Printf("partner status http %d %s", resp.StatusCode, string(b))
		return
	}
	log.Printf("order %s -> %s %s", orderID, d.Status, d.Reason)
}
