package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"polymarket_go/internal/cache"
)

// TradePayload mirrors the Polymarket WebSocket trade payload.
type TradePayload struct {
	Asset           string  `json:"asset"`
	Slug            string  `json:"slug"`
	Title           string  `json:"title"`
	Outcome         string  `json:"outcome"`
	Price           float64 `json:"price"`
	Size            float64 `json:"size"`
	Side            string  `json:"side"`
	ProxyWallet     string  `json:"proxyWallet"`
	Pseudonym       string  `json:"pseudonym"`
	TransactionHash string  `json:"transactionHash"`
	Timestamp       int64   `json:"timestamp"`
}

// WalletStats is enriched wallet data shown to users.
type WalletStats struct {
	TotalTrades int     `json:"total_trades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	WinRate     float64 `json:"win_rate"` // 0-100, -1 = unknown
	TotalPnL    float64 `json:"total_pnl"` // ALL-time USD, -999999 = unknown
}

// GetTradeCount returns the number of trades ever made by the wallet.
// Returns 999 on any error (worker skips such wallets to avoid false positives).
func GetTradeCount(ctx context.Context, client *http.Client, wallet string) int {
	// Normalize wallet address to lowercase for consistent API calls and caching
	wallet = strings.ToLower(wallet)
	cacheKey := "trades:" + wallet
	if val, ok := cache.GetString(ctx, cacheKey); ok {
		count, err := strconv.Atoi(val)
		if err != nil {
			// Corrupt cache entry — treat as API error, not as "zero trades".
			return 999
		}
		return count
	}

	resp, err := client.Get("https://data-api.polymarket.com/traded?user=" + wallet)
	if err != nil || resp == nil {
		return 999
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return 999
	}

	var d struct {
		Traded int `json:"traded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 999 // JSON parse error — treat as API failure to avoid false "0 trades"
	}
	cache.SetString(ctx, cacheKey, strconv.Itoa(d.Traded), 10*time.Minute)
	return d.Traded
}

// GetWalletStats fetches enriched stats for display.
// Cached 15 minutes per wallet. P&L and closed-positions are fetched in parallel.
func GetWalletStats(ctx context.Context, client *http.Client, wallet string) *WalletStats {
	// Normalize wallet address to lowercase for consistent API calls and caching
	wallet = strings.ToLower(wallet)
	cacheKey := "wstats:" + wallet
	var cached WalletStats
	if cache.GetJSON(ctx, cacheKey, &cached) {
		return &cached
	}

	stats := &WalletStats{WinRate: -1, TotalPnL: -999999}
	stats.TotalTrades = GetTradeCount(ctx, client, wallet)

	// Fetch P&L and closed positions in parallel to halve latency.
	// Each goroutine writes to its own local variables — no shared-state data race.
	var (
		pnl          float64 = -999999
		wins, losses int
		winRate      float64 = -1
	)

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: all-time P&L from leaderboard endpoint.
	go func() {
		defer wg.Done()
		resp, err := client.Get(fmt.Sprintf(
			"https://data-api.polymarket.com/v1/leaderboard?user=%s&timePeriod=ALL", wallet,
		))
		if err != nil || resp == nil {
			return
		}
		defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
		if resp.StatusCode != 200 {
			return
		}
		var rows []struct {
			PnL float64 `json:"pnl"`
		}
		if json.NewDecoder(resp.Body).Decode(&rows) == nil && len(rows) > 0 {
			pnl = rows[0].PnL
		}
	}()

	// Goroutine 2: win/loss counts from closed positions.
	// realizedPnl > 0 = win, realizedPnl <= 0 = loss.
	go func() {
		defer wg.Done()
		resp, err := client.Get(fmt.Sprintf(
			"https://data-api.polymarket.com/closed-positions?user=%s&limit=500", wallet,
		))
		if err != nil || resp == nil {
			return
		}
		defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
		if resp.StatusCode != 200 {
			return
		}
		var closed []struct {
			RealizedPnl float64 `json:"realizedPnl"`
		}
		if json.NewDecoder(resp.Body).Decode(&closed) == nil {
			w, l := 0, 0
			for _, pos := range closed {
				if pos.RealizedPnl > 0 {
					w++
				} else {
					l++
				}
			}
			wins, losses = w, l
			if w+l > 0 {
				winRate = float64(w) / float64(w+l) * 100
			}
		}
	}()

	wg.Wait()

	// Merge goroutine results into stats (safe: both goroutines have finished).
	stats.TotalPnL = pnl
	stats.Wins = wins
	stats.Losses = losses
	stats.WinRate = winRate

	cache.SetJSON(ctx, cacheKey, stats, 15*time.Minute)
	return stats
}
