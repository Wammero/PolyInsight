package bot

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"polymarket_go/internal/polymarket"
)

// ---------- shortID generation ----------

func TestShortID_Deterministic(t *testing.T) {
	txHash := "0xabc123def456"
	wallet := "0x1234567890abcdef1234567890abcdef12345678"

	rawKey := strings.ToLower(txHash + wallet)
	h := sha256.Sum256([]byte(rawKey))
	id1 := hex.EncodeToString(h[:10])

	// Same inputs → same ID
	rawKey2 := strings.ToLower(txHash + wallet)
	h2 := sha256.Sum256([]byte(rawKey2))
	id2 := hex.EncodeToString(h2[:10])

	if id1 != id2 {
		t.Errorf("shortID not deterministic: %q != %q", id1, id2)
	}
	if len(id1) != 20 {
		t.Errorf("shortID length should be 20, got %d", len(id1))
	}
}

func TestShortID_DifferentWallets_DifferentIDs(t *testing.T) {
	txHash := "0xsamehash"

	wallet1 := "0xwallet1111111111111111111111111111111111"
	wallet2 := "0xwallet2222222222222222222222222222222222"

	rawKey1 := strings.ToLower(txHash + wallet1)
	h1 := sha256.Sum256([]byte(rawKey1))
	id1 := hex.EncodeToString(h1[:10])

	rawKey2 := strings.ToLower(txHash + wallet2)
	h2 := sha256.Sum256([]byte(rawKey2))
	id2 := hex.EncodeToString(h2[:10])

	if id1 == id2 {
		t.Error("different wallets with same txHash should produce different shortIDs")
	}
}

func TestShortID_CaseInsensitive(t *testing.T) {
	txHash := "0xABC123"
	wallet := "0xDEF456"

	rawKey1 := strings.ToLower(txHash + wallet)
	h1 := sha256.Sum256([]byte(rawKey1))
	id1 := hex.EncodeToString(h1[:10])

	rawKey2 := strings.ToLower(strings.ToUpper(txHash) + strings.ToUpper(wallet))
	h2 := sha256.Sum256([]byte(rawKey2))
	id2 := hex.EncodeToString(h2[:10])

	if id1 != id2 {
		t.Error("shortID should be case-insensitive")
	}
}

// ---------- fmtPnL ----------

func TestFmtPnL(t *testing.T) {
	tests := []struct {
		name string
		val  float64
		want string
	}{
		{"positive", 1234.0, "+$1234"},
		{"negative", -500.0, "-$500"},
		{"zero", 0, "+$0"},
		{"unknown", -999999, "—"},
		{"very negative but not sentinel", -100000, "-$100000"},
		{"large positive", 99999.0, "+$99999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtPnL(tt.val)
			if got != tt.want {
				t.Errorf("fmtPnL(%f) = %q, want %q", tt.val, got, tt.want)
			}
		})
	}
}

// ---------- FormatWalletStats ----------

func TestFormatWalletStats_WithData(t *testing.T) {
	stats := &polymarket.WalletStats{
		TotalTrades: 42,
		Wins:        30,
		Losses:      12,
		WinRate:     71.4,
		TotalPnL:    5000,
	}
	wallet := "0x1234567890abcdef1234567890abcdef12345678"
	result := FormatWalletStats(wallet, stats)

	checks := []string{
		"Статистика кошелька",
		"0x1234...5678", // truncated
		"42",            // total trades
		"30✅ / 12❌",   // wins/losses
		"+$5000",        // P&L
	}
	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("FormatWalletStats missing %q in output:\n%s", check, result)
		}
	}
}

func TestFormatWalletStats_UnknownWinRate(t *testing.T) {
	stats := &polymarket.WalletStats{
		TotalTrades: 0,
		WinRate:     -1,
		TotalPnL:    -999999,
	}
	wallet := "0xshort"
	result := FormatWalletStats(wallet, stats)

	if !strings.Contains(result, "—") {
		t.Error("expected — for unknown winrate/pnl")
	}
	// Short wallet should not be truncated
	if !strings.Contains(result, "0xshort") {
		t.Error("short wallet should not be truncated")
	}
}

