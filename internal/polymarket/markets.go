package polymarket

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"polymarket_go/internal/cache"
)

// GetMeta fetches and caches category metadata for a market slug.
// Returns nil when the market should be dropped (inactive, closed, blacklisted tag).
func GetMeta(ctx context.Context, slug string, client *http.Client) *CategoryMeta {
	var cached CategoryMeta
	if cache.GetJSON(ctx, "meta:"+slug, &cached) {
		if cached.Label == "" {
			return nil // negative cache (was invalid)
		}
		return &cached
	}

	meta, valid := fetchMeta(ctx, slug, client)
	if !valid {
		// Store a negative-cache entry to avoid hammering the API.
		cache.SetJSON(ctx, "meta:"+slug, &CategoryMeta{}, 30*time.Minute)
		return nil
	}
	cache.SetJSON(ctx, "meta:"+slug, meta, 12*time.Hour)
	return meta
}

func fetchMeta(ctx context.Context, slug string, client *http.Client) (*CategoryMeta, bool) {
	// Step 1: resolve market ID.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://gamma-api.polymarket.com/markets?slug="+slug, nil)
	if err != nil {
		return nil, false
	}
	resp, err := client.Do(req)
	if err != nil || resp == nil || resp.StatusCode != 200 {
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		return nil, false
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	var markets []struct {
		ID     string `json:"id"`
		Active bool   `json:"active"`
		Closed bool   `json:"closed"`
	}
	json.NewDecoder(resp.Body).Decode(&markets)

	if len(markets) == 0 || !markets[0].Active || markets[0].Closed {
		return nil, false
	}

	// Step 2: fetch tags.
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://gamma-api.polymarket.com/markets/"+markets[0].ID+"/tags", nil)
	if err != nil {
		return nil, false
	}
	resp2, err := client.Do(req2)
	if err != nil || resp2 == nil || resp2.StatusCode != 200 {
		if resp2 != nil {
			io.Copy(io.Discard, resp2.Body)
			resp2.Body.Close()
		}
		return nil, false
	}
	defer func() { io.Copy(io.Discard, resp2.Body); resp2.Body.Close() }()
	var tags []struct {
		ID    json.Number `json:"id"`
		Slug  string      `json:"slug"`
		Label string      `json:"label"`
	}
	json.NewDecoder(resp2.Body).Decode(&tags)

	// Drop blacklisted markets.
	for _, t := range tags {
		for _, bt := range BlackTags {
			if t.ID.String() == bt {
				return nil, false
			}
		}
	}

	// Priority 1: sport category via tag ID.
	for _, t := range tags {
		if sportCat := ResolveSportByTagID(t.ID.String()); sportCat != "" {
			if info, ok := AllCategories[sportCat]; ok {
				return &info, true
			}
		}
	}

	// Priority 2: sport category via slug fragment.
	slugLow := strings.ToLower(slug)
	for series, sportCat := range seriesToSport {
		if strings.Contains(slugLow, series) {
			if info, ok := AllCategories[sportCat]; ok {
				return &info, true
			}
		}
	}

	// Priority 3: non-sport category via tag slug.
	for _, t := range tags {
		tagSlugLow := strings.ToLower(t.Slug)
		if emoji := NonSportEmoji(tagSlugLow); emoji != "" {
			info := AllCategories[tagSlugLow]
			if info.Category == "" {
				info = CategoryMeta{Label: t.Label, Emoji: emoji, Category: tagSlugLow}
			}
			return &info, true
		}
	}

	// Fallback: General.
	general := AllCategories["other"]
	return &general, true
}
