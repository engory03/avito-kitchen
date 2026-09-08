package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/engory03/avito-kitchen/internal/config"
)

func TestProtectedRoutesRequireIdentity(t *testing.T) {
	h := NewMux(config.Config{}, nil)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/cart"},
		{http.MethodDelete, "/api/v1/cart"},
		{http.MethodPost, "/api/v1/cart/items"},
		{http.MethodPost, "/api/v1/cart/validate"},
		{http.MethodPost, "/api/v1/orders"},
		{http.MethodGet, "/api/v1/users/anna/orders"},
		{http.MethodPut, "/api/v1/partner/menu"},
		{http.MethodGet, "/api/v1/partner/orders"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
