// Package tts turns announcements into Czech speech on the pass speaker (Piper → WAV/PCM → player or Snapcast).
package tts

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type Kind string

const (
	KindNewOrder  Kind = "new_order"
	KindComplete  Kind = "complete"
	KindWaitClaim Kind = "wait_claim"
	KindPassAging Kind = "pass_aging"
	KindFire      Kind = "fire"
	KindRepeat    Kind = "repeat"
	KindInfo      Kind = "info"
)

// Priority: urgence > kompletní stůl > nová objednávka.
func (k Kind) priority() int {
	switch k {
	case KindPassAging, KindWaitClaim:
		return 3
	case KindRepeat:
		return 4
	case KindComplete, KindFire:
		return 2
	}
	return 1
}

type Announcement struct {
	Kind  Kind      `json:"kind"`
	Text  string    `json:"text"`
	Table string    `json:"table,omitempty"`
	TS    time.Time `json:"ts"`
	// Key coalesces duplicates waiting in the queue (např. "pass_aging:7").
	Key string `json:"-"`
}

// Speaker synthesizes and plays one sentence; it blocks until playback finished
// and calls started() the moment audio output begins (for latency measurement).
type Speaker interface {
	Speak(ctx context.Context, text string, started func()) error
	Tone(ctx context.Context) error
}

// staleAfter drops new-order lines that waited too long — the display already shows them.
const staleAfter = 2 * time.Minute

type Announcer struct {
	mu       sync.Mutex
	queue    []Announcement
	wake     chan struct{}
	last     *Announcement
	quiet    bool
	minGap   time.Duration
	speaker  Speaker
	onSpeak  func(Announcement) // pro displeje (titulek) a notifikace
	latency  time.Duration      // poslední: zařazení → konec syntézy/začátek přehrávání
	lastSaid time.Time
}

func NewAnnouncer(sp Speaker, minGap time.Duration, onSpeak func(Announcement)) *Announcer {
	return &Announcer{speaker: sp, minGap: minGap, onSpeak: onSpeak, wake: make(chan struct{}, 1)}
}

func (a *Announcer) SetQuiet(on bool) {
	a.mu.Lock()
	a.quiet = on
	if on {
		a.queue = nil
	}
	a.mu.Unlock()
}

func (a *Announcer) SetMinGap(d time.Duration) {
	a.mu.Lock()
	a.minGap = d
	a.mu.Unlock()
}

// Say enqueues an announcement. In quiet mode it is still passed to onSpeak (displeje, hodinky), just not voiced.
func (a *Announcer) Say(an Announcement) {
	if an.TS.IsZero() {
		an.TS = time.Now()
	}
	a.mu.Lock()
	quiet := a.quiet
	if an.Kind != KindRepeat {
		c := an
		a.last = &c
	}
	if !quiet {
		replaced := false
		if an.Key != "" {
			for i := range a.queue {
				if a.queue[i].Key == an.Key {
					a.queue[i] = an
					replaced = true
				}
			}
		}
		if !replaced {
			a.queue = append(a.queue, an)
		}
		sort.SliceStable(a.queue, func(i, j int) bool { return a.queue[i].Kind.priority() > a.queue[j].Kind.priority() })
	}
	a.mu.Unlock()
	if quiet && a.onSpeak != nil {
		a.onSpeak(an)
	}
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// Repeat re-speaks the last announcement ("ještě jednou").
func (a *Announcer) Repeat() {
	a.mu.Lock()
	last := a.last
	a.mu.Unlock()
	if last == nil {
		return
	}
	r := *last
	r.Kind, r.Key, r.TS = KindRepeat, "repeat", time.Now()
	a.Say(r)
}

// Tone plays the short claim confirmation (no speech), unless quiet.
func (a *Announcer) Tone(ctx context.Context) {
	a.mu.Lock()
	quiet := a.quiet
	a.mu.Unlock()
	if !quiet {
		go func() {
			if err := a.speaker.Tone(ctx); err != nil {
				slog.Warn("tone", "err", err)
			}
		}()
	}
}

func (a *Announcer) LastLatency() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.latency
}

// Run speaks queued announcements one by one, keeping minGap between them.
func (a *Announcer) Run(ctx context.Context) {
	for {
		an, wait, ok := a.next()
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-a.wake:
				continue
			}
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			continue
		}
		if an.Kind == KindNewOrder && time.Since(an.TS) > staleAfter {
			slog.Info("announce dropped (stale)", "text", an.Text)
			continue
		}
		if a.onSpeak != nil {
			a.onSpeak(an)
		}
		slog.Info("announce", "kind", an.Kind, "text", an.Text, "queued", time.Since(an.TS).Round(time.Millisecond))
		ts := an.TS
		err := a.speaker.Speak(ctx, an.Text, func() {
			a.mu.Lock()
			a.latency = time.Since(ts)
			a.mu.Unlock()
		})
		a.mu.Lock()
		a.lastSaid = time.Now()
		a.mu.Unlock()
		if err != nil {
			slog.Warn("speak", "err", err, "text", an.Text)
			continue
		}
	}
}

// next pops the highest-priority announcement, or reports how long to wait for the gap.
func (a *Announcer) next() (Announcement, time.Duration, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.queue) == 0 {
		return Announcement{}, 0, false
	}
	gap := a.minGap
	if a.queue[0].Kind == KindRepeat {
		gap = 0 // na vyžádání hned
	}
	if w := time.Until(a.lastSaid.Add(gap)); w > 0 {
		return Announcement{}, w, true
	}
	an := a.queue[0]
	a.queue = a.queue[1:]
	return an, 0, true
}
