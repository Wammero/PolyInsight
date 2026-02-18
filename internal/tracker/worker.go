package tracker

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mymmrac/telego"

	"polymarket_go/internal/bot"
	"polymarket_go/internal/config"
	"polymarket_go/internal/metrics"
	"polymarket_go/internal/polymarket"
)

// sharedClient is reused across all workers for connection pool efficiency.
// 32 workers share one pool instead of maintaining 32 separate pools.
var sharedClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 40,
		IdleConnTimeout:     90 * time.Second,
	},
}

// wsMessage is the raw Polymarket WebSocket envelope.
type wsMessage struct {
	Type    string                  `json:"type"`
	Payload polymarket.TradePayload `json:"payload"`
}

// Worker reads raw trade bytes from jobs, applies filters, and dispatches alerts.
func Worker(id int, jobs <-chan []byte, b *telego.Bot, cfg *config.Config) {
	ctx := context.Background()

	for raw := range jobs {
		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil || msg.Type != "trades" {
			continue
		}

		p := msg.Payload
		volume := p.Size * p.Price

		// Global pre-filter: only absolute minimum volume.
		if volume < cfg.MinWhaleAmount {
			metrics.FilteredVolume.Inc()
			continue
		}
		metrics.WhaleEvents.Inc()

		startTime := time.Now()

		// Title ban-word filter.
		if containsBanWord(strings.ToLower(p.Title)) {
			metrics.FilteredBanWord.Inc()
			continue
		}

		// Fetch category metadata (cached).
		meta := polymarket.GetMeta(ctx, p.Slug, sharedClient)
		if meta == nil {
			metrics.FilteredNoMeta.Inc()
			continue
		}

		// Newbie detection: fetch trade count.
		trades := polymarket.GetTradeCount(ctx, sharedClient, p.ProxyWallet)
		if trades == 999 {
			metrics.FilteredAPIErr.Inc()
			continue // API error — skip to avoid false positives
		}

		// Fetch enriched wallet stats for display.
		stats := polymarket.GetWalletStats(ctx, sharedClient, p.ProxyWallet)

		// Get watchlist followers for this wallet (from in-memory cache).
		followers := GetFollowers(p.ProxyWallet)

		// Broadcast regular alert to all matching subscribers.
		bot.SendAlert(ctx, b, p, trades, stats, meta, startTime, cfg.BaseURL, followers)

		// Send dedicated "your followed wallet traded" notification to followers.
		if len(followers) > 0 {
			bot.SendFollowAlert(ctx, b, p, trades, stats, meta, followers)
			metrics.FollowAlerts.Add(float64(len(followers)))
		}

		metrics.WorkerDuration.Observe(time.Since(startTime).Seconds())
	}
}

func containsBanWord(title string) bool {
	for _, w := range polymarket.BanWords {
		if strings.Contains(title, w) {
			return true
		}
	}
	return false
}
