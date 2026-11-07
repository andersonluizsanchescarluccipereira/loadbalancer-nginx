package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Event struct {
	ID         string `json:"id"`
	Server     string `json:"server"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	Mode       string `json:"mode"`
	Phase      string `json:"phase"`
	DelayMs    int    `json:"delayMs"`
	DurationMs int64  `json:"durationMs"`
	At         string `json:"at"`
}

func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprint(time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
func ms(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if n < 0 {
		return 0
	}
	if n > 10000 {
		return 10000
	}
	return n
}
func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if os.Getenv("ROLE") == "monitor" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "7070"
		}
		log.Printf("monitor em :%s", port)
		log.Fatal((&http.Server{Addr: ":" + port, Handler: monitorHandler(newRecorder()), ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
	}
	if os.Getenv("ROLE") != "backend" {
		log.Fatal("ROLE deve ser backend ou monitor")
	}
	name := os.Getenv("SERVER_ID")
	if name != "server1" && name != "server2" && name != "server3" {
		log.Fatalf("SERVER_ID invalido %q", name)
	}
	port, err := strconv.Atoi(os.Getenv("HTTP_PORT"))
	if err != nil || port < 1 || port > 65535 {
		log.Fatal("HTTP_PORT invalida")
	}
	url := os.Getenv("MONITOR_URL")
	if url == "" {
		url = "http://localhost:7070/internal/event"
	}
	b := Backend{name: name, port: port, emit: eventDispatcher(url)}
	go func() { log.Fatal(b.serveTCP()) }()
	go func() { log.Fatal(b.serveUDP()) }()
	log.Printf("backend=%s HTTP :%d / TCP :%s / UDP :%s", name, port, listenPort("TCP_PORT", "9000"), listenPort("UDP_PORT", "9001"))
	log.Fatal((&http.Server{Addr: fmt.Sprintf(":%d", port), Handler: b.handler(), ReadHeaderTimeout: 5 * time.Second}).ListenAndServe())
}
