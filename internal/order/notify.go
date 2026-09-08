package order

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/engory03/avito-kitchen/internal/cart"
)

type Notifier struct {
	client *http.Client
}

type notifyItem struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Qty        int    `json:"qty"`
}

func NewNotifier(timeout time.Duration) *Notifier {
	return &Notifier{client: &http.Client{Timeout: timeout}}
}

func (n *Notifier) Notify(ctx context.Context, baseURL string, o Order, lines []cart.Line) error {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return fmt.Errorf("empty partner url")
	}
	ext := map[string]string{}
	for _, l := range lines {
		ext[l.DishID.String()] = l.ExternalID
	}
	items := make([]notifyItem, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, notifyItem{
			ExternalID: ext[it.DishID.String()],
			Name:       it.Name,
			Qty:        it.Qty,
		})
	}
	body, err := json.Marshal(map[string]any{
		"order_id":    o.ID,
		"status":      o.Status,
		"total_cents": o.TotalCents,
		"items":       items,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/internal/orders/notify", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("partner http %d", resp.StatusCode)
	}
	return nil
}
