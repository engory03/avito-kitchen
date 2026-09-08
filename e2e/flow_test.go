package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/engory03/avito-kitchen/internal/app"
	"github.com/engory03/avito-kitchen/internal/testdb"
)

func TestHappyPathMenuCartOrder(t *testing.T) {
	base := os.Getenv("E2E_BASE_URL")
	apiKey := "e2e-partner-key"
	if base == "" {
		pool := testdb.Start(t)
		srv := httptest.NewServer(app.NewMux(testdb.TestConfig(), pool))
		t.Cleanup(srv.Close)
		base = srv.URL
	}

	client := &http.Client{}
	putMenu(t, client, base, apiKey)

	restID := firstRestaurantID(t, client, base)
	dishID := firstDishID(t, client, base, restID)

	addCart(t, client, base, "anna", dishID, 2)
	validateOK(t, client, base, "anna")
	orderID := createOrder(t, client, base, "anna", "e2e-idem-1")
	got := getOrder(t, client, base, "anna", orderID)
	if got["id"] != orderID {
		t.Fatalf("order id mismatch")
	}
	replay := createOrder(t, client, base, "anna", "e2e-idem-1")
	if replay != orderID {
		t.Fatalf("idempotency: %s vs %s", replay, orderID)
	}
}

func putMenu(t *testing.T, client *http.Client, base, apiKey string) {
	t.Helper()
	body := []byte(`{
		"restaurant": {"name": "E2E Pelmennaya", "cuisine": "russian", "min_order_amount_cents": 50000, "is_open": true},
		"categories": [{"name": "Hot", "sort_order": 1, "dishes": [
			{"external_id": "pelmeni", "name": "Pelmeni", "price_cents": 35000, "stock_qty": 10, "is_active": true}
		]}]
	}`)
	req, _ := http.NewRequest(http.MethodPut, base+"/api/v1/partner/menu", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("put menu: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("put menu http %d %s", resp.StatusCode, b)
	}
}

func firstRestaurantID(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	resp, err := client.Get(base + "/api/v1/restaurants")
	if err != nil {
		t.Fatalf("restaurants: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode restaurants: %v", err)
	}
	for _, it := range out.Items {
		if it.Name == "E2E Pelmennaya" {
			return it.ID
		}
	}
	if len(out.Items) == 0 {
		t.Fatal("no restaurants")
	}
	return out.Items[0].ID
}

func firstDishID(t *testing.T, client *http.Client, base, restID string) string {
	t.Helper()
	resp, err := client.Get(base + "/api/v1/restaurants/" + restID + "/menu")
	if err != nil {
		t.Fatalf("menu: %v", err)
	}
	defer resp.Body.Close()
	var menu struct {
		Categories []struct {
			Dishes []struct {
				ID string `json:"id"`
			} `json:"dishes"`
		} `json:"categories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&menu); err != nil {
		t.Fatalf("decode menu: %v", err)
	}
	if len(menu.Categories) == 0 || len(menu.Categories[0].Dishes) == 0 {
		t.Fatal("no dishes")
	}
	return menu.Categories[0].Dishes[0].ID
}

func addCart(t *testing.T, client *http.Client, base, user, dishID string, qty int) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"dish_id": dishID, "qty": qty})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/cart/items", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", user)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("add cart: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("add cart http %d %s", resp.StatusCode, b)
	}
}

func validateOK(t *testing.T, client *http.Client, base, user string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/cart/validate", nil)
	req.Header.Set("X-User-Id", user)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode validate: %v", err)
	}
	if out.Status != "ok" {
		t.Fatalf("validate status=%s", out.Status)
	}
}

func createOrder(t *testing.T, client *http.Client, base, user, key string) string {
	t.Helper()
	body := []byte(`{"accept_partial":true,"delivery_address":"e2e street"}`)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", user)
	req.Header.Set("Idempotency-Key", key)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create order http %d %s", resp.StatusCode, b)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	return out.ID
}

func getOrder(t *testing.T, client *http.Client, base, user, id string) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/orders/"+id, nil)
	req.Header.Set("X-User-Id", user)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("get order http %d %s", resp.StatusCode, b)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode get order: %v", err)
	}
	return out
}
