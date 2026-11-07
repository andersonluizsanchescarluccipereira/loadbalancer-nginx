package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVisitIdentifiesServer(t *testing.T) {
	var events []Event
	b := Backend{name: "server2", port: 8082, emit: func(e Event) { events = append(events, e) }}
	request := httptest.NewRequest("GET", "/api/visit?delay=0", nil)
	request.Header.Set("X-Lab-Mode", "weighted")
	writer := httptest.NewRecorder()
	b.handler().ServeHTTP(writer, request)
	if writer.Code != 200 {
		t.Fatalf("got %d", writer.Code)
	}
	var result struct {
		Server string `json:"server"`
		Port   int    `json:"port"`
		Mode   string `json:"mode"`
	}
	if err := json.Unmarshal(writer.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Server != "server2" || result.Port != 8082 || result.Mode != "weighted" {
		t.Fatalf("unexpected response: %+v", result)
	}
	if len(events) != 2 || events[0].Phase != "start" || events[1].Phase != "finish" {
		t.Fatalf("bad events: %+v", events)
	}
}
func TestMonitorCountsAndReset(t *testing.T) {
	r := newRecorder()
	r.record(Event{ID: "x", Server: "server1", Protocol: "TCP", Mode: "tcp-rr", Phase: "start"})
	if r.total != 1 || len(r.inflight) != 1 {
		t.Fatal("start not counted")
	}
	r.record(Event{ID: "x", Server: "server1", Protocol: "TCP", Mode: "tcp-rr", Phase: "finish"})
	if r.total != 1 || len(r.inflight) != 0 {
		t.Fatal("finish must not increase count")
	}
	w := httptest.NewRecorder()
	monitorHandler(r).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dashboard/api/stats", nil))
	if w.Code != 200 {
		t.Fatalf("HTTP=%d", w.Code)
	}
	r.reset()
	if r.total != 0 {
		t.Fatal("reset error")
	}
}
func TestDelayBounds(t *testing.T) {
	if ms("50000", 0) != 10000 || ms("-1", 0) != 0 || ms("bad", 40) != 40 {
		t.Fatal("unexpected delay limit")
	}
}
