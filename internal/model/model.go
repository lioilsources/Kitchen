// Package model holds the core domain types shared by all packages.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Status is the item state machine: NEW → CLAIMED → COOKING → READY → PICKED → SERVED (+ VOID, HOLD).
type Status string

const (
	StatusHold    Status = "HOLD"
	StatusNew     Status = "NEW"
	StatusClaimed Status = "CLAIMED"
	StatusCooking Status = "COOKING"
	StatusReady   Status = "READY"
	StatusPicked  Status = "PICKED"
	StatusServed  Status = "SERVED"
	StatusVoid    Status = "VOID"
)

// Active reports whether the item still belongs on a display.
func (s Status) Active() bool {
	switch s {
	case StatusHold, StatusNew, StatusClaimed, StatusCooking, StatusReady:
		return true
	}
	return false
}

// InProgress reports whether a cook is working on the item.
func (s Status) InProgress() bool { return s == StatusClaimed || s == StatusCooking }

type Order struct {
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Table     string    `json:"table"`
	Guests    int       `json:"guests,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Items     []*Item   `json:"items"`
}

type Item struct {
	ID        string     `json:"id"`
	OrderID   string     `json:"order_id"`
	Seq       int64      `json:"seq"` // global queue order
	SKU       string     `json:"sku"`
	Name      string     `json:"name"`
	Qty       int        `json:"qty"`
	Mods      []string   `json:"mods,omitempty"`
	Station   string     `json:"station"`
	Course    int        `json:"course"`
	Status    Status     `json:"status"`
	ClaimedBy string     `json:"claimed_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ChangedAt time.Time  `json:"changed_at"` // entered current status
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
	ReadyAt   *time.Time `json:"ready_at,omitempty"`
	PickedAt  *time.Time `json:"picked_at,omitempty"`
}

type Station struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Parallelism int    `json:"parallelism" yaml:"parallelism"` // how many items it cooks at once
	Speaker     string `json:"speaker,omitempty" yaml:"speaker"`
}

type PrepTime struct {
	SKU       string        `json:"sku"`
	Station   string        `json:"station"`
	P50       time.Duration `json:"p50"`
	P90       time.Duration `json:"p90"`
	N         int           `json:"n"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// Event types. The event log is the source of truth; state is rebuilt by replaying it.
const (
	EvOrderCreated = "order.created"
	EvItemStatus   = "item.status"
	EvQuiet        = "system.quiet"
)

type Event struct {
	Seq     int64          `json:"seq"`
	TS      time.Time      `json:"ts"`
	Type    string         `json:"type"`
	OrderID string         `json:"order_id,omitempty"`
	ItemID  string         `json:"item_id,omitempty"`
	Actor   string         `json:"actor,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// NewID returns a short random id with a prefix, e.g. "or_3f9a1c0b2d4e".
func NewID(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}
