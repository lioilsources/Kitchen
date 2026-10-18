// scenarios replays scripted service situations with a simulated clock and prints the resulting
// display snapshots and voice lines as JSON — fixtures for UI work and demos.
//
//	go run ./tools/scenarios -config config.dev.yaml > scenarios.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lioilsources/Kitchen/internal/config"
	"github.com/lioilsources/Kitchen/internal/eta"
	"github.com/lioilsources/Kitchen/internal/queue"
	"github.com/lioilsources/Kitchen/internal/store"
	"github.com/lioilsources/Kitchen/internal/tts"
)

type voice struct{ said []tts.Announcement }

func (v *voice) Say(a tts.Announcement)  { v.said = append(v.said, a) }
func (v *voice) Repeat()                 {}
func (v *voice) Tone(context.Context)    {}
func (v *voice) SetQuiet(bool)           {}
func (v *voice) SetMinGap(time.Duration) {}

type Scenario struct {
	ID       string             `json:"id"`
	Title    string             `json:"title"`
	Snapshot queue.Snapshot     `json:"snapshot"`
	Voice    []tts.Announcement `json:"voice"`
}

// sim drives one engine on a fake clock.
type sim struct {
	e   *queue.Engine
	v   *voice
	now time.Time
}

func newSim(cfg *config.Config, dir, name string, hour, min int) *sim {
	st, err := store.Open(filepath.Join(dir, name+".db"))
	if err != nil {
		log.Fatal(err)
	}
	l, err := eta.NewLearner(st, cfg.MenuBySKU())
	if err != nil {
		log.Fatal(err)
	}
	s := &sim{v: &voice{}, now: time.Date(2026, 10, 17, hour, min, 0, 0, time.Local)}
	s.e, err = queue.NewWithClock(cfg, st, l, s.v, func() time.Time { return s.now })
	if err != nil {
		log.Fatal(err)
	}
	return s
}

// wait advances the clock, ticking the rules every 10 s like the real 1 s loop would.
func (s *sim) wait(d time.Duration) {
	for end := s.now.Add(d); s.now.Before(end); {
		s.now = s.now.Add(10 * time.Second)
		s.e.Tick()
	}
}

func (s *sim) order(table string, lines ...queue.OrderLine) []string {
	o, err := s.e.AddOrder(queue.NewOrder{Table: table, Source: "pos", Items: lines})
	if err != nil {
		log.Fatal(err)
	}
	var ids []string
	for _, it := range o.Items {
		ids = append(ids, it.ID)
	}
	return ids
}

func (s *sim) do(action, by string, ids ...string) {
	for _, id := range ids {
		if _, err := s.e.Transition(id, action, by); err != nil {
			log.Fatalf("%s %s: %v", action, id, err)
		}
	}
}

func l(sku string, qty int, mods ...string) queue.OrderLine {
	return queue.OrderLine{SKU: sku, Qty: qty, Mods: mods}
}

func main() {
	cfgPath := flag.String("config", "config.dev.yaml", "config with menu and stations")
	flag.Parse()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	dir, err := os.MkdirTemp("", "predavka-scenarios")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var out []Scenario
	snap := func(id, title string, s *sim, since int) {
		out = append(out, Scenario{ID: id, Title: title, Snapshot: s.e.Snapshot(), Voice: s.v.said[since:]})
	}

	// 1) Prázdná fronta: odpoledne po obědě, poslední stůl odešel.
	s := newSim(cfg, dir, "empty", 15, 0)
	ids := s.order("5", l("polevka-dne", 2))
	s.do("claim", "teple", ids...)
	s.wait(2 * time.Minute)
	s.do("ready", "", ids...)
	s.wait(40 * time.Second)
	s.do("pick", "expo", ids...)
	s.wait(5 * time.Minute)
	snap("empty", "Prázdná fronta", s, len(s.v.said))

	// 2) Málo objednávek: klidný začátek večera, tři stoly.
	s = newSim(cfg, dir, "few", 17, 45)
	t4 := s.order("4", l("svickova", 2), l("rizek", 1))
	s.wait(30 * time.Second)
	s.do("claim", "teple", t4[0])
	s.do("claim", "gril", t4[1])
	s.wait(3 * time.Minute)
	t2 := s.order("2", l("salat", 1), l("tatarak", 1, "bez cibule"))
	s.wait(20 * time.Second)
	s.do("claim", "studena", t2...)
	s.wait(4 * time.Minute)
	s.do("ready", "", t2...)
	s.wait(50 * time.Second)
	s.order("9", l("burger", 2, "medium"), l("hranolky", 2), l("tiramisu", 2))
	s.wait(40 * time.Second)
	snap("few", "Málo objednávek", s, 0)

	// 3) Nestíháme: sobota 19:30, plno, gril je úzké hrdlo, obsluha nestíhá vynášet.
	s = newSim(cfg, dir, "rush", 19, 10)
	type tbl struct {
		table string
		lines []queue.OrderLine
	}
	wave := []tbl{
		{"1", []queue.OrderLine{l("steak", 2, "medium rare"), l("hranolky", 2)}},
		{"3", []queue.OrderLine{l("burger", 3), l("hranolky", 3)}},
		{"6", []queue.OrderLine{l("rizek", 4), l("svickova", 2)}},
		{"8", []queue.OrderLine{l("steak", 1), l("burger", 2, "bez cibule")}},
		{"11", []queue.OrderLine{l("svickova", 3), l("rizek", 1, "bez omáčky")}},
		{"12", []queue.OrderLine{l("rizek", 2), l("hranolky", 1)}},
		{"14", []queue.OrderLine{l("tatarak", 2), l("salat", 2)}},
		{"15", []queue.OrderLine{l("burger", 4), l("steak", 2, "well done")}},
		{"17", []queue.OrderLine{l("svickova", 2), l("rizek", 2)}},
		{"20", []queue.OrderLine{l("steak", 3)}},
		{"bar 2", []queue.OrderLine{l("burger", 1), l("hranolky", 1)}},
	}
	orders := map[string][]string{}
	for i, w := range wave {
		orders[w.table] = s.order(w.table, w.lines...)
		if i < 7 {
			s.wait(70 * time.Second)
		} else {
			s.wait(25 * time.Second)
		}
	}
	// Teplá a studená stíhají, gril bere jen to nejstarší.
	s.do("claim", "teple", orders["6"][1], orders["11"][0])
	s.do("claim", "gril", orders["11"][1])
	s.do("claim", "gril", orders["1"][0], orders["1"][1], orders["3"][0], orders["3"][1], orders["6"][0])
	s.do("claim", "studena", orders["14"]...)
	s.wait(2 * time.Minute)
	s.do("ready", "", orders["14"]...)
	s.do("ready", "", orders["6"][1])
	s.wait(3 * time.Minute)
	s.do("ready", "", orders["11"][0])
	s.do("ready", "", orders["3"]...)
	s.do("claim", "gril", orders["8"][0])
	s.wait(4 * time.Minute)
	s.wait(90 * time.Second)
	snap("rush", "Nestíháme!", s, 0)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		log.Fatal(err)
	}
}
