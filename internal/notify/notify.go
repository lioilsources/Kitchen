// Package notify pushes selected announcements to a webhook (e.g. ntfy on the LAN → phone/watch vibration).
package notify

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

type Webhook struct {
	URL    string
	Kinds  []string
	client *http.Client
}

func NewWebhook(url string, kinds []string) *Webhook {
	if len(kinds) == 0 {
		kinds = []string{"pass_aging", "complete"}
	}
	return &Webhook{URL: url, Kinds: kinds, client: &http.Client{Timeout: 3 * time.Second}}
}

// Send posts {"kind","text","table"} asynchronously; failures are only logged (notifikace jsou bonus).
func (w *Webhook) Send(kind, text, table string) {
	if w == nil || w.URL == "" || !slices.Contains(w.Kinds, kind) {
		return
	}
	b, _ := json.Marshal(map[string]string{"kind": kind, "text": text, "table": table})
	go func() {
		req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(b))
		if err != nil {
			slog.Warn("notify", "err", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Title", "Předávka")
		resp, err := w.client.Do(req)
		if err != nil {
			slog.Warn("notify", "err", err)
			return
		}
		resp.Body.Close()
	}()
}
