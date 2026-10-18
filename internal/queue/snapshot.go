package queue

import (
	"sort"
	"time"

	"github.com/lioilsources/Kitchen/internal/eta"
	"github.com/lioilsources/Kitchen/internal/model"
)

// Snapshot is what displays render — see docs/protocol.md.
type Snapshot struct {
	Now      time.Time     `json:"now"`
	Quiet    bool          `json:"quiet"`
	Stations []StationView `json:"stations"`
	Items    []ItemView    `json:"items"`
	Tables   []TableView   `json:"tables"`
}

type StationView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ItemView struct {
	ID         string       `json:"id"`
	OrderID    string       `json:"order_id"`
	Table      string       `json:"table"`
	SKU        string       `json:"sku"`
	Name       string       `json:"name"`
	Qty        int          `json:"qty"`
	Mods       []string     `json:"mods"`
	Note       string       `json:"note"`
	Station    string       `json:"station"`
	Course     int          `json:"course"`
	Status     model.Status `json:"status"`
	ClaimedBy  string       `json:"claimed_by"`
	CreatedAt  time.Time    `json:"created_at"`
	ClaimedAt  *time.Time   `json:"claimed_at"`
	ReadyAt    *time.Time   `json:"ready_at"`
	WaitS      int          `json:"wait_s"`
	ETAMin     int          `json:"eta_min"`
	StartInMin int          `json:"start_in_min"`
	Alert      string       `json:"alert"`
}

type TableView struct {
	Table      string          `json:"table"`
	Status     string          `json:"status"`
	ETAMin     int             `json:"eta_min"`
	Course     int             `json:"course"`
	ItemsTotal int             `json:"items_total"`
	ItemsReady int             `json:"items_ready"`
	ReadyWaitS int             `json:"ready_wait_s"`
	Alert      string          `json:"alert"`
	Items      []TableItemView `json:"items"`
}

type TableItemView struct {
	Name   string       `json:"name"`
	Qty    int          `json:"qty"`
	Status model.Status `json:"status"`
}

const (
	AlertNone = "none"
	AlertWarn = "warn"
	AlertLate = "late"
)

func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshot(e.now())
}

func (e *Engine) snapshot(now time.Time) Snapshot {
	s := Snapshot{Now: now, Quiet: e.quiet, Items: []ItemView{}, Tables: []TableView{}}
	for _, st := range e.cfg.Stations {
		s.Stations = append(s.Stations, StationView{ID: st.ID, Name: st.Name})
	}
	items := e.sortedItems()
	etas := eta.Compute(items, e.stations, e.learner.Prep, now)

	for _, it := range items {
		o := e.orders[it.OrderID]
		r := etas[it.ID]
		mods := it.Mods
		if mods == nil {
			mods = []string{}
		}
		s.Items = append(s.Items, ItemView{
			ID: it.ID, OrderID: it.OrderID, Table: o.Table, SKU: it.SKU, Name: it.Name, Qty: it.Qty,
			Mods: mods, Note: o.Note, Station: it.Station, Course: it.Course, Status: it.Status,
			ClaimedBy: it.ClaimedBy, CreatedAt: it.CreatedAt, ClaimedAt: it.ClaimedAt, ReadyAt: it.ReadyAt,
			WaitS: int(now.Sub(it.ChangedAt) / time.Second), ETAMin: eta.Minutes(r.ETA),
			StartInMin: int(r.StartIn / time.Minute), Alert: e.itemAlert(it, r, now),
		})
	}
	s.Tables = e.tables(items, etas, now)
	return s
}

func (e *Engine) itemAlert(it *model.Item, r eta.Result, now time.Time) string {
	wait := now.Sub(it.ChangedAt)
	switch {
	case it.Status == model.StatusNew && wait >= e.settings.WaitToClaim.D():
		return AlertWarn
	case it.Status.InProgress() && it.ClaimedAt != nil && r.P90 > 0 &&
		now.Sub(*it.ClaimedAt) >= time.Duration(float64(r.P90)*e.settings.CookOverrunFactor):
		return AlertLate
	case it.Status == model.StatusReady && wait >= 2*e.settings.PassAging.D():
		return AlertLate
	case it.Status == model.StatusReady && wait >= e.settings.PassAging.D():
		return AlertWarn
	}
	return AlertNone
}

