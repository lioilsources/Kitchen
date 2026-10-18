// Package queue is the core: order/item state (rebuilt from the event log), claims, timers and escalations.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lioilsources/Kitchen/internal/config"
	"github.com/lioilsources/Kitchen/internal/eta"
	"github.com/lioilsources/Kitchen/internal/model"
	"github.com/lioilsources/Kitchen/internal/tts"
)

// EventStore is the persistence the engine needs (implemented by store.Store).
type EventStore interface {
	Append(*model.Event) error
	Events(since time.Time, fn func(*model.Event) error) error
	GetJSON(k string, v any) (bool, error)
	PutJSON(k string, v any) error
}

// Voice is what the engine says out loud (implemented by tts.Announcer).
type Voice interface {
	Say(tts.Announcement)
	Repeat()
	Tone(context.Context)
	SetQuiet(bool)
	SetMinGap(time.Duration)
}

var (
	ErrNotFound   = errors.New("not found")
	ErrTransition = errors.New("invalid transition")
)

// replayWindow: on startup only recent events matter (one service day plus margin).
const replayWindow = 36 * time.Hour

type Engine struct {
	mu       sync.Mutex
	cfg      *config.Config
	menu     map[string]*config.MenuItem
	stations map[string]model.Station
	store    EventStore
	learner  *eta.Learner
	voice    Voice
	settings config.Settings
	quiet    bool
	now      func() time.Time

	orders map[string]*model.Order
	items  map[string]*model.Item
	seq    int64

	alerted  map[string]time.Time // klíč eskalace → kdy naposledy hlášeno
	etaShown map[string]shownETA  // stůl → co ukazuje displej na place (mění se max 1× za eta_refresh)
	onChange []func()
}

type shownETA struct {
	min int
	at  time.Time
}

func New(cfg *config.Config, st EventStore, learner *eta.Learner, voice Voice) (*Engine, error) {
	return NewWithClock(cfg, st, learner, voice, time.Now)
}