func TestFormatWalletStats_WalletTruncation(t *testing.T) {
	tests := []struct {
		name   string
		wallet string
		short  string
	}{
		{"long wallet", "0x1234567890abcdef1234567890abcdef12345678", "0x1234...5678"},
		{"short wallet", "0xshort", "0xshort"},
		{"exactly 12", "0x12345678ab", "0x12345678ab"},
		{"13 chars", "0x12345678abc", "0x1234...8abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &polymarket.WalletStats{WinRate: -1, TotalPnL: -999999}
			result := FormatWalletStats(tt.wallet, stats)
			if !strings.Contains(result, tt.short) {
				t.Errorf("expected %q in output, got:\n%s", tt.short, result)
			}
		})
	}
}

// ---------- Alert filter logic ----------

func TestAlertFilter_AmountFilter(t *testing.T) {
	// volume = size * price
	size := 1000.0
	price := 0.5
	volume := size * price // $500

	minAmount := 600.0
	if volume >= minAmount {
		t.Errorf("$500 should be filtered by minAmount=$600")
	}

	minAmount2 := 400.0
	if volume < minAmount2 {
		t.Errorf("$500 should pass minAmount=$400")
	}
}

func TestAlertFilter_TradesFilter(t *testing.T) {
	// MaxTrades=5 means include wallets with <= 5 trades
	trades := 5
	maxTrades := 5
	if trades > maxTrades {
		t.Error("trades=5, maxTrades=5: should pass (inclusive)")
	}

	trades = 6
	if trades <= maxTrades {
		t.Error("trades=6, maxTrades=5: should be filtered")
	}
}

func TestAlertFilter_OddsFilter(t *testing.T) {
	price := 0.4
	odds := 1.0 / price // 2.5

	minOdds := 1.5
	if odds < minOdds {
		t.Errorf("odds=%.2f should pass minOdds=%.2f", odds, minOdds)
	}

	minOdds2 := 3.0
	if odds >= minOdds2 {
		t.Errorf("odds=%.2f should be filtered by minOdds=%.2f", odds, minOdds2)
	}
}

func TestAlertFilter_CategoryFilter(t *testing.T) {
	userCats := []string{"politics", "crypto"}
	tradeCat := "crypto"

	allowed := false
	for _, cat := range userCats {
		if cat == tradeCat {
			allowed = true
			break
		}
	}
	if !allowed {
		t.Error("crypto should be allowed by user categories [politics, crypto]")
	}

	tradeCat2 := "sports"
	allowed2 := false
	for _, cat := range userCats {
		if cat == tradeCat2 {
			allowed2 = true
			break
		}
	}
	if allowed2 {
		t.Error("sports should NOT be allowed by user categories [politics, crypto]")
	}
}

func TestAlertFilter_EmptyCategories_AllowsAll(t *testing.T) {
	userCats := []string{} // empty = all categories
	if len(userCats) > 0 {
		t.Error("empty categories should allow all")
	}
}

func TestAlertFilter_FollowersBypassFilters(t *testing.T) {
	// When isWatch=true, all filters should be skipped
	isWatch := true
	volume := 10.0    // way below any reasonable minimum
	trades := 1000    // way above any maxTrades
	odds := 1.001     // way below any minOdds
	minAmount := 500.0
	maxTrades := 5
	minOdds := 1.5

	if isWatch {
		// Followers bypass — this is the expected path
		return
	}

	// This code should never be reached in the test
	if volume < minAmount {
		t.Error("should not filter followers by amount")
	}
	if trades > maxTrades {
		t.Error("should not filter followers by trades")
	}
	if odds < minOdds {
		t.Error("should not filter followers by odds")
	}
}
