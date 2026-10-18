package tts

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lioilsources/Kitchen/internal/config"
)

// New builds a Speaker from config: engine (piper|say|none) × output (play|snapfifo|none).
func New(c config.TTS) (Speaker, error) {
	var syn synth
	switch c.Engine {
	case "piper":
		if c.PiperModel == "" {
			return nil, errors.New("tts: piper_model is required")
		}
		bin := c.PiperBin
		if bin == "" {
			bin = "piper"
		}
		syn = piper{bin: bin, model: c.PiperModel}
	case "piperd":
		if c.PiperModel == "" {
			return nil, errors.New("tts: piper_model is required")
		}
		py, script := c.PiperPython, c.PiperdScript
		if py == "" {
			py = "python3"
		}
		if script == "" {
			script = "deploy/piperd.py"
		}
		syn = &piperd{cmd: []string{py, "-I", script, c.PiperModel}}
	case "say":
		v := c.SayVoice
		if v == "" {
			v = "Zuzana"
		}
		syn = say{voice: v}
	case "", "none":
		return logSpeaker{}, nil
	default:
		return nil, fmt.Errorf("tts: unknown engine %q", c.Engine)
	}
	var out output
	switch c.Output {
	case "play":
		cmd := c.PlayCmd
		if len(cmd) == 0 {
			cmd = []string{"aplay", "-q"}
		}
		out = player{cmd: cmd}
	case "snapfifo":
		if c.SnapFIFO == "" {
			return nil, errors.New("tts: snap_fifo is required for output snapfifo")
		}
		out = &snapFIFO{path: c.SnapFIFO}
	case "", "none":
		out = discard{}
	default:
		return nil, fmt.Errorf("tts: unknown output %q", c.Output)
	}
	var tone []byte
	if c.ToneFile != "" {
		b, err := os.ReadFile(c.ToneFile)
		if err != nil {
			return nil, fmt.Errorf("tts: tone_file: %w", err)
		}
		tone = b
	}
	if w, ok := syn.(interface{ Warm() error }); ok {
		go func() {
			if err := w.Warm(); err != nil {
				slog.Error("tts warm-up", "err", err)
			}
		}()
	}
	return &pipeline{syn: syn, out: out, tone: tone}, nil
}

// synth renders text to a WAV (16-bit PCM) in memory.
type synth interface {
	WAV(ctx context.Context, text string) ([]byte, error)
}

// output plays a WAV and returns when playback is done.
type output interface {
	Play(ctx context.Context, wav []byte, started func()) error
}

type pipeline struct {
	syn  synth
	out  output
	tone []byte
}

func (p *pipeline) Speak(ctx context.Context, text string, started func()) error {
	t0 := time.Now()
	wav, err := p.syn.WAV(ctx, text)
	if err != nil {
		return err
	}
	slog.Debug("tts synth", "ms", time.Since(t0).Milliseconds(), "bytes", len(wav))
	return p.out.Play(ctx, wav, started)
}

func (p *pipeline) Tone(ctx context.Context) error {
	if p.tone == nil {
		return nil
	}
	return p.out.Play(ctx, p.tone, func() {})
}

// --- engines ---

type piper struct{ bin, model string }

func (p piper) WAV(ctx context.Context, text string) ([]byte, error) {
	f, err := os.CreateTemp("", "predavka-*.wav")
	if err != nil {
		return nil, err
	}
	f.Close()
	defer os.Remove(f.Name())
	cmd := exec.CommandContext(ctx, p.bin, "--model", p.model, "--output_file", f.Name())
	cmd.Stdin = bytes.NewBufferString(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("piper: %w: %s", err, lastLine(stderr.String()))
	}
	return os.ReadFile(f.Name())
}

