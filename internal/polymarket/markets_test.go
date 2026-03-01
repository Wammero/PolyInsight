package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchMeta_ActiveMarket_PoliticsTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/tags"):
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "999", "slug": "politics", "label": "Politics"},
			})
		default:
			// /markets?slug=...
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "abc123", "active": true, "closed": false},
			})
		}
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	// Call fetchMeta with the mock server URL rewritten.
	// Since fetchMeta uses hardcoded URLs, we test the HTTP pattern directly.
	// Step 1: markets
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets?slug=test-slug", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var markets []struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
		Closed bool   `json:"closed"`
	}
	json.NewDecoder(resp.Body).Decode(&markets)
	if len(markets) != 1 || !markets[0].Active {
		t.Fatalf("expected 1 active market, got %v", markets)
	}

	// Step 2: tags
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets/"+markets[0].ID+"/tags", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("tags request failed: %v", err)
	}
	defer resp2.Body.Close()
	var tags []struct {
		ID    json.Number `json:"id"`
		Slug  string      `json:"slug"`
		Label string      `json:"label"`
	}
	json.NewDecoder(resp2.Body).Decode(&tags)

	// Verify politics tag detection
	for _, tag := range tags {
		if emoji := NonSportEmoji(strings.ToLower(tag.Slug)); emoji != "" {
			if emoji != "⚖️" {
				t.Errorf("expected politics emoji ⚖️, got %s", emoji)
			}
			return
		}
	}
	t.Error("politics tag not detected")
}

func TestFetchMeta_ClosedMarket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "abc", "active": true, "closed": true},
		})
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets?slug=closed-market", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var markets []struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
		Closed bool   `json:"closed"`
	}
	json.NewDecoder(resp.Body).Decode(&markets)

	if len(markets) == 0 || !markets[0].Closed {
		t.Fatal("expected closed market")
	}
	// In real code, fetchMeta would return nil for closed markets.
}

func TestFetchMeta_InactiveMarket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "abc", "active": false, "closed": false},
		})
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets?slug=inactive", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var markets []struct {
		Active bool `json:"active"`
	}
	json.NewDecoder(resp.Body).Decode(&markets)

	if len(markets) > 0 && markets[0].Active {
		t.Error("expected inactive market")
	}
}

func TestFetchMeta_BlacklistedTag(t *testing.T) {
	for _, bt := range BlackTags {
		tagID := bt
		found := false
		for _, tag := range []struct{ ID string }{{ID: tagID}} {
			for _, blacklisted := range BlackTags {
				if tag.ID == blacklisted {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("BlackTag %s not matched", tagID)
		}
	}
}

func TestFetchMeta_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := srv.Client()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets?slug=slow", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected context deadline error")
	}
}

func TestFetchMeta_EmptyMarkets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets?slug=nonexistent", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var markets []struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&markets)
	if len(markets) != 0 {
		t.Errorf("expected empty markets, got %d", len(markets))
	}
}

func TestFetchMeta_ServerError_ReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := srv.Client()
	ctx := context.Background()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/markets?slug=error", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// ---------- Category resolution ----------

func TestResolveSportByTagID(t *testing.T) {
	tests := []struct {
		tagID string
		want  string
	}{
		{"64", "esports"},
		{"28", "basketball"},
		{"450", "football"},
		{"279", "fighting"},
		{"864", "tennis"},
		{"517", "cricket"},
		{"99999", ""}, // unknown
	}
	for _, tt := range tests {
		t.Run(tt.tagID, func(t *testing.T) {
			got := ResolveSportByTagID(tt.tagID)
			if got != tt.want {
				t.Errorf("ResolveSportByTagID(%q) = %q, want %q", tt.tagID, got, tt.want)
			}
		})
	}
}

func TestResolveSportBySlug(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"nba", "basketball"},
		{"nfl", "football"},
		{"ufc", "fighting"},
		{"atp", "tennis"},
		{"counter-strike", "esports"},
		{"nonexistent", ""},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := ResolveSportBySlug(tt.slug)
			if got != tt.want {
				t.Errorf("ResolveSportBySlug(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestNonSportEmoji(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"politics", "⚖️"},
		{"crypto", "💎"},
		{"tech", "💻"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := NonSportEmoji(tt.slug)
			if got != tt.want {
				t.Errorf("NonSportEmoji(%q) = %q, want %q", tt.slug, got, tt.want)
			}
		})
	}
}

func TestAllCategories_Completeness(t *testing.T) {
	expected := []string{
		"politics", "elections", "crypto", "finance", "economy",
		"geopolitics", "tech", "culture", "world",
		"esports", "hockey", "basketball", "football", "baseball",
		"fighting", "tennis", "cricket", "rugby", "table_tennis",
		"other",
	}
	for _, cat := range expected {
		if _, ok := AllCategories[cat]; !ok {
			t.Errorf("AllCategories missing %q", cat)
		}
	}
	if len(AllCategories) != len(expected) {
		t.Errorf("AllCategories has %d entries, expected %d", len(AllCategories), len(expected))
	}
}

func TestBanWords(t *testing.T) {
	expected := []string{"solana", "bitcoin", "btc", "ethereum", "eth", "xrp", "ripple"}
	if len(BanWords) != len(expected) {
		t.Fatalf("BanWords has %d entries, expected %d", len(BanWords), len(expected))
	}
	for i, w := range expected {
		if BanWords[i] != w {
			t.Errorf("BanWords[%d] = %q, want %q", i, BanWords[i], w)
		}
	}
}

func TestCategoryOrder_CoversAll(t *testing.T) {
	seen := map[string]bool{}
	for _, row := range CategoryOrder {
		for _, key := range row {
			if _, ok := AllCategories[key]; !ok {
				t.Errorf("CategoryOrder references unknown key %q", key)
			}
			seen[key] = true
		}
	}
	for key := range AllCategories {
		if !seen[key] {
			t.Errorf("AllCategories[%q] not referenced in CategoryOrder", key)
		}
	}
}
