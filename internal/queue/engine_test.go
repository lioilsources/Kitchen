package queue

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lioilsources/Kitchen/internal/config"
	"github.com/lioilsources/Kitchen/internal/eta"
	"github.com/lioilsources/Kitchen/internal/model"
	"github.com/lioilsources/Kitchen/internal/store"
	"github.com/lioilsources/Kitchen/internal/tts"
)

type fakeVoice struct {
	mu    sync.Mutex
	said  []tts.Announcement
	tones int
}

func (v *fakeVoice) Say(a tts.Announcement)  { v.mu.Lock(); v.said = append(v.said, a); v.mu.Unlock() }
func (v *fakeVoice) Repeat()                 {}
func (v *fakeVoice) Tone(context.Context)    { v.mu.Lock(); v.tones++; v.mu.Unlock() }
func (v *fakeVoice) SetQuiet(bool)           {}
func (v *fakeVoice) SetMinGap(time.Duration) {}

func (v *fakeVoice) texts() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	for _, a := range v.said {
		out = append(out, a.Text)
	}
	return out
}

func testConfig() *config.Config {
	c := config.Default()
	c.Stations = []model.Station{{ID: "gril", Name: "Gril", Parallelism: 2}, {ID: "teple", Name: "Teplá", Parallelism: 2}}
	c.Menu = []config.MenuItem{
		{SKU: "polevka", Name: "Polévka", Station: "teple", Prep: config.Duration(2 * time.Minute), Course: 1,
			Say: config.Say{One: "polévka", Few: "polévky", Many: "polévek", Gender: "f"}},
		{SKU: "svickova", Name: "Svíčková", Station: "teple", Prep: config.Duration(6 * time.Minute), Course: 2,
			Aliases: []string{"svíčková na smetaně"}, Say: config.Say{One: "svíčková", Few: "svíčkové", Many: "svíčkových", Gender: "f"}},
		{SKU: "rizek", Name: "Řízek", Station: "gril", Prep: config.Duration(9 * time.Minute), Course: 2,
			Say: config.Say{One: "řízek", Few: "řízky", Many: "řízků", Gender: "m"}},
	}
	if err := c.Validate(); err != nil {
		panic(err)
	}
	return c
}

type harness struct {
	t    *testing.T
	e    *Engine
	v    *fakeVoice
	now  time.Time
	st   *store.Store
	path string
	cfg  *config.Config
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, now: time.Date(2026, 10, 18, 12, 0, 0, 0, time.Local), cfg: testConfig()}
	h.path = filepath.Join(t.TempDir(), "t.db")
	h.open()
	return h
}

func (h *harness) open() {
	st, err := store.Open(h.path)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { st.Close() })
	l, err := eta.NewLearner(st, h.cfg.MenuBySKU())
	if err != nil {
		h.t.Fatal(err)
	}
	h.v = &fakeVoice{}
	e, err := NewWithClock(h.cfg, st, l, h.v, func() time.Time { return h.now })
	if err != nil {
		h.t.Fatal(err)
	}
	h.e, h.st = e, st
}

func (h *harness) advance(d time.Duration) { h.now = h.now.Add(d) }

func (h *harness) do(id, action string) {
	h.t.Helper()
	if _, err := h.e.Transition(id, action, "test"); err != nil {
		h.t.Fatalf("%s %s: %v", action, id, err)
	}
}

func itemBySKU(o *model.Order, sku string) string {
	for _, it := range o.Items {
		if it.SKU == sku {
			return it.ID
		}
	}
	return ""
}