// tables derives the floor view: per table the current course, its ETA and pass state.
func (e *Engine) tables(items []*model.Item, etas map[string]eta.Result, now time.Time) []TableView {
	byTable := map[string][]*model.Item{}
	var order []string
	for _, it := range items {
		t := e.orders[it.OrderID].Table
		if _, ok := byTable[t]; !ok {
			order = append(order, t)
		}
		byTable[t] = append(byTable[t], it)
	}
	seen := map[string]bool{}
	out := make([]TableView, 0, len(order))
	for _, t := range order {
		seen[t] = true
		its := byTable[t]
		tv := TableView{Table: t, Alert: AlertNone, Items: []TableItemView{}}
		// Aktuální chod = nejnižší chod, který není HOLD.
		for _, it := range its {
			if it.Status != model.StatusHold && (tv.Course == 0 || it.Course < tv.Course) {
				tv.Course = it.Course
			}
		}
		if tv.Course == 0 {
			tv.Status = "hold"
			for _, it := range its {
				tv.Items = append(tv.Items, TableItemView{Name: it.Name, Qty: it.Qty, Status: it.Status})
			}
			tv.ItemsTotal = len(its)
			delete(e.etaShown, t)
			out = append(out, tv)
			continue
		}
		var maxETA time.Duration
		started := false
		var oldestReady time.Time
		for _, it := range its {
			if it.Status == model.StatusHold || it.Course != tv.Course {
				continue
			}
			tv.ItemsTotal++
			tv.Items = append(tv.Items, TableItemView{Name: it.Name, Qty: it.Qty, Status: it.Status})
			if r := etas[it.ID]; r.ETA > maxETA {
				maxETA = r.ETA
			}
			switch {
			case it.Status == model.StatusReady:
				tv.ItemsReady++
				if oldestReady.IsZero() || it.ChangedAt.Before(oldestReady) {
					oldestReady = it.ChangedAt
				}
			case it.Status.InProgress():
				started = true
			}
		}
		switch {
		case tv.ItemsReady == tv.ItemsTotal:
			tv.Status = "ready"
		case tv.ItemsReady > 0:
			tv.Status = "partial"
		case started:
			tv.Status = "cooking"
		default:
			tv.Status = "waiting"
		}
		if !oldestReady.IsZero() {
			w := now.Sub(oldestReady)
			tv.ReadyWaitS = int(w / time.Second)
			switch {
			case w >= 2*e.settings.PassAging.D():
				tv.Alert = AlertLate
			case w >= e.settings.PassAging.D():
				tv.Alert = AlertWarn
			}
		}
		tv.ETAMin = e.stableETA(t, eta.Minutes(maxETA), tv.Status == "ready", now)
		out = append(out, tv)
	}
	for t := range e.etaShown {
		if !seen[t] {
			delete(e.etaShown, t)
		}
	}
	rank := map[string]int{"ready": 0, "partial": 1, "cooking": 2, "waiting": 2, "hold": 3}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if rank[a.Status] != rank[b.Status] {
			return rank[a.Status] < rank[b.Status]
		}
		if a.Status == "ready" {
			return a.ReadyWaitS > b.ReadyWaitS
		}
		return a.ETAMin < b.ETAMin
	})
	return out
}

// stableETA keeps the floor display calm: the shown minute value changes at most once per eta_refresh,
// except when the table becomes ready (0 immediately).
func (e *Engine) stableETA(table string, min int, ready bool, now time.Time) int {
	if ready {
		min = 0
	}
	prev, ok := e.etaShown[table]
	if !ok || ready || (prev.min != min && now.Sub(prev.at) >= e.settings.ETARefresh.D()) {
		e.etaShown[table] = shownETA{min: min, at: now}
		return min
	}
	return prev.min
}
