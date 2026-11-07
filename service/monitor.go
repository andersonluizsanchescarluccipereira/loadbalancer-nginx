package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

//go:embed web/*
var webFiles embed.FS

type Recorder struct {
	mu       sync.Mutex
	counts   map[string]map[string]int
	inflight map[string]Event
	events   []Event
	clients  map[chan []byte]struct{}
	total    int
}

func newRecorder() *Recorder {
	return &Recorder{counts: map[string]map[string]int{}, inflight: map[string]Event{}, clients: map[chan []byte]struct{}{}}
}
func (r *Recorder) record(e Event) {
	if e.Server != "server1" && e.Server != "server2" && e.Server != "server3" {
		return
	}
	if e.Protocol != "HTTP" && e.Protocol != "TCP" && e.Protocol != "UDP" {
		return
	}
	if e.Phase != "start" && e.Phase != "finish" {
		return
	}
	if e.ID == "" || len(e.ID) > 100 || len(e.Mode) > 40 {
		return
	}
	e.At = time.Now().Format(time.RFC3339Nano)
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.Phase == "start" {
		if _, ok := r.inflight[e.ID]; !ok {
			if r.counts[e.Mode] == nil {
				r.counts[e.Mode] = map[string]int{}
			}
			r.counts[e.Mode][e.Server]++
			r.inflight[e.ID] = e
			r.total++
		}
	} else {
		delete(r.inflight, e.ID)
	}
	r.events = append(r.events, e)
	if len(r.events) > 250 {
		r.events = append([]Event(nil), r.events[len(r.events)-250:]...)
	}
	raw, _ := json.Marshal(e)
	for client := range r.clients {
		select {
		case client <- raw:
		default:
		}
	}
}
func (r *Recorder) snapshot() any {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := map[string]map[string]int{}
	for mode, values := range r.counts {
		counts[mode] = map[string]int{}
		for server, n := range values {
			counts[mode][server] = n
		}
	}
	active := map[string]int{"server1": 0, "server2": 0, "server3": 0}
	for _, e := range r.inflight {
		active[e.Server]++
	}
	return map[string]any{"counts": counts, "active": active, "events": append([]Event{}, r.events...), "total": r.total}
}
func (r *Recorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts = map[string]map[string]int{}
	r.inflight = map[string]Event{}
	r.events = nil
	r.total = 0
	for client := range r.clients {
		select {
		case client <- []byte(`{"kind":"reset"}`):
		default:
		}
	}
}
func (r *Recorder) stream(w http.ResponseWriter, req *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE indisponivel", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	ch := make(chan []byte, 256)
	r.mu.Lock()
	r.clients[ch] = struct{}{}
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.clients, ch); r.mu.Unlock() }()
	_, _ = io.WriteString(w, ": conectado\n\n")
	f.Flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-req.Context().Done():
			return
		case raw := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
				return
			}
			f.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			f.Flush()
		}
	}
}
func monitorHandler(r *Recorder) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/event", func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, 4096)
		var e Event
		if err := json.NewDecoder(req.Body).Decode(&e); err != nil {
			http.Error(w, "JSON invalido", 400)
			return
		}
		r.record(e)
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("GET /dashboard/api/stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(r.snapshot())
	})
	mux.HandleFunc("POST /dashboard/api/reset", func(w http.ResponseWriter, _ *http.Request) { r.reset(); w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /dashboard/api/events", r.stream)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return mux
}