func TestServiceFlow(t *testing.T) {
	h := newHarness(t)
	o, err := h.e.AddOrder(NewOrder{Table: "7", Source: "test", Items: []OrderLine{
		{SKU: "polevka", Qty: 1},
		{Name: "Svíčková na smetaně", Qty: 2, Mods: []string{"bez brusinek"}}, // alias, chod 2
		{SKU: "rizek", Qty: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := h.v.texts(); len(got) != 1 || got[0] != "Stůl sedm: jedna polévka." {
		t.Fatalf("announce = %q", got)
	}
	s := h.e.Snapshot()
	if len(s.Items) != 3 || len(s.Tables) != 1 {
		t.Fatalf("snapshot items=%d tables=%d", len(s.Items), len(s.Tables))
	}
	if s.Tables[0].Course != 1 || s.Tables[0].Status != "waiting" {
		t.Errorf("table = %+v", s.Tables[0])
	}
	for _, it := range s.Items {
		if it.Course == 2 && it.Status != model.StatusHold {
			t.Errorf("course 2 item %s not on hold", it.Name)
		}
	}

	// Nikdo si polévku nevzal 2 minuty → urgence (jen jednou).
	h.advance(2*time.Minute + time.Second)
	h.e.Tick()
	h.advance(time.Minute)
	h.e.Tick()
	if got := h.v.texts(); len(got) != 2 || got[1] != "Stůl sedm, teplá, čeká na kuchaře." {
		t.Fatalf("after wait: %q", got)
	}

	pol := itemBySKU(o, "polevka")
	h.do(pol, ActBump) // claim
	if h.v.tones != 1 {
		t.Errorf("tones = %d", h.v.tones)
	}
	h.advance(2 * time.Minute)
	h.do(pol, ActBump) // ready
	if got := h.v.texts(); got[len(got)-1] != "Stůl sedm, kompletní." {
		t.Errorf("complete not announced: %q", got)
	}
	if s := h.e.Snapshot(); s.Tables[0].Status != "ready" {
		t.Errorf("table status = %s", s.Tables[0].Status)
	}

	// Leží na předávce 3 minuty → hlas.
	h.advance(3 * time.Minute)
	h.e.Tick()
	if got := h.v.texts(); got[len(got)-1] != "Stůl sedm hotovo, na předávce tři minuty." {
		t.Errorf("aging: %q", got)
	}

	// Vyzvednuto → hlavní chod se uvolní automaticky, až bude čas (course_gap 10m − nejdelší p50 9m).
	if n, err := h.e.TableAction("7", ActPick, "expo"); err != nil || n != 1 {
		t.Fatalf("pick: n=%d err=%v", n, err)
	}
	h.advance(30 * time.Second)
	h.e.Tick()
	if s := h.e.Snapshot(); s.Tables[0].Status != "hold" {
		t.Fatalf("expected hold, got %+v", s.Tables[0])
	}
	h.advance(time.Minute)
	h.e.Tick()
	s = h.e.Snapshot()
	if s.Tables[0].Status != "waiting" || s.Tables[0].Course != 2 {
		t.Fatalf("after fire: %+v", s.Tables[0])
	}
	if got := h.v.texts(); got[len(got)-1] != "Stůl sedm, připravit hlavní." {
		t.Errorf("fire: %q", got)
	}

	// Restart: stav se obnoví z event logu.
	h.open()
	s = h.e.Snapshot()
	if len(s.Items) != 2 || s.Tables[0].Course != 2 {
		t.Fatalf("after replay: items=%d %+v", len(s.Items), s.Tables)
	}
	for _, it := range s.Items {
		if it.Status != model.StatusNew {
			t.Errorf("replayed %s status %s", it.Name, it.Status)
		}
		if it.Name == "Svíčková" && (len(it.Mods) != 1 || it.Qty != 2) {
			t.Errorf("replayed svickova = %+v", it)
		}
	}
}

func TestInvalidTransition(t *testing.T) {
	h := newHarness(t)
	o, _ := h.e.AddOrder(NewOrder{Table: "1", Items: []OrderLine{{SKU: "rizek", Qty: 1}}})
	if _, err := h.e.Transition(o.Items[0].ID, ActPick, ""); err == nil || !strings.Contains(err.Error(), "invalid transition") {
		t.Errorf("pick NEW: err = %v", err)
	}
	if _, err := h.e.Transition("nope", ActBump, ""); err != ErrNotFound {
		t.Errorf("unknown item: %v", err)
	}
}

func TestLearningAndUnknownItem(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 6; i++ {
		o, _ := h.e.AddOrder(NewOrder{Table: "2", Items: []OrderLine{{SKU: "rizek", Qty: 1}}})
		id := o.Items[0].ID
		h.do(id, ActClaim)
		h.advance(12 * time.Minute)
		h.do(id, ActReady)
		h.do(id, ActPick)
	}
	p50, _ := h.e.learner.Prep("rizek", "gril")
	if p50 != 12*time.Minute {
		t.Errorf("learned p50 = %v, want 12m", p50)
	}
	o, err := h.e.AddOrder(NewOrder{Table: "3", Items: []OrderLine{{Name: "Speciál šéfa", Qty: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if it := o.Items[0]; it.Station != "gril" || it.Name != "Speciál šéfa" {
		t.Errorf("unknown item = %+v", it)
	}
}

func TestStableETA(t *testing.T) {
	h := newHarness(t)
	o, _ := h.e.AddOrder(NewOrder{Table: "4", Items: []OrderLine{{SKU: "rizek", Qty: 1}}})
	first := h.e.Snapshot().Tables[0].ETAMin
	h.do(o.Items[0].ID, ActClaim)
	h.advance(10 * time.Second)
	if got := h.e.Snapshot().Tables[0].ETAMin; got != first {
		t.Errorf("ETA changed within refresh window: %d → %d", first, got)
	}
	h.advance(5 * time.Minute)
	if got := h.e.Snapshot().Tables[0].ETAMin; got == first {
		t.Errorf("ETA did not refresh after 5 min: still %d", got)
	}
}
