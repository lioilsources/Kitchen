// Package hub fans out snapshots and announcements to display clients over WebSocket.
package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	minInterval = 250 * time.Millisecond // max 4 snapshoty/s
	heartbeat   = 5 * time.Second
	writeWait   = 5 * time.Second
)

type Message struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Hub struct {
	snapshot func() any
	mu       sync.Mutex
	clients  map[*client]struct{}
	dirty    chan struct{}
}

type client struct {
	send chan []byte
}

func New(snapshot func() any) *Hub {
	return &Hub{snapshot: snapshot, clients: map[*client]struct{}{}, dirty: make(chan struct{}, 1)}
}

// Notify marks state as changed; a snapshot goes out within minInterval.
func (h *Hub) Notify() {
	select {
	case h.dirty <- struct{}{}:
	default:
	}
}

// Send broadcasts an extra message (announce, tone) immediately.
func (h *Hub) Send(typ string, data any) {
	b, err := json.Marshal(Message{Type: typ, Data: data})
	if err != nil {
		slog.Error("hub marshal", "err", err)
		return
	}
	h.broadcast(b)
}

func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

func (h *Hub) Run(ctx context.Context) {
	hb := time.NewTicker(heartbeat)
	defer hb.Stop()
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.dirty:
			if w := minInterval - time.Since(last); w > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(w):
				}
			}
		case <-hb.C:
		}
		last = time.Now()
		h.Send("snapshot", h.snapshot())
	}
}

func (h *Hub) broadcast(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- b:
		default:
			// Pomalý klient: zahodit nejstarší, poslat nejnovější (snapshot je vždy úplný).
			select {
			case <-c.send:
			default:
			}
			select {
			case c.send <- b:
			default:
			}
		}
	}
}

// ServeHTTP upgrades to WebSocket and streams messages.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // LAN, displeje z různých originů
	if err != nil {
		return
	}
	defer conn.CloseNow()
	c := &client{send: make(chan []byte, 8)}
	first, _ := json.Marshal(Message{Type: "snapshot", Data: h.snapshot()})
	c.send <- first
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	slog.Info("display connected", "remote", r.RemoteAddr, "ua", r.UserAgent())
	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		slog.Info("display disconnected", "remote", r.RemoteAddr)
	}()

	ctx := conn.CloseRead(r.Context()) // klient nic neposílá; čtení jen obslouží ping/close
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, writeWait)
			err := conn.Write(wctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
