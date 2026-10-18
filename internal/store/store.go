// Package store persists the append-only event log, prep-time samples and settings in SQLite.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lioilsources/Kitchen/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS events (
  seq      INTEGER PRIMARY KEY AUTOINCREMENT,
  ts       INTEGER NOT NULL,           -- unix ms
  type     TEXT NOT NULL,
  order_id TEXT,
  item_id  TEXT,
  actor    TEXT,
  payload  TEXT
);
CREATE INDEX IF NOT EXISTS events_ts ON events(ts);
CREATE TABLE IF NOT EXISTS prep_samples (
  sku     TEXT NOT NULL,
  station TEXT NOT NULL,
  ts      INTEGER NOT NULL,
  dur_ms  INTEGER NOT NULL,
  hour    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS prep_samples_key ON prep_samples(sku, station, ts);
CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY, v TEXT NOT NULL);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Append writes an event and fills in its Seq.
func (s *Store) Append(e *model.Event) error {
	var payload []byte
	if e.Payload != nil {
		var err error
		if payload, err = json.Marshal(e.Payload); err != nil {
			return err
		}
	}
	res, err := s.db.Exec(`INSERT INTO events(ts,type,order_id,item_id,actor,payload) VALUES(?,?,?,?,?,?)`,
		e.TS.UnixMilli(), e.Type, e.OrderID, e.ItemID, e.Actor, string(payload))
	if err != nil {
		return err
	}
	e.Seq, err = res.LastInsertId()
	return err
}

// Events streams events with ts >= since in order.
func (s *Store) Events(since time.Time, fn func(*model.Event) error) error {
	rows, err := s.db.Query(`SELECT seq,ts,type,COALESCE(order_id,''),COALESCE(item_id,''),COALESCE(actor,''),COALESCE(payload,'') FROM events WHERE ts >= ? ORDER BY seq`, since.UnixMilli())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e model.Event
		var ts int64
		var payload string
		if err := rows.Scan(&e.Seq, &ts, &e.Type, &e.OrderID, &e.ItemID, &e.Actor, &payload); err != nil {
			return err
		}
		e.TS = time.UnixMilli(ts)
		if payload != "" {
			if err := json.Unmarshal([]byte(payload), &e.Payload); err != nil {
				return fmt.Errorf("event %d: %w", e.Seq, err)
			}
		}
		if err := fn(&e); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) AddPrepSample(sku, station string, at time.Time, d time.Duration) error {
	_, err := s.db.Exec(`INSERT INTO prep_samples(sku,station,ts,dur_ms,hour) VALUES(?,?,?,?,?)`,
		sku, station, at.UnixMilli(), d.Milliseconds(), at.Hour())
	return err
}

// PrepSamples returns the most recent n durations for sku+station (newest first).
func (s *Store) PrepSamples(sku, station string, n int) ([]time.Duration, error) {
	rows, err := s.db.Query(`SELECT dur_ms FROM prep_samples WHERE sku=? AND station=? ORDER BY ts DESC LIMIT ?`, sku, station, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Duration
	for rows.Next() {
		var ms int64
		if err := rows.Scan(&ms); err != nil {
			return nil, err
		}
		out = append(out, time.Duration(ms)*time.Millisecond)
	}
	return out, rows.Err()
}

// PrepKeys lists all sku+station pairs that have samples.
func (s *Store) PrepKeys() ([][2]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT sku, station FROM prep_samples`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][2]string
	for rows.Next() {
		var k [2]string
		if err := rows.Scan(&k[0], &k[1]); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) GetJSON(k string, v any) (bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT v FROM kv WHERE k=?`, k).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), v)
}

func (s *Store) PutJSON(k string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO kv(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, string(b))
	return err
}
