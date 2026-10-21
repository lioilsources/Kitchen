// predavka-server: one binary on the restaurant LAN — ingest, queue, ETA, escalations, TTS, displays.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lioilsources/Kitchen/internal/api"
	"github.com/lioilsources/Kitchen/internal/config"
	"github.com/lioilsources/Kitchen/internal/eta"
	"github.com/lioilsources/Kitchen/internal/hub"
	"github.com/lioilsources/Kitchen/internal/ingest/escpos"
	"github.com/lioilsources/Kitchen/internal/notify"
	"github.com/lioilsources/Kitchen/internal/queue"
	"github.com/lioilsources/Kitchen/internal/store"
	"github.com/lioilsources/Kitchen/internal/tts"
	"github.com/lioilsources/Kitchen/web"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config")
	debug := flag.Bool("debug", false, "debug logging")
	flag.Parse()
	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	if err := run(*cfgPath); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(cfg.DataFile)
	if err != nil {
		return err
	}
	defer st.Close()
	learner, err := eta.NewLearner(st, cfg.MenuBySKU())
	if err != nil {
		return err
	}
	speaker, err := tts.New(cfg.TTS)
	if err != nil {
		return err
	}

	var h *hub.Hub
	webhook := notify.NewWebhook(cfg.Notify.WebhookURL, cfg.Notify.Kinds)
	ann := tts.NewAnnouncer(speaker, cfg.Settings.AnnounceMinGap.D(), func(a tts.Announcement) {
		h.Send("announce", a)
		webhook.Send(string(a.Kind), a.Text, a.Table)
	})
	eng, err := queue.New(cfg, st, learner, ann)
	if err != nil {
		return err
	}
	h = hub.New(func() any { return eng.Snapshot() })
	eng.OnChange(h.Notify)

	go ann.Run(ctx)
	go eng.Run(ctx)
	go h.Run(ctx)

	if cfg.EscPOS.Enabled {
		p, err := escpos.NewParser(cfg.EscPOS.Charset, cfg.EscPOS.TableRe, cfg.EscPOS.ItemRe, cfg.EscPOS.ModRe, cfg.EscPOS.CourseRe)
		if err != nil {
			return err
		}
		proxy := &escpos.Proxy{Listen: cfg.EscPOS.Listen, Forward: cfg.EscPOS.Forward, Parser: p,
			OnTicket: func(t escpos.Ticket) {
				in := queue.NewOrder{Table: t.Table, Source: "escpos"}
				for _, l := range t.Lines {
					in.Items = append(in.Items, queue.OrderLine{Name: l.Name, Qty: l.Qty, Mods: l.Mods, Course: l.Course})
				}
				if _, err := eng.AddOrder(in); err != nil {
					slog.Error("escpos order", "err", err)
				}
			}}
		go func() {
			if err := proxy.Run(ctx); err != nil {
				slog.Error("escpos proxy", "err", err)
				stop()
			}
		}()
	}

	apiSrv := &api.Server{Engine: eng, WS: h, Web: web.FS, Latency: ann.LastLatency, Clients: h.Clients}
	if cfg.AppDir != "" {
		apiSrv.App = os.DirFS(cfg.AppDir)
	}
	if cfg.ArtDir != "" {
		apiSrv.Art = os.DirFS(cfg.ArtDir)
	}
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           apiSrv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	slog.Info("predavka-server listening", "addr", cfg.Listen, "tts", cfg.TTS.Engine+"/"+cfg.TTS.Output, "escpos", cfg.EscPOS.Enabled)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