// NewWithClock is New with an injectable clock (tests, scenario replays in tools/scenarios).
func NewWithClock(cfg *config.Config, st EventStore, learner *eta.Learner, voice Voice, now func() time.Time) (*Engine, error) {
	e := &Engine{
		cfg: cfg, menu: cfg.MenuBySKU(), stations: map[string]model.Station{},
		store: st, learner: learner, voice: voice, settings: cfg.Settings, now: now,
		orders: map[string]*model.Order{}, items: map[string]*model.Item{},
		alerted: map[string]time.Time{}, etaShown: map[string]shownETA{},
	}
	for _, s := range cfg.Stations {
		e.stations[s.ID] = s
	}
	var saved config.Settings
	if ok, err := st.GetJSON("settings", &saved); err != nil {
		return nil, err
	} else if ok {
		e.settings = saved
	}
	e.quiet = e.settings.Quiet
	if err := st.Events(e.now().Add(-replayWindow), func(ev *model.Event) error {
		if err := e.apply(ev); err != nil {
			slog.Warn("replay: skipping event", "seq", ev.Seq, "err", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	voice.SetQuiet(e.quiet)
	voice.SetMinGap(e.settings.AnnounceMinGap.D())
	slog.Info("engine ready", "orders", len(e.orders), "active_items", e.activeCount())
	return e, nil
}

// OnChange registers a callback fired (outside the lock) after every state change.
func (e *Engine) OnChange(fn func()) { e.onChange = append(e.onChange, fn) }

func (e *Engine) changed() {
	for _, fn := range e.onChange {
		fn()
	}
}

func (e *Engine) activeCount() int {
	n := 0
	for _, it := range e.items {
		if it.Status.Active() {
			n++
		}
	}
	return n
}

// --- event sourcing ---

func (e *Engine) emit(ev *model.Event) error {
	ev.TS = e.now()
	if err := e.store.Append(ev); err != nil {
		return err
	}
	return e.apply(ev)
}

func (e *Engine) apply(ev *model.Event) error {
	switch ev.Type {
	case model.EvOrderCreated:
		raw, err := json.Marshal(ev.Payload["order"])
		if err != nil {
			return err
		}
		var o model.Order
		if err := json.Unmarshal(raw, &o); err != nil {
			return err
		}
		e.orders[o.ID] = &o
		for _, it := range o.Items {
			e.seq++
			it.Seq = e.seq
			e.items[it.ID] = it
		}
	case model.EvItemStatus:
		it := e.items[ev.ItemID]
		if it == nil {
			return fmt.Errorf("item %s: %w", ev.ItemID, ErrNotFound)
		}
		to := model.Status(fmt.Sprint(ev.Payload["status"]))
		t := ev.TS
		it.Status, it.ChangedAt = to, t
		switch to {
		case model.StatusClaimed:
			it.ClaimedAt, it.ClaimedBy = &t, ev.Actor
		case model.StatusCooking:
			if it.ClaimedAt == nil {
				it.ClaimedAt = &t
			}
		case model.StatusReady:
			it.ReadyAt = &t
		case model.StatusPicked:
			it.PickedAt = &t
		case model.StatusNew:
			it.CreatedAt = t // po fire se čekání měří od uvolnění chodu
		}
	case model.EvQuiet:
		e.quiet, _ = ev.Payload["on"].(bool)
	default:
		return fmt.Errorf("unknown event type %q", ev.Type)
	}
	return nil
}

// --- commands ---

// OrderLine is an incoming order line; SKU or Name (resolved against the menu) is required.
type OrderLine struct {
	SKU    string   `json:"sku"`
	Name   string   `json:"name"`
	Qty    int      `json:"qty"`
	Mods   []string `json:"mods"`
	Course int      `json:"course"`
}

type NewOrder struct {
	Table  string      `json:"table"`
	Source string      `json:"source"`
	Guests int         `json:"guests"`
	Note   string      `json:"note"`
	Items  []OrderLine `json:"items"`
}

func (e *Engine) AddOrder(in NewOrder) (*model.Order, error) {
	in.Table = strings.TrimSpace(in.Table)
	if in.Table == "" {
		return nil, errors.New("table is required")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("order has no items")
	}
	e.mu.Lock()
	now := e.now()
	o := &model.Order{ID: model.NewID("or"), Source: in.Source, Table: in.Table, Guests: in.Guests, Note: in.Note, CreatedAt: now}
	minCourse := 0
	for _, l := range in.Items {
		m := e.resolve(l.SKU, l.Name)
		it := &model.Item{ID: model.NewID("it"), OrderID: o.ID, Qty: max(l.Qty, 1), Mods: l.Mods, Course: l.Course, CreatedAt: now, ChangedAt: now}
		if m != nil {
			it.SKU, it.Name, it.Station = m.SKU, m.Name, m.Station
			if it.Course == 0 {
				it.Course = m.Course
			}
		} else {
			// Neznámá položka (např. z bonu): jde na první stanici se statickým odhadem.
			it.SKU, it.Name, it.Station = "", strings.TrimSpace(l.Name), e.cfg.Stations[0].ID
			if it.Name == "" {
				it.Name = l.SKU
			}
		}
		if it.Course == 0 {
			it.Course = 1
		}
		if minCourse == 0 || it.Course < minCourse {
			minCourse = it.Course
		}
		o.Items = append(o.Items, it)
	}
	for _, it := range o.Items {
		it.Status = model.StatusNew
		if it.Course > minCourse {
			it.Status = model.StatusHold
		}
	}
	err := e.emit(&model.Event{Type: model.EvOrderCreated, OrderID: o.ID, Actor: in.Source, Payload: map[string]any{"order": o}})
	if err != nil {
		e.mu.Unlock()
		return nil, err
	}
	e.announceItems(o.Table, o.Items, model.StatusNew, tts.KindNewOrder)
	e.mu.Unlock()
	e.changed()
	return o, nil
}

func (e *Engine) resolve(sku, name string) *config.MenuItem {
	if m := e.menu[sku]; m != nil {
		return m
	}
	n := fold(name)
	if n == "" {
		n = fold(sku)
	}
	for _, m := range e.menu {
		if fold(m.Name) == n || fold(m.SKU) == n {
			return m
		}
		for _, a := range m.Aliases {
			if fold(a) == n {
				return m
			}
		}
	}
	return nil
}

// announceItems speaks the items in the given status: "Stůl sedm: dvě svíčkové, jedna polévka."
func (e *Engine) announceItems(table string, items []*model.Item, status model.Status, kind tts.Kind) {
	var lines []tts.LineItem
	mods := false
	for _, it := range items {
		if it.Status != status {
			continue
		}
		say := config.Say{One: strings.ToLower(it.Name), Few: strings.ToLower(it.Name), Many: strings.ToLower(it.Name), Gender: "f"}
		if m := e.menu[it.SKU]; m != nil {
			say = m.Say
		}
		lines = append(lines, tts.LineItem{Qty: it.Qty, Say: say})
		mods = mods || len(it.Mods) > 0
	}
	if len(lines) > 0 {
		e.say(tts.Announcement{Kind: kind, Table: table, Text: tts.NewOrder(table, lines, mods)})
	}
}

// Action names accepted by Transition.
const (
	ActBump  = "bump"
	ActClaim = "claim"
	ActStart = "start"
	ActReady = "ready"
	ActPick  = "pick"
	ActServe = "serve"
	ActVoid  = "void"
	ActHold  = "hold"
	ActFire  = "fire"
)

func next(action string, from model.Status) (model.Status, bool) {
	switch action {
	case ActBump:
		switch from {
		case model.StatusNew:
			return model.StatusClaimed, true
		case model.StatusClaimed, model.StatusCooking:
			return model.StatusReady, true
		case model.StatusReady:
			return model.StatusPicked, true
		}
	case ActClaim:
		return model.StatusClaimed, from == model.StatusNew
	case ActStart:
		return model.StatusCooking, from == model.StatusClaimed
	case ActReady:
		return model.StatusReady, from == model.StatusNew || from.InProgress()
	case ActPick:
		return model.StatusPicked, from == model.StatusReady
	case ActServe:
		return model.StatusServed, from == model.StatusReady || from == model.StatusPicked
	case ActVoid:
		return model.StatusVoid, from.Active()
	case ActHold:
		return model.StatusHold, from == model.StatusNew
	case ActFire:
		return model.StatusNew, from == model.StatusHold
	}
	return "", false
}

// Transition applies an action to one item.
func (e *Engine) Transition(itemID, action, by string) (*model.Item, error) {
	e.mu.Lock()
	it, err := e.transition(itemID, action, by)
	var cp model.Item
	if it != nil {
		cp = *it
	}
	e.mu.Unlock()
	if err != nil {
		return nil, err
	}
	e.changed()
	return &cp, nil
}

func (e *Engine) transition(itemID, action, by string) (*model.Item, error) {
	it := e.items[itemID]
	if it == nil {
		return nil, ErrNotFound
	}
	to, ok := next(action, it.Status)
	if !ok {
		return nil, fmt.Errorf("%w: %s from %s", ErrTransition, action, it.Status)
	}
	if err := e.emit(&model.Event{Type: model.EvItemStatus, OrderID: it.OrderID, ItemID: it.ID, Actor: by, Payload: map[string]any{"status": string(to)}}); err != nil {
		return nil, err
	}
	e.sideEffects(it, to)
	return it, nil
}

// sideEffects runs live-only consequences of a transition (never during replay).
func (e *Engine) sideEffects(it *model.Item, to model.Status) {
	o := e.orders[it.OrderID]
	switch to {
	case model.StatusClaimed:
		e.voice.Tone(context.Background())
	case model.StatusReady:
		if it.ClaimedAt != nil && it.SKU != "" {
			if err := e.learner.Observe(it.SKU, it.Station, *it.ReadyAt, it.ReadyAt.Sub(*it.ClaimedAt)); err != nil {
				slog.Warn("learner", "err", err)
			}
		}
		if e.courseComplete(o, it.Course) {
			e.say(tts.Announcement{Kind: tts.KindComplete, Table: o.Table, Text: tts.Complete(o.Table), Key: "complete:" + o.ID})
		}
	}
}

// courseComplete: all items of the course are READY (or beyond) and at least one is still waiting on the pass.
func (e *Engine) courseComplete(o *model.Order, course int) bool {
	waiting := false
	for _, it := range o.Items {
		if it.Course != course || it.Status == model.StatusVoid {
			continue
		}
		switch it.Status {
		case model.StatusReady:
			waiting = true
		case model.StatusPicked, model.StatusServed:
		default:
			return false
		}
	}
	return waiting
}

// BumpStation bumps the oldest actionable item of a station (bump bar without selection).
func (e *Engine) BumpStation(station, by string) (*model.Item, error) {
	e.mu.Lock()
	var pick *model.Item
	for _, it := range e.sortedItems() {
		if it.Station != station {
			continue
		}
		// Nejdřív dokončit rozvařené, pak brát nové.
		if it.Status.InProgress() {
			pick = it
			break
		}
		if it.Status == model.StatusNew && pick == nil {
			pick = it
		}
	}
	if pick == nil {
		e.mu.Unlock()
		return nil, ErrNotFound
	}
	id := pick.ID
	e.mu.Unlock()
	return e.Transition(id, ActBump, by)
}

// TableAction applies pick (READY→PICKED) or fire (next HOLD course → NEW) to a whole table.
func (e *Engine) TableAction(table, action, by string) (int, error) {
	e.mu.Lock()
	n, err := e.tableAction(table, action, by)
	e.mu.Unlock()
	if n > 0 {
		e.changed()
	}
	return n, err
}

func (e *Engine) tableAction(table, action, by string) (int, error) {
	n := 0
	switch action {
	case ActPick:
		for _, it := range e.sortedItems() {
			if e.orders[it.OrderID].Table == table && it.Status == model.StatusReady {
				if _, err := e.transition(it.ID, ActPick, by); err != nil {
					return n, err
				}
				n++
			}
		}
	case ActFire:
		for _, o := range e.ordersOf(table) {
			k, err := e.fire(o, by)
			if err != nil {
				return n, err
			}
			n += k
		}
	default:
		return 0, fmt.Errorf("%w: %s", ErrTransition, action)
	}
	if n == 0 {
		return 0, ErrNotFound
	}
	return n, nil
}

// fire releases the next held course of an order and announces it.
func (e *Engine) fire(o *model.Order, by string) (int, error) {
	c := 0
	for _, it := range o.Items {
		if it.Status == model.StatusHold && (c == 0 || it.Course < c) {
			c = it.Course
		}
	}
	if c == 0 {
		return 0, nil
	}
	var fired []*model.Item
	for _, it := range o.Items {
		if it.Status == model.StatusHold && it.Course == c {
			if _, err := e.transition(it.ID, ActFire, by); err != nil {
				return len(fired), err
			}
			fired = append(fired, it)
		}
	}
	e.say(tts.Announcement{Kind: tts.KindFire, Table: o.Table, Text: tts.Fire(o.Table, e.cfg.CourseNames[c]), Key: "fire:" + o.ID})
	return len(fired), nil
}

func (e *Engine) ordersOf(table string) []*model.Order {
	var out []*model.Order
	for _, o := range e.orders {
		if o.Table == table {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (e *Engine) SetQuiet(on bool) error {
	e.mu.Lock()
	err := e.emit(&model.Event{Type: model.EvQuiet, Payload: map[string]any{"on": on}})
	if err == nil {
		e.settings.Quiet = on
		err = e.store.PutJSON("settings", e.settings)
	}
	e.mu.Unlock()
	if err != nil {
		return err
	}
	e.voice.SetQuiet(on)
	e.changed()
	return nil
}

func (e *Engine) Repeat() { e.voice.Repeat() }

func (e *Engine) Settings() config.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

func (e *Engine) SetSettings(s config.Settings) error {
	if s.CookOverrunFactor < 1 {
		return errors.New("cook_overrun_factor must be >= 1")
	}
	e.mu.Lock()
	quietChanged := s.Quiet != e.quiet
	e.settings = s
	err := e.store.PutJSON("settings", s)
	e.mu.Unlock()
	if err != nil {
		return err
	}
	e.voice.SetMinGap(s.AnnounceMinGap.D())
	if quietChanged {
		return e.SetQuiet(s.Quiet)
	}
	e.changed()
	return nil
}

func (e *Engine) Menu() []config.MenuItem { return e.cfg.Menu }

func (e *Engine) PrepStats() []eta.Stat { return e.learner.Stats() }

// sortedItems returns active items in queue order. Caller holds the lock.
func (e *Engine) sortedItems() []*model.Item {
	out := make([]*model.Item, 0, len(e.items))
	for _, it := range e.items {
		if it.Status.Active() {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// fold normalizes a dish name for matching: lower case, no diacritics, single spaces.
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if f, ok := diacritics[r]; ok {
			r = f
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

var diacritics = map[rune]rune{
	'á': 'a', 'č': 'c', 'ď': 'd', 'é': 'e', 'ě': 'e', 'í': 'i', 'ň': 'n', 'ó': 'o',
	'ř': 'r', 'š': 's', 'ť': 't', 'ú': 'u', 'ů': 'u', 'ý': 'y', 'ž': 'z', 'ä': 'a', 'ö': 'o', 'ü': 'u',
}

// say stamps an announcement with the engine clock before queueing it.
func (e *Engine) say(a tts.Announcement) {
	a.TS = e.now()
	e.voice.Say(a)
}
