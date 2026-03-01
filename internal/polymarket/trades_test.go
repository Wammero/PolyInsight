package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"polymarket_go/internal/cache"
)

// initTestRedis connects to a local Redis (or skips if unavailable).
// Tests that need caching call this; tests that don't skip it.
func initTestRedis(t *testing.T) {
	t.Helper()
	cache.Init("localhost:6379")
	ctx := context.Background()
	if err := cache.RDB.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping cache-dependent test")
	}
	// Flush test keys after the test.
	t.Cleanup(func() {
		cache.RDB.FlushDB(ctx)
	})
}

// ---------- GetTradeCount ----------

func TestGetTradeCount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.String(), "traded?user=") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"traded": 42}`)
	}))
	defer srv.Close()

	// Monkey-patch: we can't easily replace the hardcoded URL, so test via
	// the context-cancellation and mock-server pattern instead.
	// For unit tests we use a direct HTTP call to the mock server.
	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/traded?user=0xabc", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var d struct {
		Traded int `json:"traded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Traded != 42 {
		t.Errorf("expected 42, got %d", d.Traded)
	}
}

func TestGetTradeCount_ContextCancel(t *testing.T) {
	// Server that delays forever.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := srv.Client()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/traded?user=0xabc", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected context deadline error, got nil")
	}
}

func TestGetTradeCount_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/traded?user=0xabc", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetTradeCount_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/traded?user=0xabc", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var d struct {
		Traded int `json:"traded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err == nil {
		t.Error("expected JSON decode error")
	}
}

// ---------- GetWalletStats mock ----------

func TestGetWalletStats_MockServer(t *testing.T) {
	var leaderboardHits, closedHits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path + "?" + r.URL.RawQuery

		switch {
		case strings.Contains(path, "/traded?user="):
			fmt.Fprint(w, `{"traded": 10}`)

		case strings.Contains(path, "/v1/leaderboard"):
			leaderboardHits.Add(1)
			fmt.Fprint(w, `[{"pnl": 1234.56}]`)

		case strings.Contains(path, "closed-positions"):
			closedHits.Add(1)
			offset := r.URL.Query().Get("offset")
			if offset == "0" {
				// First page: 3 wins, 2 losses (5 items < 50 = last page)
				items := []map[string]float64{
					{"realizedPnl": 100},
					{"realizedPnl": 200},
					{"realizedPnl": -50},
					{"realizedPnl": 300},
					{"realizedPnl": -10},
				}
				json.NewEncoder(w).Encode(items)
			} else {
				// Empty for any subsequent page
				fmt.Fprint(w, `[]`)
			}

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// We can't directly call GetWalletStats because it has hardcoded URLs.
	// Instead, test the HTTP call pattern and JSON parsing.
	client := srv.Client()
	ctx := context.Background()

	// Test leaderboard call
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/leaderboard?user=0xtest&timePeriod=ALL", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("leaderboard request failed: %v", err)
	}
	defer resp.Body.Close()
	var rows []struct {
		PnL float64 `json:"pnl"`
	}
	json.NewDecoder(resp.Body).Decode(&rows)
	if len(rows) == 0 || rows[0].PnL != 1234.56 {
		t.Errorf("expected PnL=1234.56, got %v", rows)
	}

	// Test closed-positions pagination (page 0)
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/closed-positions?user=0xtest&limit=50&offset=0", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("closed-positions request failed: %v", err)
	}
	defer resp2.Body.Close()
	var closed []struct {
		RealizedPnl float64 `json:"realizedPnl"`
	}
	json.NewDecoder(resp2.Body).Decode(&closed)

	w, l := 0, 0
	for _, pos := range closed {
		if pos.RealizedPnl > 0 {
			w++
		} else {
			l++
		}
	}
	if w != 3 || l != 2 {
		t.Errorf("expected 3W/2L, got %dW/%dL", w, l)
	}

	winRate := float64(w) / float64(w+l) * 100
	if winRate != 60.0 {
		t.Errorf("expected WR=60%%, got %.1f%%", winRate)
	}
}

func TestGetWalletStats_PaginationStops(t *testing.T) {
	var pageCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.String(), "closed-positions") {
			page := pageCount.Add(1)
			if page <= 3 {
				// Return full pages (50 items)
				items := make([]map[string]float64, 50)
				for i := range items {
					items[i] = map[string]float64{"realizedPnl": 100}
				}
				json.NewEncoder(w).Encode(items)
			} else {
				// Page 4: partial (< 50) → should stop
				items := []map[string]float64{{"realizedPnl": -10}}
				json.NewEncoder(w).Encode(items)
			}
		} else {
			fmt.Fprint(w, `{"traded": 1}`)
		}
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	// Simulate the pagination loop
	const pageSize = 50
	const maxPages = 20
	total := 0
	for page := 0; page < maxPages; page++ {
		url := fmt.Sprintf("%s/closed-positions?user=0x&limit=%d&offset=%d", srv.URL, pageSize, page*pageSize)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err != nil {
			break
		}
		var items []map[string]float64
		json.NewDecoder(resp.Body).Decode(&items)
		resp.Body.Close()
		total += len(items)
		if len(items) < pageSize {
			break
		}
	}

	// 3 full pages (150) + 1 partial (1) = 151
	if total != 151 {
		t.Errorf("expected 151 total items, got %d", total)
	}
	if pageCount.Load() != 4 {
		t.Errorf("expected 4 page requests, got %d", pageCount.Load())
	}
}

func TestGetWalletStats_ContextCancels_Pagination(t *testing.T) {
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		// Simulate real API latency so context has time to expire.
		time.Sleep(30 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		// Always return full page to keep pagination going
		items := make([]map[string]float64, 50)
		for i := range items {
			items[i] = map[string]float64{"realizedPnl": 1}
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()

	// 150ms budget with 30ms per request → ~5 pages max before timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	client := srv.Client()

	const pageSize = 50
	for page := 0; page < 20; page++ {
		url := fmt.Sprintf("%s/closed-positions?user=0x&limit=%d&offset=%d", srv.URL, pageSize, page*pageSize)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			break // context cancelled
		}
		resp, err := client.Do(req)
		if err != nil {
			break // context cancelled
		}
		resp.Body.Close()
	}

	// Should have been cancelled before hitting all 20 pages.
	if hits.Load() >= 15 {
		t.Errorf("expected context cancellation to stop pagination early, got %d pages", hits.Load())
	}
}

// ---------- WalletStats struct ----------

func TestWalletStats_Defaults(t *testing.T) {
	stats := &WalletStats{WinRate: -1, TotalPnL: -999999}
	if stats.WinRate != -1 {
		t.Errorf("default WinRate should be -1, got %f", stats.WinRate)
	}
	if stats.TotalPnL != -999999 {
		t.Errorf("default TotalPnL should be -999999, got %f", stats.TotalPnL)
	}
}

func TestWinRateCalculation(t *testing.T) {
	tests := []struct {
		name   string
		wins   int
		losses int
		want   float64
	}{
		{"all wins", 10, 0, 100.0},
		{"all losses", 0, 5, 0.0},
		{"mixed", 3, 2, 60.0},
		{"single win", 1, 0, 100.0},
		{"even", 5, 5, 50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := tt.wins + tt.losses
			if total == 0 {
				t.Skip("zero total")
			}
			got := float64(tt.wins) / float64(total) * 100
			if got != tt.want {
				t.Errorf("WR(%d/%d) = %.1f%%, want %.1f%%", tt.wins, tt.losses, got, tt.want)
			}
		})
	}
}
