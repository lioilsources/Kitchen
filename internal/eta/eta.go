// Package eta estimates prep times (static from menu, then learned from history) and item/table ETAs.
package eta

import (
	"sort"
	"sync"
	"time"

	"github.com/lioilsources/Kitchen/internal/config"
	"github.com/lioilsources/Kitchen/internal/model"
)

const (
	window     = 50 // počet posledních vzorků pro p50/p90
	minSamples = 5  // pod tímto počtem platí statický odhad šéfkuchaře
)

// SampleStore is the persistence the learner needs (implemented by store.Store).
type SampleStore interface {
	AddPrepSample(sku, station string, at time.Time, d time.Duration) error
	PrepSamples(sku, station string, n int) ([]time.Duration, error)
	PrepKeys() ([][2]string, error)
}

type Learner struct {
	mu      sync.RWMutex
	store   SampleStore
	menu    map[string]*config.MenuItem
	learned map[[2]string]model.PrepTime
}

func NewLearner(st SampleStore, menu map[string]*config.MenuItem) (*Learner, error) {
	l := &Learner{store: st, menu: menu, learned: map[[2]string]model.PrepTime{}}
	keys, err := st.PrepKeys()
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if err := l.recompute(k[0], k[1], time.Now()); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// Observe records a finished cook (claimed → ready).
func (l *Learner) Observe(sku, station string, at time.Time, d time.Duration) error {
	if d <= 0 || d > 2*time.Hour { // zapomenutý bump, nesmysl
		return nil
	}
	if err := l.store.AddPrepSample(sku, station, at, d); err != nil {
		return err
	}
	return l.recompute(sku, station, at)
}

func (l *Learner) recompute(sku, station string, at time.Time) error {
	ds, err := l.store.PrepSamples(sku, station, window)
	if err != nil {
		return err
	}
	pt := model.PrepTime{SKU: sku, Station: station, N: len(ds), UpdatedAt: at}
	pt.P50, pt.P90 = Quantile(ds, 0.5), Quantile(ds, 0.9)
	l.mu.Lock()
	l.learned[[2]string{sku, station}] = pt
	l.mu.Unlock()
	return nil
}

// Prep returns p50/p90 for sku at station: learned when enough samples exist, else the menu default.
func (l *Learner) Prep(sku, station string) (p50, p90 time.Duration) {
	l.mu.RLock()
	pt, ok := l.learned[[2]string{sku, station}]
	l.mu.RUnlock()
	if ok && pt.N >= minSamples {
		return pt.P50, pt.P90
	}
	if m := l.menu[sku]; m != nil {
		return m.Prep.D(), m.PrepP90.D()
	}
	return 8 * time.Minute, 12 * time.Minute
}

// Stats lists all prep times (learned and static) for the admin UI.
func (l *Learner) Stats() []Stat {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Stat
	for sku, m := range l.menu {
		s := Stat{SKU: sku, Name: m.Name, Station: m.Station, StaticP50: m.Prep.D(), StaticP90: m.PrepP90.D()}
		if pt, ok := l.learned[[2]string{sku, m.Station}]; ok {
			s.LearnedP50, s.LearnedP90, s.N = pt.P50, pt.P90, pt.N
		}
		s.Using = "static"
		if s.N >= minSamples {
			s.Using = "learned"
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SKU < out[j].SKU })
	return out
}

type Stat struct {
	SKU        string        `json:"sku"`
	Name       string        `json:"name"`
	Station    string        `json:"station"`
	StaticP50  time.Duration `json:"static_p50_ns"`
	StaticP90  time.Duration `json:"static_p90_ns"`
	LearnedP50 time.Duration `json:"learned_p50_ns"`
	LearnedP90 time.Duration `json:"learned_p90_ns"`
	N          int           `json:"n"`
	Using      string        `json:"using"`
}

// Quantile returns the q-quantile (nearest-rank) of ds; 0 for empty input.
func Quantile(ds []time.Duration, q float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := int(q*float64(len(s))+0.999999) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

// PrepFunc gives p50/p90 for an item; Learner.Prep satisfies it.
type PrepFunc func(sku, station string) (p50, p90 time.Duration)

// Result is the computed estimate for one item.
type Result struct {
	ETA     time.Duration // do READY
	StartIn time.Duration // expo: kdy začít, aby chod dorazil najednou
	P90     time.Duration
}

// Compute estimates ETA for every active item.
//
//	eta_item  = queue_ahead(station) + p50          (NEW)
//	eta_item  = claimed_at + p50 − now              (CLAIMED/COOKING, min 0)
//	queue_ahead = Σ zbývající p50 položek před ní ve frontě stanice / paralelismus
//
// Items must be sorted by Seq. HOLD items get ETA as if fired now (zobrazuje se jen informativně).
func Compute(items []*model.Item, stations map[string]model.Station, prep PrepFunc, now time.Time) map[string]Result {
	out := make(map[string]Result, len(items))
	ahead := map[string]time.Duration{} // per station: work in front, before dividing by parallelism

	// Work already in progress occupies the station first.
	for _, it := range items {
		if !it.Status.InProgress() {
			continue
		}
		p50, p90 := prep(it.SKU, it.Station)
		rem := p50
		if it.ClaimedAt != nil {
			rem = it.ClaimedAt.Add(p50).Sub(now)
		}
		if rem < 0 {
			rem = 0
		}
		ahead[it.Station] += rem
		out[it.ID] = Result{ETA: rem, P90: p90}
	}
	for _, it := range items {
		switch it.Status {
		case model.StatusNew, model.StatusHold:
			p50, p90 := prep(it.SKU, it.Station)
			par := stations[it.Station].Parallelism
			if par <= 0 {
				par = 1
			}
			q := ahead[it.Station] / time.Duration(par)
			out[it.ID] = Result{ETA: q + p50, P90: p90}
			if it.Status == model.StatusNew {
				ahead[it.Station] += p50
			}
		case model.StatusReady:
			out[it.ID] = Result{}
		}
	}

	// Expo "fire" timing: within an order+course, not-yet-started items with shorter ETA can wait.
	type key struct {
		order  string
		course int
	}
	maxETA := map[key]time.Duration{}
	for _, it := range items {
		k := key{it.OrderID, it.Course}
		if r := out[it.ID]; r.ETA > maxETA[k] {
			maxETA[k] = r.ETA
		}
	}
	for _, it := range items {
		if it.Status != model.StatusNew {
			continue
		}
		r := out[it.ID]
		r.StartIn = maxETA[key{it.OrderID, it.Course}] - r.ETA
		out[it.ID] = r
	}
	return out
}

// Minutes rounds a duration up to whole minutes for display (calm, no seconds).
func Minutes(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	return int((d + time.Minute - 1) / time.Minute)
}
