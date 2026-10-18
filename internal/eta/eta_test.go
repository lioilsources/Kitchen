package eta

import (
	"testing"
	"time"

	"github.com/lioilsources/Kitchen/internal/model"
)

func TestQuantile(t *testing.T) {
	ds := []time.Duration{5, 1, 3, 2, 4, 6, 7, 8, 9, 10}
	if q := Quantile(ds, 0.5); q != 5 {
		t.Errorf("p50 = %d", q)
	}
	if q := Quantile(ds, 0.9); q != 9 {
		t.Errorf("p90 = %d", q)
	}
	if Quantile(nil, 0.5) != 0 {
		t.Error("empty")
	}
}

func TestCompute(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	claimed := now.Add(-4 * time.Minute)
	prep := func(sku, _ string) (time.Duration, time.Duration) {
		switch sku {
		case "steak":
			return 14 * time.Minute, 20 * time.Minute
		case "hranolky":
			return 5 * time.Minute, 7 * time.Minute
		}
		return 10 * time.Minute, 15 * time.Minute
	}
	st := map[string]model.Station{"gril": {ID: "gril", Parallelism: 1}}
	items := []*model.Item{
		{ID: "a", OrderID: "o1", SKU: "x", Station: "gril", Course: 1, Status: model.StatusCooking, ClaimedAt: &claimed, Seq: 1},
		{ID: "b", OrderID: "o2", SKU: "steak", Station: "gril", Course: 1, Status: model.StatusNew, Seq: 2},
		{ID: "c", OrderID: "o2", SKU: "hranolky", Station: "gril", Course: 1, Status: model.StatusNew, Seq: 3},
	}
	r := Compute(items, st, prep, now)
	// a: 10 − 4 = 6 min zbývá
	if r["a"].ETA != 6*time.Minute {
		t.Errorf("a eta = %v", r["a"].ETA)
	}
	// b: čeká na a (6) + 14 = 20
	if r["b"].ETA != 20*time.Minute {
		t.Errorf("b eta = %v", r["b"].ETA)
	}
	// c: 6 + 14 + 5 = 25 (paralelismus 1); start_in 0, protože je nejdelší v chodu
	if r["c"].ETA != 25*time.Minute || r["c"].StartIn != 0 {
		t.Errorf("c = %+v", r["c"])
	}
	// s paralelismem 4 se fronta dělí: b = 6/4 + 14, c = (6+14)/4 + 5 = 10 → c může začít o 5,5 min později
	st["gril"] = model.Station{ID: "gril", Parallelism: 4}
	r = Compute(items, st, prep, now)
	if want := 90*time.Second + 14*time.Minute; r["b"].ETA != want {
		t.Errorf("b eta par4 = %v want %v", r["b"].ETA, want)
	}
	if r["c"].StartIn <= 0 {
		t.Errorf("c start_in = %v, want > 0", r["c"].StartIn)
	}
}

func TestMinutes(t *testing.T) {
	for d, want := range map[time.Duration]int{0: 0, time.Second: 1, time.Minute: 1, 61 * time.Second: 2, -time.Minute: 0} {
		if got := Minutes(d); got != want {
			t.Errorf("Minutes(%v) = %d want %d", d, got, want)
		}
	}
}
