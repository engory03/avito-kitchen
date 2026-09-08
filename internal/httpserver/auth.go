package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

const HeaderAPIKey = "X-Api-Key"
const HeaderUserID = "X-User-Id"
const HeaderIdempotency = "Idempotency-Key"

type ctxUserID struct{}
type ctxAPIKeyHash struct{}

// HashAPIKey — как храним ключ заведения в restaurants.api_key_hash.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

func headerValue(r *http.Request, name string) string {
	return strings.TrimSpace(r.Header.Get(name))
}

// RequireUser — X-User-Id обязателен (корзина и заказы). Пустой или пробелы → 401.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := headerValue(r, HeaderUserID)
		if user == "" {
			WriteError(w, http.StatusUnauthorized, "X-User-Id required")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAPIKey — X-Api-Key обязателен (partner). Пустой или пробелы → 401.
func RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := headerValue(r, HeaderAPIKey)
		if key == "" {
			WriteError(w, http.StatusUnauthorized, "X-Api-Key required")
			return
		}
		ctx := context.WithValue(r.Context(), ctxAPIKeyHash{}, HashAPIKey(key))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserID — id из middleware или из заголовка.
func UserID(r *http.Request) string {
	if v, ok := r.Context().Value(ctxUserID{}).(string); ok && v != "" {
		return v
	}
	return headerValue(r, HeaderUserID)
}

// APIKeyHash — sha256 ключа из middleware или из заголовка.
func APIKeyHash(r *http.Request) string {
	if v, ok := r.Context().Value(ctxAPIKeyHash{}).(string); ok && v != "" {
		return v
	}
	key := headerValue(r, HeaderAPIKey)
	if key == "" {
		return ""
	}
	return HashAPIKey(key)
}

// UserFrom пишет 401, если пользователя нет.
func UserFrom(w http.ResponseWriter, r *http.Request) (string, bool) {
	user := UserID(r)
	if user == "" {
		WriteError(w, http.StatusUnauthorized, "X-User-Id required")
		return "", false
	}
	return user, true
}

// APIKeyHashFrom пишет 401, если ключа нет.
func APIKeyHashFrom(w http.ResponseWriter, r *http.Request) (string, bool) {
	hash := APIKeyHash(r)
	if hash == "" {
		WriteError(w, http.StatusUnauthorized, "X-Api-Key required")
		return "", false
	}
	return hash, true
}
