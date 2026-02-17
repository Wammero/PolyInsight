package main

import (
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

func main() {
	cfg := config.Load()

	db.Init(cfg.DatabaseURL)
	defer db.Close()

	cache.Init(cfg.RedisURL)

	tgBot, err := telego.NewBot(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("telegram init: %v", err)
	}

	jobs := make(chan []byte, 5000)

	// Mini App URL for settings keyboard Web App button.
	botpkg.SetMiniAppURL(cfg.MiniAppURL)

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

	// Buffer fill gauge.
	go func() {
		for {
			metrics.BufferGauge.Set(float64(len(jobs)))
			time.Sleep(time.Second)
		}
	}()

	// Worker pool: 16 parallel trade processors.
	for i := 1; i <= 16; i++ {
		go tracker.Worker(i, jobs, tgBot, cfg)
	}

	// WebSocket with automatic reconnection.
	go func() {
		for {
			log.Println("Connecting to Polymarket WebSocket...")
			if err := tracker.StartSocket(jobs); err != nil {
				log.Printf("WebSocket error: %v", err)
			}
			time.Sleep(5 * time.Second)
		}
	}()

	// Start in-memory watchlist cache (refreshed every 30s) for real-time follow detection.
	tracker.StartFollowCache()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down...")
}
