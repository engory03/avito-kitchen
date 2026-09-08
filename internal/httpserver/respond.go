// Package httpserver — JSON-ответы, заголовки пользователя и ключа заведения.
package httpserver

import (
	"encoding/json"
	"net/http"
)

// WriteJSON отдаёт JSON-ответ с кодом status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// ErrorBody — {"error": "message"}. Текст в error уточняет причину.
type ErrorBody struct {
	Error string `json:"error" example:"message"`
}

// HealthResponse — тело GET /health.
type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	Service string `json:"service" example:"kitchen-api"`
}

// StatusOK — короткое подтверждение partner-операций.
type StatusOK struct {
	Status string `json:"status" example:"ok"`
}

// WriteError отдаёт JSON {"error": "..."} с HTTP-кодом status.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, ErrorBody{Error: msg})
}
