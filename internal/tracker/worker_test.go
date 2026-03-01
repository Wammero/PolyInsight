package tracker

import (
	"encoding/json"
	"testing"

	"polymarket_go/internal/polymarket"
)

func TestContainsBanWord(t *testing.T) {
	// NOTE: containsBanWord expects lowercase input.
	// In production, the worker calls containsBanWord(strings.ToLower(p.Title)).
	tests := []struct {
		title string
		want  bool
	}{
		{"will bitcoin reach $100k?", true},
		{"ethereum price above $5k", true},
		{"will btc moon?", true},
		{"solana ecosystem growth", true},
		{"will eth flip bitcoin?", true},
		{"xrp lawsuit outcome", true},
		{"ripple vs sec", true},
		{"will trump win 2024?", false},
		{"nba finals winner", false},
		{"fed rate decision", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := containsBanWord(tt.title)
			if got != tt.want {
				t.Errorf("containsBanWord(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestContainsBanWord_CaseSensitive(t *testing.T) {
	// containsBanWord expects lowercase input (caller lowercases).
	if containsBanWord("BITCOIN") {
		t.Error("containsBanWord should be case-sensitive; caller is responsible for lowering")
	}
	if !containsBanWord("bitcoin") {
		t.Error("expected 'bitcoin' to match")
	}
}

func TestWSMessage_Unmarshal(t *testing.T) {
	raw := `{
		"type": "trades",
		"payload": {
			"asset": "abc",
			"slug": "test-market",
			"title": "Test Market",
			"outcome": "Yes",
			"price": 0.65,
			"size": 1000,
			"side": "BUY",
			"proxyWallet": "0x1234",
			"pseudonym": "TestUser",
			"transactionHash": "0xhash123",
			"timestamp": 1700000000
		}
	}`

	var msg wsMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg.Type != "trades" {
		t.Errorf("type = %q, want 'trades'", msg.Type)
	}
	if msg.Payload.Slug != "test-market" {
		t.Errorf("slug = %q, want 'test-market'", msg.Payload.Slug)
	}
	if msg.Payload.Price != 0.65 {
		t.Errorf("price = %f, want 0.65", msg.Payload.Price)
	}
	if msg.Payload.Size != 1000 {
		t.Errorf("size = %f, want 1000", msg.Payload.Size)
	}

	volume := msg.Payload.Size * msg.Payload.Price
	if volume != 650 {
		t.Errorf("volume = %.0f, want 650", volume)
	}
}

func TestWSMessage_NonTrade(t *testing.T) {
	raw := `{"type": "heartbeat", "payload": {}}`
	var msg wsMessage
	err := json.Unmarshal([]byte(raw), &msg)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type == "trades" {
		t.Error("heartbeat should not be parsed as trades")
	}
}

func TestWSMessage_InvalidJSON(t *testing.T) {
	raw := `{invalid}`
	var msg wsMessage
	err := json.Unmarshal([]byte(raw), &msg)
	if err == nil {
		t.Error("expected unmarshal error for invalid JSON")
	}
}

func TestVolumeFilter(t *testing.T) {
	tests := []struct {
		name           string
		size           float64
		price          float64
		minWhaleAmount float64
		isWatched      bool
		shouldPass     bool
	}{
		{"whale pass", 2000, 0.5, 100, false, true},         // $1000 > $100
		{"below threshold", 50, 0.5, 100, false, false},     // $25 < $100
		{"exactly threshold", 200, 0.5, 100, false, true},   // $100 >= $100
		{"watched bypass", 1, 0.01, 100, true, true},        // $0.01 but watched
		{"zero volume", 0, 0, 100, false, false},             // $0 < $100
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volume := tt.size * tt.price
			pass := tt.isWatched || volume >= tt.minWhaleAmount
			if pass != tt.shouldPass {
				t.Errorf("volume=%.2f, isWatched=%v: pass=%v, want %v",
					volume, tt.isWatched, pass, tt.shouldPass)
			}
		})
	}
}

func TestTradePayload_JSON_Roundtrip(t *testing.T) {
	original := polymarket.TradePayload{
		Asset:           "asset-id",
		Slug:            "market-slug",
		Title:           "Market Title",
		Outcome:         "Yes",
		Price:           0.75,
		Size:            500,
		Side:            "BUY",
		ProxyWallet:     "0xabcdef1234567890abcdef1234567890abcdef12",
		Pseudonym:       "Player",
		TransactionHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Timestamp:       1700000000,
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded polymarket.TradePayload
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch:\n  got  %+v\n  want %+v", decoded, original)
	}
}
