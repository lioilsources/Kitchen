package escpos

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// maxJob caps one print job held in memory (bony jsou malé; obrázky/loga se jen přepošlou).
const maxJob = 1 << 20

// Proxy listens like a network printer (RAW 9100), forwards everything to the real printer
// and hands every parsed bon to OnTicket.
type Proxy struct {
	Listen   string
	Forward  string // "" = jen odposlech (tiskárna není připojena — např. test s `nc`)
	Parser   *Parser
	OnTicket func(Ticket)
}

func (p *Proxy) Run(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", p.Listen)
	if err != nil {
		return err
	}
	slog.Info("escpos proxy listening", "addr", p.Listen, "forward", p.Forward)
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("escpos accept", "err", err)
			time.Sleep(time.Second)
			continue
		}
		go p.handle(ctx, c)
	}
}

func (p *Proxy) handle(ctx context.Context, pos net.Conn) {
	defer pos.Close()
	var printer net.Conn
	if p.Forward != "" {
		d := net.Dialer{Timeout: 3 * time.Second}
		var err error
		printer, err = d.DialContext(ctx, "tcp", p.Forward)
		if err != nil {
			// Bon se stejně zpracuje (kuchyň ho uvidí na displeji), jen se nevytiskne.
			slog.Error("escpos: printer unreachable, bon will not be printed", "printer", p.Forward, "err", err)
		} else {
			defer printer.Close()
		}
	}
	var wg sync.WaitGroup
	if printer != nil {
		// Stavové odpovědi tiskárny (DLE EOT) zpět do pokladny.
		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(pos, printer)
		}()
	}
	var job []byte
	buf := make([]byte, 32*1024)
	for {
		pos.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := pos.Read(buf)
		if n > 0 {
			if printer != nil {
				if _, werr := printer.Write(buf[:n]); werr != nil {
					slog.Error("escpos: forward failed", "err", werr)
					printer.Close()
					printer = nil
				}
			}
			if len(job)+n <= maxJob {
				job = append(job, buf[:n]...)
			}
		}
		if err != nil {
			var ne net.Error
			if !errors.Is(err, io.EOF) && !(errors.As(err, &ne) && ne.Timeout()) {
				slog.Warn("escpos read", "err", err)
			}
			break
		}
	}
	if printer != nil {
		if tc, ok := printer.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
		printer.SetReadDeadline(time.Now().Add(2 * time.Second)) // dočíst případné stavové odpovědi
	}
	p.process(job)
	wg.Wait()
}

func (p *Proxy) process(job []byte) {
	for _, bon := range Split(job) {
		t, ok := p.Parser.Parse(bon)
		if !ok {
			slog.Info("escpos: not a kitchen bon (skipped)", "text", t.Raw)
			continue
		}
		slog.Info("escpos: bon", "table", t.Table, "lines", len(t.Lines))
		if p.OnTicket != nil {
			p.OnTicket(t)
		}
	}
}
