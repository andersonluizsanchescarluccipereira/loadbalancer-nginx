package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Backend struct {
	name string
	port int
	emit func(Event)
}

// A telemetria vai para o monitor sem bloquear o processamento HTTP/TCP/UDP.
func eventDispatcher(url string) func(Event) {
	queue := make(chan Event, 8192)
	go func() {
		client := &http.Client{Timeout: 650 * time.Millisecond}
		for e := range queue {
			payload, _ := json.Marshal(e)
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := client.Do(req)
			if err == nil {
				_ = response.Body.Close()
			}
		}
	}()
	return func(e Event) {
		select {
		case queue <- e:
		default:
			log.Printf("event queue cheia; perdendo id=%s", e.ID)
		}
	}
}
func (b Backend) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprintf(w, "ok %s:%d\n", b.name, b.port) })
	mux.HandleFunc("GET /api/visit", func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query()
		id := req.Header.Get("X-Request-ID")
		if len(id) < 1 || len(id) > 100 {
			id = newID()
		}
		mode := req.Header.Get("X-Lab-Mode")
		if mode == "" || len(mode) > 40 {
			mode = "direct"
		}
		delay := ms(query.Get("delay"), 0)
		if query.Get("slow") != "" {
			if query.Get("slow") == b.name {
				delay = ms(query.Get("slow_ms"), 3500)
			} else {
				delay = ms(query.Get("fast_ms"), 70)
			}
		}
		start := time.Now()
		b.emit(Event{ID: id, Server: b.name, Port: b.port, Protocol: "HTTP", Mode: mode, Phase: "start", DelayMs: delay})
		defer func() {
			b.emit(Event{ID: id, Server: b.name, Port: b.port, Protocol: "HTTP", Mode: mode, Phase: "finish", DurationMs: time.Since(start).Milliseconds()})
		}()
		timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-req.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Backend-Server", b.name)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "server": b.name, "port": b.port, "protocol": "HTTP", "layer": 7, "mode": mode, "delayMs": delay, "durationMs": time.Since(start).Milliseconds(), "at": time.Now().Format(time.RFC3339Nano)})
	})
	return mux
}

// Uma conexão TCP transmite exatamente uma linha e recebe uma linha como resposta.
func listenPort(key, fallback string) string {
	port := os.Getenv(key)
	if port == "" {
		return fallback
	}
	return port
}

func (b Backend) serveTCP() error {
	ln, err := net.Listen("tcp", ":"+listenPort("TCP_PORT", "9000"))
	if err != nil {
		return err
	}
	log.Printf("%s TCP :%s", b.name, listenPort("TCP_PORT", "9000"))
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go b.handleTCP(conn)
	}
}
func readMessage(message, defaultMode string) (mode, id string, delay int) {
	mode = defaultMode
	id = newID()
	for _, field := range strings.Fields(message) {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "id":
			if len(parts[1]) > 0 && len(parts[1]) <= 100 {
				id = parts[1]
			}
		case "mode":
			if len(parts[1]) <= 40 && strings.HasPrefix(parts[1], "tcp-") {
				mode = parts[1]
			}
		case "delay":
			delay = ms(parts[1], 0)
		}
	}
	return
}
func (b Backend) handleTCP(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	msg, err := bufio.NewReader(io.LimitReader(conn, 1024)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return
	}
	mode, id, delay := readMessage(msg, "tcp-rr")
	start := time.Now()
	b.emit(Event{ID: id, Server: b.name, Port: b.port, Protocol: "TCP", Mode: mode, Phase: "start", DelayMs: delay})
	time.Sleep(time.Duration(delay) * time.Millisecond)
	_, _ = fmt.Fprintf(conn, "id=%s server=%s port=%d protocol=TCP delay=%d\n", id, b.name, b.port, delay)
	b.emit(Event{ID: id, Server: b.name, Port: b.port, Protocol: "TCP", Mode: mode, Phase: "finish", DurationMs: time.Since(start).Milliseconds()})
}
func (b Backend) serveUDP() error {
	conn, err := net.ListenPacket("udp", ":"+listenPort("UDP_PORT", "9001"))
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("%s UDP :%s", b.name, listenPort("UDP_PORT", "9001"))
	buf := make([]byte, 2048)
	for {
		n, remote, err := conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		msg := string(buf[:n])
		go func() {
			_, id, _ := readMessage(msg, "udp-rr")
			start := time.Now()
			b.emit(Event{ID: id, Server: b.name, Port: b.port, Protocol: "UDP", Mode: "udp-rr", Phase: "start"})
			_, _ = conn.WriteTo([]byte(fmt.Sprintf("id=%s server=%s port=%d protocol=UDP\n", id, b.name, b.port)), remote)
			b.emit(Event{ID: id, Server: b.name, Port: b.port, Protocol: "UDP", Mode: "udp-rr", Phase: "finish", DurationMs: time.Since(start).Milliseconds()})
		}()
	}
}
