package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mymmrac/telego"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	botpkg "polymarket_go/internal/bot"
	"polymarket_go/internal/cache"
	"polymarket_go/internal/config"
	"polymarket_go/internal/db"
	"polymarket_go/internal/metrics"
	"polymarket_go/internal/tracker"
)

const bufferSize = 5000

func main() {
	cfg := config.Load()

	db.Init(cfg.DatabaseURL)
	defer db.Close()

	cache.Init(cfg.RedisURL)

	tgBot, err := telego.NewBot(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("telegram init: %v", err)
	}

	jobs := make(chan []byte, bufferSize)

	// Constant metrics set once at startup.
	metrics.BufferCapacity.Set(bufferSize)
	metrics.WorkerCount.Set(32)

	// Mini App URL for settings keyboard Web App button.
	botpkg.SetMiniAppURL(cfg.MiniAppURL)

	// In-memory subscriber cache — eliminates DB query per whale event.
	// ActiveUsers metric is updated inside StartSubCache on every refresh.
	botpkg.StartSubCache()

	// Rate-limited Telegram send queue — 25 msg/sec, capacity 20 000.
	// All outgoing alerts go through Enqueue(), never directly via b.SendMessage.
	botpkg.StartSendQueue(tgBot)

	// Telegram update handlers (commands + callback queries + Mini App data).
	go botpkg.StartHandlers(tgBot)

	// HTTP server: Prometheus metrics + optional redirect tracking.
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if cfg.BaseURL != "" {
			mux.HandleFunc("/r/", botpkg.HandleRedirect)
			log.Printf("Redirect tracking enabled at %s/r/<shortID>", cfg.BaseURL)
		}
		log.Fatal(http.ListenAndServe(":2112", mux))
	}()

	// Buffer fill gauge — updated every second.
	go func() {
		for {
			metrics.BufferGauge.Set(float64(len(jobs)))
			time.Sleep(time.Second)
		}
	}()

	// Watchlist entries gauge — updated every 30s.
	go func() {
		ctx := context.Background()
		for {
			if entries, err := db.CountWatchlistEntries(ctx); err == nil {
				metrics.WatchlistEntries.Set(float64(entries))
			}
			time.Sleep(30 * time.Second)
		}
	}()

	// Worker pool: 32 parallel trade processors.
	for i := 1; i <= 32; i++ {
		go tracker.Worker(i, jobs, tgBot, cfg)
	}

	// WebSocket with automatic reconnection.
	go func() {
		for {
			log.Println("Connecting to Polymarket WebSocket...")
			if err := tracker.StartSocket(jobs); err != nil {
				log.Printf("WebSocket error: %v", err)
				metrics.WSReconnects.Inc()
			}
			time.Sleep(5 * time.Second)
		}
	}()

	// In-memory watchlist cache — refreshed every 30s for real-time follow detection.
	tracker.StartFollowCache()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down...")
}
