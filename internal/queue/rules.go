package queue

import (
	"context"
	"strconv"
	"time"

	"github.com/lioilsources/Kitchen/internal/model"
	"github.com/lioilsources/Kitchen/internal/tts"
)

// Run evaluates timers once per second until ctx is done.
func (e *Engine) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if e.Tick() {
				e.changed()
			}
		}
	}
}

// Tick applies the escalation rules (docs/PLAN.md §3 "Timery a eskalace") and reports whether state changed.
//
//	wait-to-claim  NEW déle než práh       → hlas 1× "Stůl 7, gril, čeká na kuchaře"
//	cook overrun   vaření > p90 × faktor   → jen displej (červená), bez hlasu
//	pass aging     READY déle než práh     → hlas "Stůl 7 hotovo, na předávce 3 minuty" (+ opakování)
//	course gap     další chod              → auto fire: hotový dřív o p50 než uplyne mezera
func (e *Engine) Tick() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	s := e.settings
	items := e.sortedItems()

	for _, it := range items {
		o := e.orders[it.OrderID]
		if it.Status == model.StatusNew && now.Sub(it.ChangedAt) >= s.WaitToClaim.D() {
			key := "claim:" + o.ID + ":" + it.Station
			if _, done := e.alerted[key]; !done {
				e.alerted[key] = now
				e.voice.Say(tts.Announcement{Kind: tts.KindWaitClaim, Table: o.Table, Key: key,
					Text: tts.WaitClaim(o.Table, e.stations[it.Station].Name)})
			}
		}
	}

	// Pass aging per order+course: nejstarší READY položka chodu.
	type oc struct {
		order  string
		course int
	}
	oldest := map[oc]time.Time{}
	for _, it := range items {
		if it.Status != model.StatusReady {
			continue
		}
		k := oc{it.OrderID, it.Course}
		if t, ok := oldest[k]; !ok || it.ChangedAt.Before(t) {
			oldest[k] = it.ChangedAt
		}
	}
	for k, since := range oldest {
		wait := now.Sub(since)
		if wait < s.PassAging.D() {
			continue
		}
		o := e.orders[k.order]
		key := "aging:" + k.order + ":" + strconv.Itoa(k.course)
		last, done := e.alerted[key]
		if done && (s.ReannounceAging.D() <= 0 || now.Sub(last) < s.ReannounceAging.D()) {
			continue
		}
		e.alerted[key] = now
		e.voice.Say(tts.Announcement{Kind: tts.KindPassAging, Table: o.Table, Key: key,
			Text: tts.PassAging(o.Table, int(wait/time.Minute))})
	}

	changed := false
	if s.AutoFire {
		for _, o := range e.orders {
			if e.shouldFire(o, now) {
				if n, err := e.fire(o, "auto"); err == nil && n > 0 {
					changed = true
				}
			}
		}
	}

	// Úklid klíčů eskalací pro neaktivní objednávky.
	for key, at := range e.alerted {
		if now.Sub(at) > 6*time.Hour {
			delete(e.alerted, key)
		}
	}
	return changed || len(items) > 0 // aktivní položky → čekací časy na displejích se mění
}

// shouldFire: previous courses are all off the pass and the next course, cooked now, would land
// roughly course_gap after the previous one was picked up.
func (e *Engine) shouldFire(o *model.Order, now time.Time) bool {
	held := 0
	for _, it := range o.Items {
		if it.Status == model.StatusHold && (held == 0 || it.Course < held) {
			held = it.Course
		}
	}
	if held == 0 {
		return false
	}
	var lastPicked time.Time
	var longest time.Duration
	for _, it := range o.Items {
		switch {
		case it.Course < held:
			if it.Status.Active() {
				return false // předchozí chod ještě není venku
			}
			if it.PickedAt != nil && it.PickedAt.After(lastPicked) {
				lastPicked = *it.PickedAt
			}
		case it.Course == held:
			if p50, _ := e.learner.Prep(it.SKU, it.Station); p50 > longest {
				longest = p50
			}
		}
	}
	if lastPicked.IsZero() {
		return true // předchozí chod zrušen / servírován bez pick → uvolnit hned
	}
	return now.After(lastPicked.Add(e.settings.CourseGap.D() - longest))
}
