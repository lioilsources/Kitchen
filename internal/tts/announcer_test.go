package tts

import (
	"context"
	"encoding/binary"
	"sync"
	"testing"
	"time"
)

type recSpeaker struct {
	mu   sync.Mutex
	said []string
}

func (r *recSpeaker) Speak(_ context.Context, text string, started func()) error {
	started()
	r.mu.Lock()
	r.said = append(r.said, text)
	r.mu.Unlock()
	return nil
}
func (r *recSpeaker) Tone(context.Context) error { return nil }

func (r *recSpeaker) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.said...)
}

func TestAnnouncerPriorityAndCoalescing(t *testing.T) {
	sp := &recSpeaker{}
	a := NewAnnouncer(sp, 50*time.Millisecond, nil)
	a.Say(Announcement{Kind: KindNewOrder, Text: "nová 1"})
	a.Say(Announcement{Kind: KindNewOrder, Text: "nová 2"})
	a.Say(Announcement{Kind: KindPassAging, Text: "stárne A", Key: "aging:1"})
	a.Say(Announcement{Kind: KindPassAging, Text: "stárne B", Key: "aging:1"}) // nahradí A
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	deadline := time.Now().Add(2 * time.Second)
	for len(sp.get()) < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := sp.get()
	want := []string{"stárne B", "nová 1", "nová 2"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("said %q, want %q", got, want)
	}
}

func TestQuietSkipsVoiceButNotifies(t *testing.T) {
	sp := &recSpeaker{}
	var notified []string
	a := NewAnnouncer(sp, 0, func(an Announcement) { notified = append(notified, an.Text) })
	a.SetQuiet(true)
	a.Say(Announcement{Kind: KindComplete, Text: "Stůl sedm, kompletní."})
	if len(notified) != 1 {
		t.Errorf("quiet mode must still notify displays, got %v", notified)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	a.Run(ctx)
	if len(sp.get()) != 0 {
		t.Errorf("quiet mode spoke: %v", sp.get())
	}
}

func TestPCM(t *testing.T) {
	data := []byte{1, 0, 2, 0, 3, 0}
	wav := []byte("RIFF\x00\x00\x00\x00WAVE")
	fmtChunk := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtChunk[0:], 1)
	binary.LittleEndian.PutUint16(fmtChunk[2:], 1)
	binary.LittleEndian.PutUint32(fmtChunk[4:], 22050)
	binary.LittleEndian.PutUint16(fmtChunk[14:], 16)
	wav = append(wav, []byte("fmt \x10\x00\x00\x00")...)
	wav = append(wav, fmtChunk...)
	wav = append(wav, []byte("LIST\x03\x00\x00\x00abc\x00")...) // lichá délka + padding
	wav = append(wav, []byte("data\x06\x00\x00\x00")...)
	wav = append(wav, data...)
	pcm, rate, ch, err := PCM(wav)
	if err != nil || rate != 22050 || ch != 1 || string(pcm) != string(data) {
		t.Fatalf("PCM = %v %d %d %v", pcm, rate, ch, err)
	}
}
