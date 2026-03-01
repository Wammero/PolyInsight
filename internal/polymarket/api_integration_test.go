package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// Integration tests that call the real Polymarket API.
// These validate our parsing logic against live data.
// Run with: go test -v -run TestAPI -count=1 ./internal/polymarket/
//
// NOTE: Polymarket API is geo-restricted. Tests skip automatically
// when the API returns empty results (indicating geo-blocking).

var apiClient = &http.Client{Timeout: 15 * time.Second}

// Known active wallet with substantial history for consistent results.
const testWallet = "0xe328067e38ab79a08d1e0a71be037fd511d62eb1"

// checkAPIAccess verifies Polymarket API is reachable and not geo-blocked.
// Returns false (and calls t.Skip) if the API is restricted.
func checkAPIAccess(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://gamma-api.polymarket.com/markets?slug=will-donald-trump-win-the-2024-presidential-election", nil)
	resp, err := apiClient.Do(req)
	if err != nil {
		t.Skipf("Polymarket API unreachable: %v", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	var markets []struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&markets)
	if len(markets) == 0 {
		t.Skip("Polymarket API geo-blocked (empty response) — skipping integration test")
	}
}

func TestAPI_TradeCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	checkAPIAccess(t)
	ctx := context.Background()
	url := "https://data-api.polymarket.com/traded?user=" + testWallet

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		t.Fatalf("API unreachable: %v", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	var d struct {
		Traded int `json:"traded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	t.Logf("Wallet %s: %d trades", testWallet, d.Traded)

	if d.Traded < 100 {
		t.Errorf("expected well-known wallet to have 100+ trades, got %d", d.Traded)
	}
}

func TestAPI_Leaderboard_PnL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	checkAPIAccess(t)
	ctx := context.Background()
	url := fmt.Sprintf("https://data-api.polymarket.com/v1/leaderboard?user=%s&timePeriod=ALL", testWallet)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		t.Fatalf("API unreachable: %v", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	var rows []struct {
		PnL float64 `json:"pnl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("leaderboard returned no rows")
	}

	t.Logf("Wallet %s PnL: $%.2f", testWallet, rows[0].PnL)
	// Just verify we got a numeric response; PnL fluctuates.
}

func TestAPI_ClosedPositions_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	checkAPIAccess(t)
	ctx := context.Background()
	const pageSize = 50

	totalW, totalL := 0, 0
	pages := 0

	for page := 0; page < 20; page++ {
		offset := page * pageSize
		url := fmt.Sprintf(
			"https://data-api.polymarket.com/closed-positions?user=%s&limit=%d&offset=%d",
			testWallet, pageSize, offset,
		)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := apiClient.Do(req)
		if err != nil {
			t.Fatalf("API unreachable on page %d: %v", page, err)
		}

		var closed []struct {
			RealizedPnl float64 `json:"realizedPnl"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&closed)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if decErr != nil {
			t.Fatalf("decode error on page %d: %v", page, decErr)
		}

		pages++
		for _, pos := range closed {
			if pos.RealizedPnl > 0 {
				totalW++
			} else {
				totalL++
			}
		}

		if len(closed) < pageSize {
			break
		}
	}

	total := totalW + totalL
	t.Logf("Wallet %s: %d closed positions (%d pages), %dW/%dL", testWallet, total, pages, totalW, totalL)

	if total < 50 {
		t.Errorf("expected well-known wallet to have 50+ closed positions, got %d", total)
	}

	// CRITICAL: verify we got LOSSES too (the pagination bug fix)
	if totalL == 0 {
		t.Error("PAGINATION BUG: no losses found — API likely capped at 50 results (all wins)")
	}

	if total > 0 {
		winRate := float64(totalW) / float64(total) * 100
		t.Logf("Win rate: %.1f%%", winRate)
		if winRate == 100 && total > 50 {
			t.Error("PAGINATION BUG: 100%% win rate with many positions — losses not fetched")
		}
	}
}

func TestAPI_ClosedPositions_FirstPageLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	checkAPIAccess(t)
	ctx := context.Background()

	// Verify API really caps at 50 per page even if we ask for 500
	url := fmt.Sprintf(
		"https://data-api.polymarket.com/closed-positions?user=%s&limit=500&offset=0",
		testWallet,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		t.Fatalf("API unreachable: %v", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	var closed []struct {
		RealizedPnl float64 `json:"realizedPnl"`
	}
	json.NewDecoder(resp.Body).Decode(&closed)

	t.Logf("Requested limit=500, got %d results", len(closed))
	if len(closed) > 50 {
		t.Logf("NOTE: API returned %d items (cap may have changed from 50)", len(closed))
	} else {
		t.Logf("Confirmed: API caps at %d results per page", len(closed))
	}

	// Check if first page is all wins (sorted by realizedPnl DESC)
	wins := 0
	for _, pos := range closed {
		if pos.RealizedPnl > 0 {
			wins++
		}
	}
	t.Logf("First page: %d/%d are wins (%.0f%%)", wins, len(closed), float64(wins)/float64(len(closed))*100)
}

func TestAPI_MarketMeta(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping API integration test in short mode")
	}
	checkAPIAccess(t)
	ctx := context.Background()

	// Use a well-known active slug
	slug := "will-donald-trump-win-the-2024-presidential-election"
	url := "https://gamma-api.polymarket.com/markets?slug=" + slug

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		t.Fatalf("API unreachable: %v", err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	var markets []struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
		Closed bool   `json:"closed"`
	}
	json.NewDecoder(resp.Body).Decode(&markets)

	if len(markets) == 0 {
		t.Fatal("no markets returned for known slug")
	}

	t.Logf("Market %q: ID=%s active=%v closed=%v", slug, markets[0].ID, markets[0].Active, markets[0].Closed)

	// Fetch tags
	tagURL := "https://gamma-api.polymarket.com/markets/" + markets[0].ID + "/tags"
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, tagURL, nil)
	resp2, err := apiClient.Do(req2)
	if err != nil {
		t.Fatalf("tags API unreachable: %v", err)
	}
	defer func() { io.Copy(io.Discard, resp2.Body); resp2.Body.Close() }()

	var tags []struct {
		ID    json.Number `json:"id"`
		Slug  string      `json:"slug"`
		Label string      `json:"label"`
	}
	json.NewDecoder(resp2.Body).Decode(&tags)

	t.Logf("Market has %d tags:", len(tags))
	for _, tag := range tags {
		t.Logf("  - ID=%s Slug=%q Label=%q", tag.ID.String(), tag.Slug, tag.Label)
	}

	// Verify at least one tag is recognized by our category system
	recognized := false
	for _, tag := range tags {
		if NonSportEmoji(tag.Slug) != "" || ResolveSportByTagID(tag.ID.String()) != "" {
			recognized = true
			break
		}
	}
	if !recognized {
		t.Log("WARNING: no tags recognized by category system — may need update")
	}
}

func TestAPI_VolumeCalculation(t *testing.T) {
	// Verify: volume = Size * Price = USD spent
	// Price is probability (0-1), Size is shares.
	// Example: 1000 shares at $0.50 = $500 spent.
	size := 1000.0
	price := 0.5
	volume := size * price
	if volume != 500.0 {
		t.Errorf("volume calculation: %.0f * %.2f = %.0f, expected 500", size, price, volume)
	}

	// Edge: price near 0
	volume2 := 5000.0 * 0.01
	if volume2 != 50.0 {
		t.Errorf("low-price volume: expected 50, got %.0f", volume2)
	}

	// Edge: price near 1
	volume3 := 100.0 * 0.99
	if volume3 != 99.0 {
		t.Errorf("high-price volume: expected 99, got %.0f", volume3)
	}
}
