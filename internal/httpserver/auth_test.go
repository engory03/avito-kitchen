package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUserRejectsMissingAndBlank(t *testing.T) {
	h := RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, user := range []string{"", "   "} {
		req := httptest.NewRequest(http.MethodGet, "/cart", nil)
		if user != "" {
			req.Header.Set(HeaderUserID, user)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("user %q: got %d", user, rec.Code)
		}
		var body ErrorBody
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Error != "X-User-Id required" {
			t.Fatalf("error %q", body.Error)
		}
	}
}

func TestRequireUserPassesTrimmedID(t *testing.T) {
	var got string
	h := RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = UserID(r)
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/cart", nil)
	req.Header.Set(HeaderUserID, "  anna  ")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code %d", rec.Code)
	}
	if got != "anna" {
		t.Fatalf("user %q", got)
	}
}

func TestRequireAPIKeyRejectsMissing(t *testing.T) {
	h := RequireAPIKey(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/partner/orders", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", rec.Code)
	}
}
