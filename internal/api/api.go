// Package api exposes the HTTP API and the embedded web displays (docs/protocol.md).
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/lioilsources/Kitchen/internal/queue"
)

type Server struct {
	Engine  *queue.Engine
	WS      http.Handler
	Web     fs.FS
	App     fs.FS // volitelně Flutter web build (apps/display/build/web), servírovaný na /app/
	Art     fs.FS // volitelně obrázky jídel (config art_dir), servírované na /art/
	Latency func() time.Duration
	Clients func() int
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /ws", s.WS)
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Engine.Snapshot()) })
	mux.HandleFunc("POST /api/orders", s.postOrder)
	mux.HandleFunc("POST /api/items/{id}/{action}", s.itemAction)
	mux.HandleFunc("POST /api/stations/{id}/bump", s.stationBump)
	mux.HandleFunc("POST /api/tables/{table}/{action}", s.tableAction)
	mux.HandleFunc("POST /api/repeat", func(w http.ResponseWriter, r *http.Request) {
		s.Engine.Repeat()
		writeJSON(w, 200, ok())
	})
	mux.HandleFunc("POST /api/quiet", s.quiet)
	mux.HandleFunc("GET /api/menu", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Engine.Menu()) })
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Engine.Settings()) })
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"prep":             s.Engine.PrepStats(),
			"voice_latency_ms": s.Latency().Milliseconds(),
			"displays":         s.Clients(),
		})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	if s.App != nil {
		mux.Handle("GET /app/", http.StripPrefix("/app/", http.FileServerFS(s.App)))
	}
	if s.Art != nil {
		mux.Handle("GET /art/", http.StripPrefix("/art/", http.FileServerFS(s.Art)))
	}
	mux.Handle("GET /", http.FileServerFS(s.Web))
	return logRequests(mux)
}

func (s *Server) postOrder(w http.ResponseWriter, r *http.Request) {
	var in queue.NewOrder
	if !readJSON(w, r, &in) {
		return
	}
	if in.Source == "" {
		in.Source = "tablet"
	}
	o, err := s.Engine.AddOrder(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, o)
}

var itemActions = map[string]bool{
	queue.ActBump: true, queue.ActClaim: true, queue.ActStart: true, queue.ActReady: true, queue.ActPick: true,
	queue.ActServe: true, queue.ActVoid: true, queue.ActHold: true, queue.ActFire: true,
}

type actor struct {
	By string `json:"by"`
}

func (s *Server) itemAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if !itemActions[action] {
		writeJSON(w, 404, errBody("unknown action"))
		return
	}
	var a actor
	if r.ContentLength > 0 && !readJSON(w, r, &a) {
		return
	}
	it, err := s.Engine.Transition(r.PathValue("id"), action, a.By)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) stationBump(w http.ResponseWriter, r *http.Request) {
	var a actor
	if r.ContentLength > 0 && !readJSON(w, r, &a) {
		return
	}
	if a.By == "" {
		a.By = r.PathValue("id")
	}
	it, err := s.Engine.BumpStation(r.PathValue("id"), a.By)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, it)
}

func (s *Server) tableAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != queue.ActPick && action != queue.ActFire {
		writeJSON(w, 404, errBody("unknown action"))
		return
	}
	n, err := s.Engine.TableAction(r.PathValue("table"), action, "expo")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"items": n})
}

func (s *Server) quiet(w http.ResponseWriter, r *http.Request) {
	var in struct {
		On bool `json:"on"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.Engine.SetQuiet(in.On); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, ok())
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	in := s.Engine.Settings()
	if !readJSON(w, r, &in) {
		return
	}
	if err := s.Engine.SetSettings(in); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		writeJSON(w, 400, errBody("bad json: "+err.Error()))
		return false
	}
	return true
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, queue.ErrNotFound):
		writeJSON(w, 404, errBody(err.Error()))
	case errors.Is(err, queue.ErrTransition):
		writeJSON(w, 409, errBody(err.Error()))
	default:
		writeJSON(w, 400, errBody(err.Error()))
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write json", "err", err)
	}
}

func errBody(msg string) map[string]string { return map[string]string{"error": msg} }
func ok() map[string]bool                  { return map[string]bool{"ok": true} }

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS pro Flutter web / displeje servírované odjinud (LAN, bez autentizace).
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(204)
			return
		}
		if r.Method != http.MethodGet {
			slog.Debug("http", "method", r.Method, "path", r.URL.Path)
		}
		h.ServeHTTP(w, r)
	})
}
