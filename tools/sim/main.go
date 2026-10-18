// sim generates realistic restaurant traffic against a running predavka-server (no restaurant needed).
//
//	go run ./tools/sim -url http://localhost:8080 -rate 40 -cooks
//
// -rate   objednávek za hodinu (pilot: 20–60 míst ≈ 15–50/h ve špičce)
// -cooks  simulovaní kuchaři: berou z fronty, vaří (p50 × šum), hlásí hotovo; expo vyzvedává
// -speed  zrychlení vaření pro demo (2 = vaří 2× rychleji; zkresluje učení prep-time)
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type menuItem struct {
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Station string `json:"station"`
	Prep    string `json:"prep"`
	Course  int    `json:"course"`
}

type item struct {
	ID        string     `json:"id"`
	Table     string     `json:"table"`
	SKU       string     `json:"sku"`
	Station   string     `json:"station"`
	Status    string     `json:"status"`
	ClaimedAt *time.Time `json:"claimed_at"`
	WaitS     int        `json:"wait_s"`
}

type snapshot struct {
	Items  []item `json:"items"`
	Tables []struct {
		Table      string `json:"table"`
		Status     string `json:"status"`
		ReadyWaitS int    `json:"ready_wait_s"`
	} `json:"tables"`
}

var (
	base   = flag.String("url", "http://localhost:8080", "server URL")
	rate   = flag.Float64("rate", 30, "orders per hour")
	cooks  = flag.Bool("cooks", false, "simulate cooks and expo")
	speed  = flag.Float64("speed", 1, "cooking speed-up for demos")
	tables = flag.Int("tables", 15, "number of tables")
	count  = flag.Int("n", 0, "stop after n orders (0 = forever)")
)

func main() {
	flag.Parse()
	var menu []menuItem
	if err := get("/api/menu", &menu); err != nil {
		log.Fatalf("menu: %v (běží server?)", err)
	}
	log.Printf("menu: %d položek, %.0f objednávek/h, kuchaři=%v", len(menu), *rate, *cooks)
	if *cooks {
		go cookLoop(menu)
	}
	for n := 0; *count == 0 || n < *count; n++ {
		order(menu)
		// Poissonův proces: exponenciální mezery mezi objednávkami.
		gap := time.Duration(rand.ExpFloat64() / *rate * float64(time.Hour))
		time.Sleep(min(gap, 15*time.Minute))
	}
	if *cooks {
		select {}
	}
}

func order(menu []menuItem) {
	byCourse := map[int][]menuItem{}
	for _, m := range menu {
		byCourse[m.Course] = append(byCourse[m.Course], m)
	}
	guests := 1 + rand.IntN(4)
	var lines []map[string]any
	add := func(c int, p float64) {
		opts := byCourse[c]
		if len(opts) == 0 {
			return
		}
		qty := map[string]int{}
		for g := 0; g < guests; g++ {
			if rand.Float64() < p {
				qty[opts[rand.IntN(len(opts))].SKU]++
			}
		}
		for sku, q := range qty {
			l := map[string]any{"sku": sku, "qty": q}
			if rand.Float64() < 0.15 {
				l["mods"] = []string{[]string{"bez cibule", "extra omáčka", "bezlepkové", "medium"}[rand.IntN(4)]}
			}
			lines = append(lines, l)
		}
	}
	add(1, 0.5)
	add(2, 0.95)
	add(3, 0.3)
	if len(lines) == 0 {
		return
	}
	table := strconv.Itoa(1 + rand.IntN(*tables))
	body := map[string]any{"table": table, "source": "sim", "guests": guests, "items": lines}
	if err := post("/api/orders", body); err != nil {
		log.Printf("order: %v", err)
		return
	}
	log.Printf("stůl %s: %d hostů, %d řádků", table, guests, len(lines))
}

// cookLoop: each station has cooks that claim after a short delay, cook ~p50 with noise, bump ready;
// expo picks up ready tables after 0.5–4 min.
func cookLoop(menu []menuItem) {
	prep := map[string]time.Duration{}
	for _, m := range menu {
		d, _ := time.ParseDuration(m.Prep)
		prep[m.SKU] = d
	}
	var mu sync.Mutex
	busy := map[string]bool{}
	pickAt := map[string]time.Time{}
	for range time.Tick(2 * time.Second) {
		var s snapshot
		if err := get("/api/state", &s); err != nil {
			log.Printf("state: %v", err)
			continue
		}
		for _, it := range s.Items {
			mu.Lock()
			b := busy[it.ID]
			mu.Unlock()
			if b {
				continue
			}
			switch it.Status {
			case "NEW":
				// Kuchař si všimne za 10–150 s (občas přetáhne práh 2 min).
				if it.WaitS > 10+rand.IntN(140) {
					it := it
					mu.Lock()
					busy[it.ID] = true
					mu.Unlock()
					go func() {
						defer func() { mu.Lock(); delete(busy, it.ID); mu.Unlock() }()
						if err := post("/api/items/"+it.ID+"/claim", map[string]string{"by": it.Station}); err != nil {
							return
						}
						noise := 0.75 + rand.Float64()*0.7
						time.Sleep(time.Duration(float64(prep[it.SKU]) * noise / *speed))
						post("/api/items/"+it.ID+"/ready", nil)
					}()
				}
			}
		}
		for _, t := range s.Tables {
			if t.Status != "ready" {
				continue
			}
			at, ok := pickAt[t.Table]
			if !ok {
				at = time.Now().Add(time.Duration(30+rand.IntN(210)) * time.Second)
				pickAt[t.Table] = at
			}
			if time.Now().After(at) {
				delete(pickAt, t.Table)
				if err := post("/api/tables/"+t.Table+"/pick", nil); err == nil {
					log.Printf("expo: stůl %s vyzvednut (na předávce %ds)", t.Table, t.ReadyWaitS)
				}
			}
		}
	}
}

func get(path string, v any) error {
	r, err := http.Get(*base + path)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("%s: %s", path, r.Status)
	}
	return json.NewDecoder(r.Body).Decode(v)
}

func post(path string, body any) error {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r, err := http.Post(*base+path, "application/json", &buf)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		var e struct{ Error string }
		json.NewDecoder(r.Body).Decode(&e)
		return fmt.Errorf("%s: %s %s", path, r.Status, e.Error)
	}
	return nil
}