// piperd keeps one Piper process with the voice loaded (deploy/piperd.py); restarts it on failure.
type piperd struct {
	cmd    []string
	mu     sync.Mutex
	proc   *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

func (p *piperd) start() error {
	cmd := exec.Command(p.cmd[0], p.cmd[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("piperd: %w", err)
	}
	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "READY" {
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("piperd: did not start (%q, %v)", line, err)
	}
	p.proc, p.stdin, p.stdout = cmd, stdin, r
	slog.Info("piperd ready", "cmd", strings.Join(p.cmd, " "))
	return nil
}

func (p *piperd) stop() {
	if p.proc != nil {
		p.stdin.Close()
		p.proc.Process.Kill()
		p.proc.Wait()
		p.proc = nil
	}
}

// Warm starts the process ahead of the first announcement (model load ~1 s).
func (p *piperd) Warm() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proc != nil {
		return nil
	}
	return p.start()
}

func (p *piperd) WAV(ctx context.Context, text string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proc == nil {
		if err := p.start(); err != nil {
			return nil, err
		}
	}
	f, err := os.CreateTemp("", "predavka-*.wav")
	if err != nil {
		return nil, err
	}
	f.Close()
	defer os.Remove(f.Name())
	clean := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(text)
	if _, err := fmt.Fprintf(p.stdin, "%s\t%s\n", f.Name(), clean); err != nil {
		p.stop()
		return nil, fmt.Errorf("piperd write: %w", err)
	}
	type res struct {
		line string
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		l, err := p.stdout.ReadString('\n')
		ch <- res{strings.TrimSpace(l), err}
	}()
	select {
	case <-ctx.Done():
		p.stop()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		p.stop()
		return nil, errors.New("piperd: timeout")
	case r := <-ch:
		if r.err != nil {
			p.stop()
			return nil, fmt.Errorf("piperd read: %w", r.err)
		}
		if strings.HasPrefix(r.line, "ERR ") {
			return nil, errors.New("piperd: " + r.line[4:])
		}
	}
	return os.ReadFile(f.Name())
}

type say struct{ voice string }

func (s say) WAV(ctx context.Context, text string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "predavka-say")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "out.wav")
	cmd := exec.CommandContext(ctx, "say", "-v", s.voice, "--file-format=WAVE", "--data-format=LEI16@22050", "-o", path, text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("say: %w: %s", err, out)
	}
	return os.ReadFile(path)
}

// --- outputs ---

type player struct{ cmd []string }

func (p player) Play(ctx context.Context, wav []byte, started func()) error {
	f, err := os.CreateTemp("", "predavka-play-*.wav")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(wav); err != nil {
		f.Close()
		return err
	}
	f.Close()
	args := append(append([]string(nil), p.cmd[1:]...), f.Name())
	cmd := exec.CommandContext(ctx, p.cmd[0], args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	started()
	return cmd.Wait()
}

// snapFIFO writes raw PCM into the Snapcast server pipe; all snapclients (předávka + kuchyň) play it in sync.
// Snapcast stream musí mít stejný formát jako WAV z TTS (Piper medium: 22050:16:1).
type snapFIFO struct {
	path string
	f    *os.File
}

func (s *snapFIFO) Play(ctx context.Context, wav []byte, started func()) error {
	pcm, rate, ch, err := PCM(wav)
	if err != nil {
		return err
	}
	if s.f == nil {
		// O_RDWR so open does not block when snapserver is not reading yet.
		f, err := os.OpenFile(s.path, os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("snapfifo: %w", err)
		}
		s.f = f
	}
	started()
	if _, err := s.f.Write(pcm); err != nil {
		s.f.Close()
		s.f = nil
		return err
	}
	// Writing to a pipe returns once buffered; wait for the real playback time so gaps hold.
	dur := time.Duration(len(pcm)) * time.Second / time.Duration(rate*ch*2)
	select {
	case <-ctx.Done():
	case <-time.After(dur):
	}
	return nil
}

type discard struct{}

func (discard) Play(_ context.Context, _ []byte, started func()) error { started(); return nil }

// logSpeaker only logs (engine none) — useful for tests and silent installs.
type logSpeaker struct{}

func (logSpeaker) Speak(_ context.Context, text string, started func()) error {
	started()
	slog.Info("tts (none)", "text", text)
	return nil
}
func (logSpeaker) Tone(context.Context) error { return nil }

// PCM extracts the data chunk of a 16-bit PCM WAV.
func PCM(wav []byte) (pcm []byte, rate, channels int, err error) {
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, 0, 0, errors.New("wav: not RIFF/WAVE")
	}
	r := bytes.NewReader(wav[12:])
	for {
		var hdr [8]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, 0, 0, errors.New("wav: no data chunk")
		}
		size := int(binary.LittleEndian.Uint32(hdr[4:]))
		body := make([]byte, size)
		n, _ := io.ReadFull(r, body)
		body = body[:n]
		switch string(hdr[:4]) {
		case "fmt ":
			if len(body) < 16 {
				return nil, 0, 0, errors.New("wav: short fmt")
			}
			if bits := binary.LittleEndian.Uint16(body[14:]); bits != 16 {
				return nil, 0, 0, fmt.Errorf("wav: %d-bit not supported", bits)
			}
			channels = int(binary.LittleEndian.Uint16(body[2:]))
			rate = int(binary.LittleEndian.Uint32(body[4:]))
		case "data":
			if rate == 0 {
				return nil, 0, 0, errors.New("wav: data before fmt")
			}
			return body, rate, channels, nil
		}
		if size%2 == 1 {
			r.ReadByte()
		}
	}
}

func lastLine(s string) string {
	s = string(bytes.TrimSpace([]byte(s)))
	if i := bytes.LastIndexByte([]byte(s), '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
