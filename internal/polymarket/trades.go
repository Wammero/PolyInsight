package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
// Returns 999 on error (worker skips such wallets).
func GetTradeCount(ctx context.Context, client *http.Client, wallet string) int {
	cacheKey := "trades:" + wallet
	if val, ok := cache.GetString(ctx, cacheKey); ok {
		count, _ := strconv.Atoi(val)
		return count
	}

	resp, err := client.Get("https://data-api.polymarket.com/traded?user=" + wallet)
	if err != nil || resp == nil {
		return 999
	}
	// Always drain and close — prevents TCP connection leaks.
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return 999
	}

	var d struct {
		Traded int `json:"traded"`
	}
	json.NewDecoder(resp.Body).Decode(&d)
	cache.SetString(ctx, cacheKey, strconv.Itoa(d.Traded), 10*time.Minute)
	return d.Traded
}

// GetWalletStats fetches enriched stats for display.
// Cached 15 minutes per wallet. Never blocks the alert pipeline on API errors.
func GetWalletStats(ctx context.Context, client *http.Client, wallet string) *WalletStats {
	cacheKey := "wstats:" + wallet
	var cached WalletStats
	if cache.GetJSON(ctx, cacheKey, &cached) {
		return &cached
	}

	stats := &WalletStats{WinRate: -1, TotalPnL: -999999}
	stats.TotalTrades = GetTradeCount(ctx, client, wallet)

	// All-time P&L from leaderboard endpoint.
	if resp, err := client.Get(fmt.Sprintf(
		"https://data-api.polymarket.com/v1/leaderboard?user=%s&timePeriod=ALL", wallet,
	)); err == nil && resp != nil {
		defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
		if resp.StatusCode == 200 {
			var rows []struct {
				PnL float64 `json:"pnl"`
			}
			if json.NewDecoder(resp.Body).Decode(&rows) == nil && len(rows) > 0 {
				stats.TotalPnL = rows[0].PnL
			}
		}
	}

	// Closed positions: win/loss by realized profit (matches what user sees on Polymarket).
	// realizedPnl > 0 = win, realizedPnl <= 0 = loss.
	if resp, err := client.Get(fmt.Sprintf(
		"https://data-api.polymarket.com/closed-positions?user=%s&limit=500", wallet,
	)); err == nil && resp != nil {
		defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
		if resp.StatusCode == 200 {
			var closed []struct {
				RealizedPnl float64 `json:"realizedPnl"`
			}
			if json.NewDecoder(resp.Body).Decode(&closed) == nil {
				wins, losses := 0, 0
				for _, pos := range closed {
					if pos.RealizedPnl > 0 {
						wins++
					} else {
						losses++
					}
				}
				stats.Wins = wins
				stats.Losses = losses
				if wins+losses > 0 {
					stats.WinRate = float64(wins) / float64(wins+losses) * 100
				}
			}
		}
	}

	cache.SetJSON(ctx, cacheKey, stats, 15*time.Minute)
	return stats
}

// RecentTrades returns the N most recent tx hashes for a wallet (for favorites monitor).
func RecentTrades(ctx context.Context, client *http.Client, wallet string, limit int) []string {
	resp, err := client.Get(fmt.Sprintf(
		"https://data-api.polymarket.com/activity?user=%s&limit=%d", wallet, limit,
	))
	if err != nil || resp == nil {
		return nil
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil
	}

	var rows []struct {
		TransactionHash string `json:"transactionHash"`
	}
	json.NewDecoder(resp.Body).Decode(&rows)

	hashes := make([]string, 0, len(rows))
	for _, r := range rows {
		hashes = append(hashes, r.TransactionHash)
	}
	return hashes
}
